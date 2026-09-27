package views

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/noyukii/ALcli/internal/api"
)

// The image id rides in the placeholder cells' foreground color; if lipgloss
// (card borders, joins, width padding) ever rewrote or stripped that SGR, the
// terminal would pair cards with the wrong image.
func TestCoverIDSurvivesCardRender(t *testing.T) {
	url := "https://example.com/cover.jpg"
	large := url
	m := api.Media{TitleRomaji: "Test Show", CoverImageLarge: &large}

	id := kittyIDFor(url, coverCellWidth, coverCellHeight)
	storeCover(url, coverCellWidth, coverCellHeight, kittyPlacementCells(coverCellWidth, coverCellHeight, id))

	card := MediaCard(m, false, gridCellWidth)
	want := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", byte(id>>16), byte(id>>8), byte(id))
	if !strings.Contains(card, want) {
		t.Fatalf("card lost the id color %q; cover would bind to the wrong image", want)
	}
	if !strings.Contains(card, "\x1b[58;2;0;0;1m") {
		t.Fatal("card lost the underline color carrying the placement id")
	}
}

// Bubble Tea discards a frame that is superseded before the next paint tick, so
// a transmit emitted exactly once can die with the frame carrying it — leaving
// placeholder cells pointing at an image the terminal never received.
func TestKittyTransmitSurvivesDroppedFrames(t *testing.T) {
	resetKittyTransmits(t)

	const id = 4242
	transmit := fmt.Sprintf("\x1b_Gf=100,a=t,i=%d", id)
	if cells := kittyPlaceholderCover(image.NewRGBA(image.Rect(0, 0, 8, 8)), 4, 4, id); cells == "" {
		t.Fatal("no placeholder cells produced")
	}

	if first := DrainKittyTransmits(); !strings.Contains(first, transmit) {
		t.Fatal("first frame carried no image transmit")
	}
	if second := DrainKittyTransmits(); !strings.Contains(second, transmit) {
		t.Fatal("transmit dropped after one frame; a superseded frame loses the image entirely")
	}

	time.Sleep(2 * kittyFrameInterval)
	if third := DrainKittyTransmits(); strings.Contains(third, transmit) {
		t.Fatal("transmit re-sent after a paint tick had certainly flushed it")
	}
}

func resetKittyTransmits(t *testing.T) {
	t.Helper()
	clear := func() {
		kittyTxMu.Lock()
		kittyPending = nil
		kittyReassert = nil
		kittyTransmitted = map[uint32]bool{}
		kittyTxMu.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

func TestKittyIDStablePerURLAndSize(t *testing.T) {
	a := kittyIDFor("u1", 17, 12)
	if kittyIDFor("u1", 17, 12) != a {
		t.Fatal("same url+size produced different ids")
	}
	if kittyIDFor("u1", 24, 15) == a || kittyIDFor("u2", 17, 12) == a {
		t.Fatal("different url or size shared an id")
	}
}
