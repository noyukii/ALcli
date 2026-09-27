package views

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	coverCellWidth  = 17
	coverCellHeight = 12
)

type CoverReadyMsg struct {
	URL      string
	W        int
	H        int
	Rendered string
}

// ShowImages controls whether views fetch and render cover art. Set once at startup.
var ShowImages = true

// ApplyMode sets the cover render mode and keeps ShowImages consistent with it.
func ApplyMode(m RenderMode) {
	Mode = m
	ShowImages = m != ModeOff
}

var (
	coverCacheMu sync.Mutex
	coverCache   = map[string]string{}
)

// coverKey scopes the cache by size, since a cover rendered for a 20-cell card
// can't be reused when the card flexes to 30 cells.
func coverKey(url string, w, h int) string {
	return fmt.Sprintf("%s|%dx%d", url, w, h)
}

func cachedCover(url string, w, h int) (string, bool) {
	coverCacheMu.Lock()
	defer coverCacheMu.Unlock()
	s, ok := coverCache[coverKey(url, w, h)]
	return s, ok
}

func storeCover(url string, w, h int, rendered string) {
	if url == "" {
		return
	}
	coverCacheMu.Lock()
	coverCache[coverKey(url, w, h)] = rendered
	coverCacheMu.Unlock()
}

func FetchCoverCmd(url string, w, h int) tea.Cmd {
	return func() tea.Msg {
		msg := CoverReadyMsg{URL: url, W: w, H: h}
		if url == "" || w <= 0 || h <= 0 {
			return msg
		}
		coverDirOnce.Do(evictCoverDir)

		var img image.Image
		if raw, ok := readDiskCover(url); ok {
			img, _, _ = image.Decode(bytes.NewReader(raw))
		}
		if img == nil {
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				return msg
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return msg
			}
			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				return msg
			}
			img, _, err = image.Decode(bytes.NewReader(raw))
			if err != nil {
				return msg
			}
			writeDiskCover(url, raw)
		}
		if Mode == ModeKitty {
			msg.Rendered = kittyPlaceholderCover(img, w, h, kittyIDFor(url, w, h))
		} else {
			msg.Rendered = renderCoverCells(img, w, h)
		}
		return msg
	}
}

// coverCacheMaxBytes caps the on-disk cover cache (~150 MiB).
const coverCacheMaxBytes = 150 << 20

var coverDirOnce sync.Once

func coverDiskDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "anilist-cli", "covers")
}

// coverDiskPath keys the disk cache by URL hash; contents are the raw
// downloaded bytes, so decoding stays identical to the network path.
func coverDiskPath(url string) string {
	dir := coverDiskDir()
	if dir == "" {
		return ""
	}
	sum := sha1.Sum([]byte(url))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".img")
}

func readDiskCover(url string) ([]byte, bool) {
	p := coverDiskPath(url)
	if p == "" {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil || len(b) == 0 {
		return nil, false
	}
	return b, true
}

func writeDiskCover(url string, b []byte) {
	p := coverDiskPath(url)
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0o644) // best-effort; a failed write just means a refetch next run
}

// evictCoverDir deletes oldest-modified files once the cache exceeds the size
// cap. Runs at most once per process, inside a fetch goroutine.
func evictCoverDir() {
	dir := coverDiskDir()
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type file struct {
		path  string
		size  int64
		mtime time.Time
	}
	files := make([]file, 0, len(entries))
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, file{filepath.Join(dir, e.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total <= coverCacheMaxBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mtime.Before(files[j].mtime) })
	for _, f := range files {
		if total <= coverCacheMaxBytes {
			break
		}
		if err := os.Remove(f.path); err == nil {
			total -= f.size
		}
	}
}

// coverSettleMsg triggers a re-render shortly after a cover arrives so
// DrainKittyTransmits can re-assert the kitty placement once the terminal has
// decoded the image (some terminals fix the placement scale at creation time,
// leaving the cover stuck small).
type coverSettleMsg struct{}

// coverSettle schedules the post-decode re-render. Nil outside kitty mode.
func coverSettle() tea.Cmd {
	if Mode != ModeKitty {
		return nil
	}
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return coverSettleMsg{} })
}

func renderCoverCells(img image.Image, w, h int) string {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return ""
	}
	rows := make([]string, 0, h)
	for row := 0; row < h; row++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			sx := bounds.Min.X + x*srcW/w
			topY := bounds.Min.Y + (row*2*srcH)/(h*2)
			botY := bounds.Min.Y + ((row*2+1)*srcH)/(h*2)
			tr, tg, tb, _ := img.At(sx, topY).RGBA()
			br, bg, bb, _ := img.At(sx, botY).RGBA()
			fg := lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", uint8(tr>>8), uint8(tg>>8), uint8(tb>>8)))
			bgColor := lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", uint8(br>>8), uint8(bg>>8), uint8(bb>>8)))
			sb.WriteString(lipgloss.NewStyle().Foreground(fg).Background(bgColor).Render("▀"))
		}
		rows = append(rows, sb.String())
	}
	return strings.Join(rows, "\n")
}

func CoverPlaceholder(width, heightCells int, seed string) string {
	if width < 1 {
		width = 1
	}
	if heightCells < 1 {
		heightCells = 1
	}
	letter := "?"
	for _, r := range seed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letter = strings.ToUpper(string(r))
			break
		}
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7A8C")).
		Background(lipgloss.Color("#1E2A38"))
	mid := heightCells / 2
	padLeft := (width - 1) / 2
	lines := make([]string, heightCells)
	for i := range lines {
		content := strings.Repeat(" ", width)
		if i == mid {
			content = strings.Repeat(" ", padLeft) + letter + strings.Repeat(" ", width-padLeft-1)
		}
		lines[i] = style.Render(content)
	}
	return strings.Join(lines, "\n")
}
