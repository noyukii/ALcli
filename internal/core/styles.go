package core

import "github.com/charmbracelet/lipgloss"

const AccentColor = lipgloss.Color("#3DB4F2")

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor)

	SubtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7A8C"))

	MutedStyle = SubtleStyle

	AccentStyle = lipgloss.NewStyle().
			Foreground(AccentColor)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3D4A5C")).
			Padding(0, 1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0B1622")).
			Background(AccentColor).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7A8C")).
			Italic(true)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0B1622")).
			Background(AccentColor).
			Bold(true).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7A8C")).
				Padding(0, 1)

	tabDisabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3D4A5C")).
				Padding(0, 1)
)

var tabs = []struct {
	route Route
	label string
}{
	{RouteHome, "1 Home"},
	{RouteSearch, "2 Search"},
	{RouteMyList, "3 My List"},
	{RouteProfile, "4 Profile"},
}

// TabBar renders the top navigation strip with the active route highlighted.
// When enabled is false (a view is stacked on top of the root, so the number
// keys are inert), all tabs render dimmed and without a highlight.
func TabBar(active Route, enabled bool) string {
	rendered := make([]string, 0, len(tabs))
	for _, t := range tabs {
		switch {
		case !enabled:
			rendered = append(rendered, tabDisabledStyle.Render(t.label))
		case t.route == active:
			rendered = append(rendered, tabActiveStyle.Render(t.label))
		default:
			rendered = append(rendered, tabInactiveStyle.Render(t.label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// TabAt returns the route whose tab contains the given x cell, for mouse
// hit-testing on the tab bar line.
func TabAt(x int) (Route, bool) {
	pos := 0
	for _, t := range tabs {
		w := len(t.label) + 2 // tab horizontal padding
		if x >= pos && x < pos+w {
			return t.route, true
		}
		pos += w
	}
	return 0, false
}
