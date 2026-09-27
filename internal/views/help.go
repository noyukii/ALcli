package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noyukii/ALcli/internal/core"
)

type helpEntry struct {
	key  string
	desc string
}

type helpSection struct {
	title   string
	entries []helpEntry
}

var helpSections = []helpSection{
	{"Navigation", []helpEntry{
		{"1", "Go to Home (Trending)"},
		{"2", "Go to Search"},
		{"3", "Go to My List"},
		{"4", "Go to Profile"},
		{"q", "Quit / go back"},
		{"Esc", "Go back / close modal"},
	}},
	{"Movement", []helpEntry{
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"h / ←", "Move left"},
		{"l / →", "Move right"},
		{"Tab", "Switch tabs"},
	}},
	{"Search", []helpEntry{
		{"/", "Focus search input"},
		{"Enter", "Submit search / select item"},
		{"Esc", "Clear search"},
	}},
	{"My List", []helpEntry{
		{"Enter", "Open media details"},
		{"e", "Edit list entry"},
		{"r", "Refresh list"},
	}},
	{"Media Details", []helpEntry{
		{"e", "Add to / edit list entry"},
		{"Esc", "Go back"},
	}},
	{"General", []helpEntry{
		{"r", "Refresh current view"},
		{"?", "Show this help"},
		{"q", "Quit"},
	}},
}

type Help struct {
	width  int
	height int
}

func NewHelp() *Help {
	return &Help{}
}

func (h *Help) Init() tea.Cmd {
	return nil
}

func (h *Help) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height
		return h, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "enter", "?":
			return h, func() tea.Msg { return core.PopViewMsg{} }
		}
	}
	return h, nil
}

func (h *Help) View() string {
	var b strings.Builder
	b.WriteString(core.TitleStyle.Render("AniList CLI — Keyboard Shortcuts"))
	for _, sec := range helpSections {
		b.WriteString("\n\n")
		b.WriteString(core.AccentStyle.Bold(true).Render(sec.title))
		for _, e := range sec.entries {
			b.WriteString("\n  ")
			b.WriteString(core.SelectedStyle.Render(fmt.Sprintf("%-8s", e.key)))
			b.WriteString("  ")
			b.WriteString(core.SubtleStyle.Render(e.desc))
		}
	}
	b.WriteString("\n\n")
	b.WriteString(core.HelpStyle.Render("esc / q / enter close"))

	box := core.PanelStyle.Render(b.String())
	if h.width > 0 && h.height > 0 {
		return lipgloss.Place(h.width, max(1, h.height-1), lipgloss.Center, lipgloss.Center, box)
	}
	return box
}
