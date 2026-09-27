package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

const searchPerPage = 20

type searchResultMsg struct {
	results []api.Media
	hasNext bool
	page    int
	err     error
}

type Search struct {
	client  *api.Client
	input   textinput.Model
	filters *FilterBar
	spinner spinner.Model
	grid    *Grid

	page       int
	hasNext    bool
	loading    bool
	searched   bool
	err        error
	filterMode bool

	width  int
	height int

	gridTop int // body-relative line where the grid starts, for mouse hit-tests
}

func NewSearch(client *api.Client) *Search {
	input := textinput.New()
	input.Placeholder = "Search anime or manga…"
	input.Prompt = "/ "
	input.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = core.AccentStyle

	return &Search{
		client:  client,
		input:   input,
		filters: NewFilterBar(),
		spinner: sp,
		grid:    NewGrid(),
		page:    1,
	}
}

func (s *Search) Init() tea.Cmd {
	return textinput.Blink
}

func (s *Search) runSearch(page int) tea.Cmd {
	s.loading = true
	s.err = nil
	client := s.client
	query := strings.TrimSpace(s.input.Value())
	mediaType, genre, status, format, season, year, sort := s.filters.Values()
	return tea.Batch(s.spinner.Tick, func() tea.Msg {
		media, hasNext, err := client.SearchMedia(query, mediaType, genre, status, format, sort, season, year, page, searchPerPage, false)
		return searchResultMsg{results: media, hasNext: hasNext, page: page, err: err}
	})
}

func (s *Search) fetchCovers() tea.Cmd {
	if !ShowImages {
		return nil
	}
	inner := s.grid.CellWidth() - 2
	if inner < 8 {
		inner = 8 // match MediaCard's clamp so the cache key lines up
	}
	cmds := make([]tea.Cmd, 0)
	for _, m := range s.grid.VisibleItems() {
		url := m.CoverURL()
		if url == "" {
			continue
		}
		if _, ok := cachedCover(url, inner, coverCellHeight); ok {
			continue
		}
		cmds = append(cmds, FetchCoverCmd(url, inner, coverCellHeight))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (s *Search) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.SizedMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.input.Width = max(10, msg.Width-8)
		s.grid.SetSize(msg.Width, max(1, msg.Height-10))
		return s, s.fetchCovers()

	case searchResultMsg:
		s.loading = false
		s.searched = true
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		s.err = nil
		s.page = msg.page
		s.hasNext = msg.hasNext
		s.grid.SetItems(msg.results)
		return s, s.fetchCovers()

	case CoverReadyMsg:
		storeCover(msg.URL, msg.W, msg.H, msg.Rendered)
		return s, coverSettle()

	case coverSettleMsg:
		return s, nil

	case spinner.TickMsg:
		if !s.loading {
			return s, nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case tea.KeyMsg:
		return s.handleKey(msg)

	case tea.MouseMsg:
		return s.handleMouse(msg)
	}
	return s, nil
}

func (s *Search) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if s.input.Focused() || s.filterMode || s.loading || s.grid.Len() == 0 {
		return s, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		s.grid.Update(tea.KeyMsg{Type: tea.KeyUp})
		return s, s.fetchCovers()
	case tea.MouseButtonWheelDown:
		s.grid.Update(tea.KeyMsg{Type: tea.KeyDown})
		return s, s.fetchCovers()
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		idx, ok := s.grid.IndexAt(msg.X, msg.Y-s.gridTop)
		if !ok {
			return s, nil
		}
		if idx == s.grid.Cursor() {
			if m, ok := s.grid.Selected(); ok {
				id := m.ID
				return s, func() tea.Msg { return core.OpenDetailsMsg{MediaID: id} }
			}
			return s, nil
		}
		s.grid.SetCursor(idx)
	}
	return s, nil
}

func (s *Search) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s.input.Focused() {
		switch msg.String() {
		case "esc":
			s.input.Blur()
			return s, nil
		case "enter":
			s.input.Blur()
			return s, s.runSearch(1)
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}

	if s.filterMode {
		if msg.String() == "esc" {
			s.filterMode = false
			return s, nil
		}
		if s.filters.Update(msg) {
			return s, s.runSearch(1)
		}
		return s, nil
	}

	switch {
	case key.Matches(msg, core.Keys.SearchSlash):
		return s, s.input.Focus()
	case msg.String() == "tab":
		s.filterMode = true
		return s, nil
	case key.Matches(msg, core.Keys.Select):
		if m, ok := s.grid.Selected(); ok {
			id := m.ID
			return s, func() tea.Msg { return core.OpenDetailsMsg{MediaID: id} }
		}
		return s, nil
	case msg.String() == "n":
		if s.hasNext && !s.loading {
			return s, s.runSearch(s.page + 1)
		}
		return s, nil
	case msg.String() == "p":
		if s.page > 1 && !s.loading {
			return s, s.runSearch(s.page - 1)
		}
		return s, nil
	case key.Matches(msg, core.Keys.Refresh):
		if !s.loading && s.searched {
			return s, s.runSearch(s.page)
		}
		return s, nil
	default:
		s.grid.Update(msg)
		return s, s.fetchCovers()
	}
}

func (s *Search) View() string {
	var b strings.Builder
	b.WriteString(core.TitleStyle.Render("Search"))
	b.WriteString("\n\n")
	b.WriteString(s.input.View())
	b.WriteString("\n\n")
	b.WriteString(s.filters.View(s.filterMode))
	b.WriteString("\n\n")
	b.WriteString(s.statusLine())
	b.WriteString("\n\n")
	if s.grid.Len() > 0 {
		s.gridTop = strings.Count(b.String(), "\n")
		b.WriteString(s.grid.View())
		b.WriteString("\n\n")
	}
	b.WriteString(core.HelpStyle.Render("/ search · tab filters · hjkl move · enter open · n/p page · r refresh"))
	return b.String()
}

func (s *Search) statusLine() string {
	switch {
	case s.loading:
		return s.spinner.View() + " Searching…"
	case s.err != nil:
		return core.ErrorStyle.Render("Error: " + s.err.Error())
	case !s.searched:
		return core.SubtleStyle.Render("Type a title and press enter, or tab to browse with filters.")
	case s.grid.Len() == 0:
		return core.SubtleStyle.Render("No results found.")
	}
	label := fmt.Sprintf("%d results · Page %d", s.grid.Len(), s.page)
	if s.page > 1 {
		label = "← " + label
	}
	if s.hasNext {
		label += " · more →"
	}
	return core.SubtleStyle.Render(label)
}
