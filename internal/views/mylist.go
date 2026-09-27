package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

type myListRow struct {
	header  string
	entry   api.MediaList
	isEntry bool
}

type myListLoadedMsg struct {
	groups []api.MediaListGroup
	err    error
}

type myListProgressSavedMsg struct{ err error }

type myListDeletedMsg struct{ err error }

type MyList struct {
	client *api.Client
	userID int
	width  int
	height int

	mediaType string
	loading   bool
	spinner   spinner.Model
	groups    []api.MediaListGroup
	rows      []myListRow
	cursor    int
	offset    int

	loadErr          string
	actionErr        string
	confirmingDelete bool
}

func NewMyList(client *api.Client, userID int) *MyList {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = core.AccentStyle
	return &MyList{
		client:    client,
		userID:    userID,
		mediaType: "ANIME",
		loading:   true,
		spinner:   sp,
	}
}

func (m *MyList) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *MyList) fetchCmd() tea.Cmd {
	client := m.client
	userID := m.userID
	mediaType := m.mediaType
	return func() tea.Msg {
		groups, err := client.GetMediaList(userID, mediaType)
		return myListLoadedMsg{groups: groups, err: err}
	}
}

func (m *MyList) refetch() tea.Cmd {
	m.loading = true
	m.loadErr = ""
	m.actionErr = ""
	return tea.Batch(m.spinner.Tick, m.fetchCmd())
}

func (m *MyList) rebuildRows() {
	rows := make([]myListRow, 0)
	for _, g := range m.groups {
		entries := append([]api.MediaList(nil), g.Entries...)
		sort.SliceStable(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Media.DisplayTitle()) < strings.ToLower(entries[j].Media.DisplayTitle())
		})
		label := api.MediaList{Status: g.Status, Media: api.Media{Type: m.mediaType}}.StatusDisplay()
		rows = append(rows, myListRow{header: fmt.Sprintf("%s (%d)", label, len(entries))})
		for _, e := range entries {
			rows = append(rows, myListRow{entry: e, isEntry: true})
		}
	}
	m.rows = rows
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if len(rows) > 0 && !rows[m.cursor].isEntry {
		m.moveCursor(1)
	}
	m.ensureVisible()
}

func (m *MyList) moveCursor(delta int) {
	if len(m.rows) == 0 {
		return
	}
	i := m.cursor
	for {
		i += delta
		if i < 0 || i >= len(m.rows) {
			return
		}
		if m.rows[i].isEntry {
			m.cursor = i
			return
		}
	}
}

func (m *MyList) selectedEntry() *api.MediaList {
	if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].isEntry {
		e := m.rows[m.cursor].entry
		return &e
	}
	return nil
}

func (m *MyList) visibleRows() int {
	v := m.height - 6
	if v < 5 {
		v = 5
	}
	return v
}

func (m *MyList) ensureVisible() {
	visible := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func myListTotal(media api.Media) int {
	if media.Type == "ANIME" {
		if media.Episodes != nil {
			return *media.Episodes
		}
		return 0
	}
	if media.Chapters != nil {
		return *media.Chapters
	}
	return 0
}

func myListEntryMedia(entry api.MediaList) api.Media {
	media := entry.Media
	id := entry.ID
	status := entry.Status
	progress := entry.Progress
	score := entry.Score
	media.ListEntryID = &id
	media.ListStatus = &status
	media.ListProgress = &progress
	media.ListScore = &score
	if entry.Notes != nil {
		notes := *entry.Notes
		media.ListNotes = &notes
	}
	return media
}

func (m *MyList) adjustProgress(delta int) tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}
	newProgress := entry.Progress + delta
	if newProgress < 0 {
		newProgress = 0
	}
	if total := myListTotal(entry.Media); total > 0 && newProgress > total {
		newProgress = total
	}
	if newProgress == entry.Progress {
		return nil
	}
	for gi := range m.groups {
		for ei := range m.groups[gi].Entries {
			if m.groups[gi].Entries[ei].ID == entry.ID {
				m.groups[gi].Entries[ei].Progress = newProgress
			}
		}
	}
	updated := *entry
	updated.Progress = newProgress
	m.rows[m.cursor].entry = updated
	return m.saveProgressCmd(updated)
}

func (m *MyList) saveProgressCmd(entry api.MediaList) tea.Cmd {
	client := m.client
	entryID := entry.ID
	notes := ""
	if entry.Notes != nil {
		notes = *entry.Notes
	}
	return func() tea.Msg {
		_, err := client.SaveListEntry(entry.Media.ID, entry.Status, entry.Progress, entry.Score, notes, &entryID, entry.Repeat, entry.Private)
		return myListProgressSavedMsg{err: err}
	}
}

func (m *MyList) deleteCmd(entryID int) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		_, err := client.DeleteListEntry(entryID)
		return myListDeletedMsg{err: err}
	}
}

func (m *MyList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case myListLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err.Error()
			return m, nil
		}
		m.groups = msg.groups
		m.cursor = 0
		m.offset = 0
		m.rebuildRows()
		return m, nil
	case myListProgressSavedMsg:
		if msg.err != nil {
			m.actionErr = msg.err.Error()
			return m, m.refetch()
		}
		return m, nil
	case myListDeletedMsg:
		if msg.err != nil {
			m.actionErr = msg.err.Error()
			return m, nil
		}
		return m, m.refetch()
	case EntrySavedMsg:
		return m, m.refetch()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *MyList) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmingDelete {
		switch msg.String() {
		case "y", "Y":
			m.confirmingDelete = false
			if entry := m.selectedEntry(); entry != nil {
				return m, m.deleteCmd(entry.ID)
			}
		case "n", "N", "esc":
			m.confirmingDelete = false
		}
		return m, nil
	}
	if m.loading {
		return m, nil
	}
	if m.loadErr != "" {
		if key.Matches(msg, core.Keys.Refresh) {
			return m, m.refetch()
		}
		return m, nil
	}
	switch msg.String() {
	case "tab", "left", "right":
		if m.mediaType == "ANIME" {
			m.mediaType = "MANGA"
		} else {
			m.mediaType = "ANIME"
		}
		return m, m.refetch()
	case "+", "=":
		return m, m.adjustProgress(1)
	case "-":
		return m, m.adjustProgress(-1)
	case "d":
		if m.selectedEntry() != nil {
			m.confirmingDelete = true
		}
		return m, nil
	}
	switch {
	case key.Matches(msg, core.Keys.Refresh):
		return m, m.refetch()
	case key.Matches(msg, core.Keys.Up):
		m.moveCursor(-1)
		m.ensureVisible()
	case key.Matches(msg, core.Keys.Down):
		m.moveCursor(1)
		m.ensureVisible()
	case key.Matches(msg, core.Keys.Select):
		if entry := m.selectedEntry(); entry != nil {
			id := entry.Media.ID
			return m, func() tea.Msg { return core.OpenDetailsMsg{MediaID: id} }
		}
	case key.Matches(msg, core.Keys.Edit):
		if entry := m.selectedEntry(); entry != nil {
			media := myListEntryMedia(*entry)
			return m, func() tea.Msg { return core.EditEntryMsg{Media: media} }
		}
	}
	return m, nil
}

func (m *MyList) renderRow(row myListRow, selected bool) string {
	if !row.isEntry {
		return core.AccentStyle.Bold(true).Render(row.header)
	}
	e := row.entry
	progress := fmt.Sprintf("%d", e.Progress)
	if total := myListTotal(e.Media); total > 0 {
		progress = fmt.Sprintf("%d/%d", e.Progress, total)
	}
	line := fmt.Sprintf("  %-40s %10s %6s  %s",
		runeTruncate(e.Media.DisplayTitle(), 40), progress, e.DisplayScore(), e.StatusDisplay())
	if selected {
		return core.SelectedStyle.Render(runePad(line, m.rowWidth()))
	}
	return line
}

func (m *MyList) rowWidth() int {
	w := m.width - 2
	if w < 20 {
		w = 20
	}
	if w > 100 {
		w = 100
	}
	return w
}

func (m *MyList) View() string {
	var b strings.Builder
	animeBtn := " Anime "
	mangaBtn := " Manga "
	if m.mediaType == "ANIME" {
		animeBtn = core.SelectedStyle.Render(animeBtn)
		mangaBtn = core.SubtleStyle.Render(mangaBtn)
	} else {
		animeBtn = core.SubtleStyle.Render(animeBtn)
		mangaBtn = core.SelectedStyle.Render(mangaBtn)
	}
	b.WriteString(core.TitleStyle.Render("My List") + "  " + core.SubtleStyle.Render("Type:") + " " + animeBtn + " " + mangaBtn + "\n\n")

	switch {
	case m.loading:
		b.WriteString(m.spinner.View() + " Loading list…\n")
	case m.loadErr != "":
		b.WriteString(core.ErrorStyle.Render("Failed to load list: "+m.loadErr) + "\n")
		b.WriteString(core.HelpStyle.Render("r retry") + "\n")
	case len(m.rows) == 0:
		b.WriteString(core.SubtleStyle.Render("No entries yet.") + "\n")
	default:
		end := m.offset + m.visibleRows()
		if end > len(m.rows) {
			end = len(m.rows)
		}
		for i := m.offset; i < end; i++ {
			b.WriteString(m.renderRow(m.rows[i], i == m.cursor) + "\n")
		}
	}

	if m.confirmingDelete {
		if entry := m.selectedEntry(); entry != nil {
			b.WriteString(core.ErrorStyle.Render(fmt.Sprintf("Delete %q from your list? (y/n)", entry.Media.DisplayTitle())) + "\n")
		}
	}
	if m.actionErr != "" {
		b.WriteString(core.ErrorStyle.Render(m.actionErr) + "\n")
	}
	b.WriteString(core.HelpStyle.Render("j/k move · enter details · e edit · +/- progress · d delete · tab anime/manga · r refresh"))
	return b.String()
}
