package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

const homePerPage = 20

type homeLoadedMsg struct {
	items   []api.Media
	page    int
	hasNext bool
	err     error
}

type Home struct {
	client  *api.Client
	grid    *Grid
	spinner spinner.Model

	page    int
	hasNext bool
	loading bool
	loaded  bool
	errMsg  string

	width  int
	height int

	gridTop int // body-relative line where the grid starts, for mouse hit-tests
}

func NewHome(client *api.Client) *Home {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = core.AccentStyle
	return &Home{
		client:  client,
		grid:    NewGrid(),
		spinner: sp,
		page:    1,
		loading: true,
	}
}

func (h *Home) Init() tea.Cmd {
	return tea.Batch(h.spinner.Tick, h.load(1))
}

func (h *Home) load(page int) tea.Cmd {
	client := h.client
	return func() tea.Msg {
		items, hasNext, err := client.GetTrending("ANIME", page, homePerPage)
		return homeLoadedMsg{items: items, page: page, hasNext: hasNext, err: err}
	}
}

func (h *Home) fetchCovers() tea.Cmd {
	if !ShowImages {
		return nil
	}
	inner := h.grid.CellWidth() - 2
	if inner < 8 {
		inner = 8 // match MediaCard's clamp so the cache key lines up
	}
	cmds := make([]tea.Cmd, 0)
	for _, m := range h.grid.VisibleItems() {
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

func (h *Home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.SizedMsg:
		h.width = msg.Width
		h.height = msg.Height
		h.grid.SetSize(msg.Width, max(1, msg.Height-4))
		return h, h.fetchCovers()

	case spinner.TickMsg:
		if !h.loading {
			return h, nil
		}
		var cmd tea.Cmd
		h.spinner, cmd = h.spinner.Update(msg)
		return h, cmd

	case homeLoadedMsg:
		h.loading = false
		h.loaded = true
		if msg.err != nil {
			h.errMsg = msg.err.Error()
			return h, nil
		}
		h.errMsg = ""
		h.page = msg.page
		h.hasNext = msg.hasNext
		h.grid.SetItems(msg.items)
		return h, h.fetchCovers()

	case CoverReadyMsg:
		storeCover(msg.URL, msg.W, msg.H, msg.Rendered)
		return h, coverSettle()

	case coverSettleMsg:
		return h, nil

	case tea.KeyMsg:
		return h.handleKey(msg)

	case tea.MouseMsg:
		return h.handleMouse(msg)
	}
	return h, nil
}

func (h *Home) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if h.loading || h.grid.Len() == 0 {
		return h, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		h.grid.Update(tea.KeyMsg{Type: tea.KeyUp})
		return h, h.fetchCovers()
	case tea.MouseButtonWheelDown:
		h.grid.Update(tea.KeyMsg{Type: tea.KeyDown})
		return h, h.fetchCovers()
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		idx, ok := h.grid.IndexAt(msg.X, msg.Y-h.gridTop)
		if !ok {
			return h, nil
		}
		if idx == h.grid.Cursor() {
			if m, ok := h.grid.Selected(); ok {
				id := m.ID
				return h, func() tea.Msg { return core.OpenDetailsMsg{MediaID: id} }
			}
			return h, nil
		}
		h.grid.SetCursor(idx)
	}
	return h, nil
}

func (h *Home) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if h.loading {
		return h, nil
	}
	switch {
	case key.Matches(msg, core.Keys.Refresh):
		return h, h.reload(h.page)
	case msg.String() == "n":
		if h.hasNext {
			return h, h.reload(h.page + 1)
		}
		return h, nil
	case msg.String() == "p":
		if h.page > 1 {
			return h, h.reload(h.page - 1)
		}
		return h, nil
	case key.Matches(msg, core.Keys.Select):
		if m, ok := h.grid.Selected(); ok {
			id := m.ID
			return h, func() tea.Msg { return core.OpenDetailsMsg{MediaID: id} }
		}
		return h, nil
	default:
		h.grid.Update(msg)
		return h, h.fetchCovers()
	}
}

func (h *Home) reload(page int) tea.Cmd {
	h.loading = true
	h.errMsg = ""
	return tea.Batch(h.spinner.Tick, h.load(page))
}

func (h *Home) View() string {
	var b strings.Builder
	b.WriteString(core.TitleStyle.Render("Trending Anime"))
	b.WriteString("\n")

	switch {
	case h.loading:
		b.WriteString("\n" + h.spinner.View() + " Loading…")
	case h.errMsg != "":
		b.WriteString("\n" + core.ErrorStyle.Render("Failed to load: "+h.errMsg) + "\n\n" + core.HelpStyle.Render("r retry"))
	case h.grid.Len() == 0:
		b.WriteString("\n" + core.SubtleStyle.Render("Nothing trending right now."))
	default:
		// Prefix ends with a newline, so the newline count is the grid's line.
		h.gridTop = strings.Count(b.String(), "\n")
		b.WriteString(h.grid.View())
		b.WriteString("\n")
		b.WriteString(h.pageLine())
		b.WriteString("\n")
		b.WriteString(core.HelpStyle.Render("hjkl/arrows move · enter open · n/p page · r refresh"))
	}
	return b.String()
}

func (h *Home) pageLine() string {
	label := fmt.Sprintf("Page %d", h.page)
	if h.hasNext {
		label += " · more →"
	}
	if h.page > 1 {
		label = "← " + label
	}
	return core.SubtleStyle.Render(label)
}
