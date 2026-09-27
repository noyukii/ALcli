package views

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"sync"
	"time"
)

// RenderMode selects how cover art is drawn. Halfblock is the portable default
// (two pixels per character cell); Kitty uses the terminal graphics protocol
// for true images on kitty / Ghostty / WezTerm.
type RenderMode int

const (
	ModeHalfblock RenderMode = iota
	ModeKitty
	ModeOff
)

// Mode is the active cover render mode.
var Mode = ModeHalfblock

// KittyCapable reports whether the current terminal likely speaks the kitty
// graphics protocol (kitty, Ghostty, WezTerm).
func KittyCapable() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}
	term := os.Getenv("TERM")
	if strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") {
		return true
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "Ghostty", "WezTerm":
		return true
	}
	return false
}

// The kitty Unicode-placeholder protocol encodes a cell's (row, column) as
// combining diacritics from this fixed list (these are the first entries of
// kitty's rowcolumn diacritics table). We only index up to the cover height.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
}

const kittyPlaceholder = 0x10EEEE // U+10EEEE, the kitty image placeholder cell

var (
	kittyIDMu   sync.Mutex
	kittyIDs    = map[string]uint32{}
	kittyIDNext uint32
)

var (
	kittyTxMu        sync.Mutex
	kittyTransmitted = map[uint32]bool{}
	kittyPending     []kittyTransmission
	kittyReassert    []kittyReassertion
)

// Bubble Tea's renderer holds a single frame — renderer.write resets its buffer
// — and paints on a ~60fps ticker, so a frame superseded before the next tick is
// discarded whole. Anything emitted through View exactly once can therefore
// never reach the terminal: when covers land in a burst, most of their image
// data dies with the frames that carried it while the placeholder cells
// referencing that data survive in every later frame, leaving blank cards.
//
// kittyTransmission keeps a transmit queued and re-emits it on each drain until
// a full frame interval has passed with no drain, at which point a paint tick
// must have flushed a frame containing it. Retransmits are idempotent (same
// image id), so an extra copy costs bandwidth and nothing else.
type kittyTransmission struct {
	seq      string
	queued   time.Time
	lastEmit time.Time
	emitted  bool
}

const (
	// kittyFrameInterval is one 60fps paint tick plus margin for scheduling jitter.
	kittyFrameInterval = 25 * time.Millisecond
	// kittyTxMaxAge drops a transmit that somehow never settled, so the queue
	// can't retain cover payloads for the life of the process.
	kittyTxMaxAge = 2 * time.Second
)

// kittyReassertion is a placement command re-emitted on every drain until
// `until` passes. Terminals decode transmitted PNGs asynchronously; if a
// terminal fixes the placement's scale at creation time while the image is
// still undecoded, the cover stays stuck at a wrong size. Re-asserting the
// placement (tiny, idempotent) converges it to the correct size once decode
// has finished.
type kittyReassertion struct {
	seq   string
	until time.Time
}

// DrainKittyTransmits returns the image-transmit sequences still awaiting
// delivery plus live placement re-assertions. Emitted at the top of a frame, so
// the heavy base64 payload never goes through the per-frame layout. A transmit
// leaves the queue only once a paint tick has certainly carried it (see
// kittyTransmission).
func DrainKittyTransmits() string {
	kittyTxMu.Lock()
	defer kittyTxMu.Unlock()
	if len(kittyPending) == 0 && len(kittyReassert) == 0 {
		return ""
	}
	now := time.Now()
	var sb strings.Builder
	keptTx := kittyPending[:0]
	for _, tx := range kittyPending {
		flushed := tx.emitted && now.Sub(tx.lastEmit) >= kittyFrameInterval
		if flushed || now.Sub(tx.queued) > kittyTxMaxAge {
			continue
		}
		sb.WriteString(tx.seq)
		tx.emitted = true
		tx.lastEmit = now
		keptTx = append(keptTx, tx)
	}
	kittyPending = keptTx
	kept := kittyReassert[:0]
	for _, ra := range kittyReassert {
		if now.Before(ra.until) {
			sb.WriteString(ra.seq)
			kept = append(kept, ra)
		}
	}
	kittyReassert = kept
	return sb.String()
}

// kittyIDFor returns a stable, non-zero image id for a cover keyed by url+size,
// so re-fetches reuse the same terminal-side image slot.
func kittyIDFor(url string, w, h int) uint32 {
	key := coverKey(url, w, h)
	kittyIDMu.Lock()
	defer kittyIDMu.Unlock()
	if id, ok := kittyIDs[key]; ok {
		return id
	}
	kittyIDNext++
	kittyIDs[key] = kittyIDNext
	return kittyIDNext
}

// kittyChunks writes a base64 PNG payload as the protocol's ≤4096-byte chunks,
// using the given first-chunk control keys.
func kittyChunks(sb *strings.Builder, controls, b64 string) {
	const chunkSize = 4096
	first := true
	for len(b64) > 0 {
		n := chunkSize
		if n > len(b64) {
			n = len(b64)
		}
		part := b64[:n]
		b64 = b64[n:]
		more := 0
		if len(b64) > 0 {
			more = 1
		}
		if first {
			fmt.Fprintf(sb, "\x1b_G%s,m=%d;%s\x1b\\", controls, more, part)
			first = false
		} else {
			fmt.Fprintf(sb, "\x1b_Gm=%d;%s\x1b\\", more, part)
		}
	}
}

// kittyPlaceholderCover returns the cover as a cols×rows block of kitty Unicode
// placeholder cells (light: only the placeholders, which measure cols×rows
// since escapes are zero-width). The heavy image transmit is queued once via
// kittyPending and flushed by DrainKittyTransmits, keeping it out of the
// per-frame layout entirely.
func kittyPlaceholderCover(img image.Image, cols, rows int, id uint32) string {
	if cols < 1 || rows < 1 || rows > len(kittyDiacritics) {
		return ""
	}

	kittyTxMu.Lock()
	if !kittyTransmitted[id] {
		if seq := kittyTransmitSeq(img, id); seq != "" {
			now := time.Now()
			place := kittyPlacementSeq(cols, rows, id)
			// The placement rides along with the transmit so every retry stays
			// self-contained: re-transmitting an id replaces the image, and the
			// trailing placement re-creates the virtual placement it needs.
			kittyPending = append(kittyPending, kittyTransmission{seq: seq + place, queued: now})
			kittyReassert = append(kittyReassert, kittyReassertion{seq: place, until: now.Add(800 * time.Millisecond)})
			kittyTransmitted[id] = true
		}
	}
	kittyTxMu.Unlock()

	return kittyPlacementCells(cols, rows, id)
}

// kittyTransmitSeq builds the one-time sequence that stores the image under id
// (a=t, no display). This is the heavy part (base64 PNG) and is sent once, not
// per frame.
func kittyTransmitSeq(img image.Image, id uint32) string {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(pngBuf.Bytes())

	var sb strings.Builder
	kittyChunks(&sb, fmt.Sprintf("f=100,a=t,i=%d,q=2", id), b64)
	return sb.String()
}

// kittyPlacementSeq builds the command that creates a virtual placement sized
// to the cell box. Tiny and idempotent, so it can be re-asserted after the
// terminal finishes decoding the image. The placement id is explicit (p=1):
// without it the terminal auto-assigns a fresh id per command, stacking
// placements (some made pre-decode with a bogus scale) and picking one
// arbitrarily for the placeholder cells.
func kittyPlacementSeq(cols, rows int, id uint32) string {
	return fmt.Sprintf("\x1b_Ga=p,U=1,i=%d,p=1,c=%d,r=%d,q=2\x1b\\", id, cols, rows)
}

// kittyPlacementCells builds just the placeholder-cell grid that references an
// already-transmitted image by id. Cheap to emit every frame.
func kittyPlacementCells(cols, rows int, id uint32) string {
	// Foreground color encodes the 24-bit image id; underline color encodes
	// the placement id (p=1), keeping the cell→placement reference explicit.
	colorOn := fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[58;2;0;0;1m", byte(id>>16), byte(id>>8), byte(id))
	lines := make([]string, rows)
	for r := 0; r < rows; r++ {
		var row strings.Builder
		row.WriteString(colorOn)
		for c := 0; c < cols; c++ {
			row.WriteRune(kittyPlaceholder)
			if c == 0 {
				// Only the first cell needs explicit (row, col); kitty
				// auto-increments the column for the bare cells that follow.
				row.WriteRune(kittyDiacritics[r])
				row.WriteRune(kittyDiacritics[0])
			}
		}
		row.WriteString("\x1b[39m\x1b[59m")
		lines[r] = row.String()
	}
	return strings.Join(lines, "\n")
}
