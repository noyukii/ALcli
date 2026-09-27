package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noyukii/ALcli/internal/api"
)

const (
	gridColGap = 2 // blank columns between cards
	// Cards are poster-shaped: cover is (gridCellWidth-2)×coverCellHeight cells,
	// sized so the box matches a cover's aspect ratio and the art fills it.
	gridCellWidth  = coverCellWidth + 2  // cover inner width + card border
	gridCardHeight = coverCellHeight + 5 // cover + title/meta/score + border
)

type Grid struct {
	items     []api.Media
	cursor    int
	rowOffset int
	width     int
	height    int
}

func NewGrid() *Grid {
	return &Grid{}
}

func (g *Grid) SetSize(width, height int) {
	g.width = width
	g.height = height
	g.clamp()
}

func (g *Grid) SetItems(items []api.Media) {
	g.items = items
	g.cursor = 0
	g.rowOffset = 0
	g.clamp()
}

func (g *Grid) Len() int {
	return len(g.items)
}

// Columns is how many poster cards (plus gaps) fit across the width. Fewer on a
// narrow terminal, more on a wide one — the responsive part.
func (g *Grid) Columns() int {
	cols := (g.width + gridColGap) / (gridCellWidth + gridColGap)
	if cols < 1 {
		cols = 1
	}
	return cols
}

// CellWidth is the fixed poster-card width, clamped down only if the terminal
// is narrower than a single card.
func (g *Grid) CellWidth() int {
	if g.width > 0 && g.width < gridCellWidth {
		return g.width
	}
	return gridCellWidth
}

func (g *Grid) visibleRows() int {
	rows := g.height / gridCardHeight
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (g *Grid) clamp() {
	if g.cursor > len(g.items)-1 {
		g.cursor = len(g.items) - 1
	}
	if g.cursor < 0 {
		g.cursor = 0
	}
	g.ensureVisible()
}

func (g *Grid) ensureVisible() {
	cols := g.Columns()
	rows := g.visibleRows()
	row := g.cursor / cols
	if row < g.rowOffset {
		g.rowOffset = row
	}
	if row >= g.rowOffset+rows {
		g.rowOffset = row - rows + 1
	}
	totalRows := (len(g.items) + cols - 1) / cols
	if maxOffset := totalRows - rows; g.rowOffset > maxOffset {
		g.rowOffset = maxOffset
	}
	if g.rowOffset < 0 {
		g.rowOffset = 0
	}
}

func (g *Grid) Update(msg tea.KeyMsg) {
	if len(g.items) == 0 {
		return
	}
	cols := g.Columns()
	switch msg.String() {
	case "h", "left":
		if g.cursor > 0 {
			g.cursor--
		}
	case "l", "right":
		if g.cursor < len(g.items)-1 {
			g.cursor++
		}
	case "k", "up":
		if g.cursor-cols >= 0 {
			g.cursor -= cols
		}
	case "j", "down":
		if g.cursor+cols < len(g.items) {
			g.cursor += cols
		}
	default:
		return
	}
	g.ensureVisible()
}

func (g *Grid) Selected() (api.Media, bool) {
	if g.cursor >= 0 && g.cursor < len(g.items) {
		return g.items[g.cursor], true
	}
	return api.Media{}, false
}

// Cursor is the selected item's index.
func (g *Grid) Cursor() int {
	return g.cursor
}

// SetCursor moves the selection to i, keeping it in bounds and visible.
func (g *Grid) SetCursor(i int) {
	g.cursor = i
	g.clamp()
}

// IndexAt maps grid-relative cell coordinates (as printed by View) to an item
// index, for mouse hit-testing. Returns false for gaps, borders beyond the
// last item, and out-of-range rows.
func (g *Grid) IndexAt(x, y int) (int, bool) {
	if x < 0 || y < 0 {
		return 0, false
	}
	row := y / gridCardHeight
	if row >= g.visibleRows() {
		return 0, false
	}
	stride := g.CellWidth() + gridColGap
	if x%stride >= g.CellWidth() {
		return 0, false // the gap between cards
	}
	col := x / stride
	if col >= g.Columns() {
		return 0, false
	}
	idx := (g.rowOffset+row)*g.Columns() + col
	if idx >= len(g.items) {
		return 0, false
	}
	return idx, true
}

func (g *Grid) VisibleItems() []api.Media {
	cols := g.Columns()
	start := g.rowOffset * cols
	if start > len(g.items) {
		start = len(g.items)
	}
	end := start + g.visibleRows()*cols
	if end > len(g.items) {
		end = len(g.items)
	}
	return g.items[start:end]
}

func (g *Grid) View() string {
	if len(g.items) == 0 {
		return ""
	}
	cols := g.Columns()
	cellWidth := g.CellWidth()
	start := g.rowOffset * cols
	if start >= len(g.items) {
		start = 0
	}
	rows := make([]string, 0, g.visibleRows())
	for r := 0; r < g.visibleRows(); r++ {
		rowStart := start + r*cols
		if rowStart >= len(g.items) {
			break
		}
		cards := make([]string, 0, cols*2)
		for c := 0; c < cols; c++ {
			i := rowStart + c
			if i >= len(g.items) {
				break
			}
			if c > 0 {
				cards = append(cards, strings.Repeat(" ", gridColGap))
			}
			cards = append(cards, MediaCard(g.items[i], i == g.cursor, cellWidth))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards...))
	}
	return strings.Join(rows, "\n")
}
