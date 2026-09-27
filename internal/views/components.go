package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

func MediaCard(m api.Media, selected bool, width int) string {
	inner := width - 2
	if inner < 8 {
		inner = 8
	}

	var cover string
	if rendered, ok := cachedCover(m.CoverURL(), inner, coverCellHeight); ok && rendered != "" {
		cover = rendered
	} else {
		cover = CoverPlaceholder(inner, coverCellHeight, m.DisplayTitle())
	}

	title := runeTruncate(m.DisplayTitle(), inner)
	titleLine := lipgloss.NewStyle().Width(inner).Render(title)
	if selected {
		titleLine = core.SelectedStyle.Width(inner).Render(title)
	}

	format := m.Type
	if m.Format != nil && *m.Format != "" {
		format = *m.Format
	}
	year := ""
	if m.SeasonYear != nil {
		year = fmt.Sprintf(" · %d", *m.SeasonYear)
	} else if m.Year != nil {
		year = fmt.Sprintf(" · %d", *m.Year)
	}
	meta := core.MutedStyle.Render(runeTruncate(humanizeLabel(format)+year, inner))
	score := core.MutedStyle.Render("★ ") + scoreStyled(m.Score)

	body := lipgloss.JoinVertical(lipgloss.Left, cover, titleLine, meta, score)

	borderColor := lipgloss.Color("#3D4A5C")
	if selected {
		borderColor = core.AccentColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(inner).
		Render(body)
}

// cardStrip renders a single horizontal row of media cards with a label line
// above each, truncated to width. Returns the strip plus the items actually
// shown, left to right, so callers can hit-test mouse clicks.
func cardStrip(items []api.Media, labels []string, width int) (string, []api.Media) {
	stride := gridCellWidth + gridColGap
	maxCards := max(1, (width+gridColGap)/stride)
	parts := make([]string, 0, maxCards*2)
	shown := make([]api.Media, 0, maxCards)
	for i, m := range items {
		if i >= maxCards {
			break
		}
		label := ""
		if labels != nil && i < len(labels) {
			label = labels[i]
		}
		if len(parts) > 0 {
			parts = append(parts, strings.Repeat(" ", gridColGap))
		}
		parts = append(parts, lipgloss.JoinVertical(lipgloss.Left,
			core.SubtleStyle.Render(runeTruncate(label, gridCellWidth)),
			MediaCard(m, false, gridCellWidth)))
		shown = append(shown, m)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...), shown
}
