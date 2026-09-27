package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

type editEntryField int

const (
	editFieldStatus editEntryField = iota
	editFieldProgress
	editFieldScore
	editFieldNotes
	editFieldSave
	editFieldDelete
)

var editStatusOrder = []string{"CURRENT", "PLANNING", "COMPLETED", "PAUSED", "DROPPED", "REPEATING"}

type editEntrySaveResultMsg struct{ err error }

type editEntryDeleteResultMsg struct{ err error }

type EditEntry struct {
	client *api.Client
	media  api.Media
	width  int
	height int

	fields []editEntryField
	focus  int
	status string

	progressInput textinput.Model
	scoreInput    textinput.Model
	notesInput    textinput.Model

	spinner       spinner.Model
	saving        bool
	deleting      bool
	confirmDelete bool
	errMsg        string
}

func NewEditEntry(client *api.Client, media api.Media) *EditEntry {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = core.AccentStyle

	e := &EditEntry{
		client:  client,
		media:   media,
		status:  "PLANNING",
		spinner: sp,
	}
	if media.ListStatus != nil && *media.ListStatus != "" {
		e.status = *media.ListStatus
	}

	progress := 0
	if media.ListProgress != nil {
		progress = *media.ListProgress
	}
	score := 0.0
	if media.ListScore != nil {
		score = *media.ListScore / 10
	}
	notes := ""
	if media.ListNotes != nil {
		notes = *media.ListNotes
	}

	pi := textinput.New()
	pi.SetValue(strconv.Itoa(progress))
	pi.Placeholder = "0"
	pi.CharLimit = 6
	pi.Width = 10
	e.progressInput = pi

	si := textinput.New()
	si.SetValue(strconv.FormatFloat(score, 'f', 1, 64))
	si.Placeholder = "0.0"
	si.CharLimit = 5
	si.Width = 10
	e.scoreInput = si

	ni := textinput.New()
	ni.SetValue(notes)
	ni.CharLimit = 200
	ni.Width = 44
	e.notesInput = ni

	e.fields = []editEntryField{editFieldStatus, editFieldProgress, editFieldScore, editFieldNotes, editFieldSave}
	if e.editing() {
		e.fields = append(e.fields, editFieldDelete)
	}
	return e
}

func (e *EditEntry) editing() bool {
	return e.media.ListEntryID != nil
}

func (e *EditEntry) Init() tea.Cmd {
	return nil
}

func (e *EditEntry) blurInputs() {
	e.progressInput.Blur()
	e.scoreInput.Blur()
	e.notesInput.Blur()
}

func (e *EditEntry) focusInputs() tea.Cmd {
	e.blurInputs()
	switch e.fields[e.focus] {
	case editFieldProgress:
		return e.progressInput.Focus()
	case editFieldScore:
		return e.scoreInput.Focus()
	case editFieldNotes:
		return e.notesInput.Focus()
	}
	return nil
}

func (e *EditEntry) moveFocus(delta int) tea.Cmd {
	e.focus = (e.focus + delta + len(e.fields)) % len(e.fields)
	return e.focusInputs()
}

func (e *EditEntry) cycleStatus(delta int) {
	idx := 0
	for i, s := range editStatusOrder {
		if s == e.status {
			idx = i
			break
		}
	}
	e.status = editStatusOrder[(idx+delta+len(editStatusOrder))%len(editStatusOrder)]
}

func (e *EditEntry) progressTotal() int {
	if e.media.Type == "ANIME" {
		if e.media.Episodes != nil {
			return *e.media.Episodes
		}
		return 0
	}
	if e.media.Chapters != nil {
		return *e.media.Chapters
	}
	return 0
}

func (e *EditEntry) bumpProgress(delta int) {
	p, _ := strconv.Atoi(strings.TrimSpace(e.progressInput.Value()))
	p += delta
	if p < 0 {
		p = 0
	}
	if total := e.progressTotal(); total > 0 && p > total {
		p = total
	}
	e.progressInput.SetValue(strconv.Itoa(p))
}

func (e *EditEntry) bumpScore(delta float64) {
	s, _ := strconv.ParseFloat(strings.TrimSpace(e.scoreInput.Value()), 64)
	s += delta
	if s < 0 {
		s = 0
	}
	if s > 10 {
		s = 10
	}
	e.scoreInput.SetValue(strconv.FormatFloat(s, 'f', 1, 64))
}

func (e *EditEntry) saveCmd(progress int, score float64) tea.Cmd {
	client := e.client
	mediaID := e.media.ID
	status := e.status
	notes := strings.TrimSpace(e.notesInput.Value())
	entryID := e.media.ListEntryID
	return func() tea.Msg {
		_, err := client.SaveListEntry(mediaID, status, progress, score, notes, entryID, 0, false)
		return editEntrySaveResultMsg{err: err}
	}
}

func (e *EditEntry) deleteCmd() tea.Cmd {
	client := e.client
	entryID := *e.media.ListEntryID
	return func() tea.Msg {
		_, err := client.DeleteListEntry(entryID)
		return editEntryDeleteResultMsg{err: err}
	}
}

func (e *EditEntry) save() (tea.Model, tea.Cmd) {
	progressStr := strings.TrimSpace(e.progressInput.Value())
	progress := 0
	if progressStr != "" {
		p, err := strconv.Atoi(progressStr)
		if err != nil || p < 0 {
			e.errMsg = "Progress must be a whole number."
			return e, nil
		}
		progress = p
	}
	scoreStr := strings.TrimSpace(e.scoreInput.Value())
	score := 0.0
	if scoreStr != "" {
		s, err := strconv.ParseFloat(scoreStr, 64)
		if err != nil || s < 0 || s > 10 {
			e.errMsg = "Score must be between 0.0 and 10.0."
			return e, nil
		}
		score = s
	}
	e.saving = true
	e.errMsg = ""
	return e, tea.Batch(e.spinner.Tick, e.saveCmd(progress, score*10))
}

func (e *EditEntry) successCmds() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return EntrySavedMsg{} },
		func() tea.Msg { return core.PopViewMsg{} },
	)
}

func (e *EditEntry) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.height = msg.Height
		return e, nil
	case spinner.TickMsg:
		if e.saving || e.deleting {
			var cmd tea.Cmd
			e.spinner, cmd = e.spinner.Update(msg)
			return e, cmd
		}
		return e, nil
	case editEntrySaveResultMsg:
		e.saving = false
		if msg.err != nil {
			e.errMsg = "Save failed: " + msg.err.Error()
			return e, nil
		}
		return e, e.successCmds()
	case editEntryDeleteResultMsg:
		e.deleting = false
		if msg.err != nil {
			e.errMsg = "Delete failed: " + msg.err.Error()
			return e, nil
		}
		return e, e.successCmds()
	case tea.KeyMsg:
		return e.handleKey(msg)
	}
	return e, nil
}

func (e *EditEntry) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if e.confirmDelete {
		switch msg.String() {
		case "y", "Y":
			e.confirmDelete = false
			if e.editing() {
				e.deleting = true
				e.errMsg = ""
				return e, tea.Batch(e.spinner.Tick, e.deleteCmd())
			}
		case "n", "N", "esc":
			e.confirmDelete = false
		}
		return e, nil
	}
	if e.saving || e.deleting {
		return e, nil
	}
	switch msg.String() {
	case "ctrl+s":
		return e.save()
	case "esc":
		return e, func() tea.Msg { return core.PopViewMsg{} }
	case "tab":
		return e, e.moveFocus(1)
	case "shift+tab":
		return e, e.moveFocus(-1)
	case "d":
		if e.editing() {
			e.confirmDelete = true
		}
		return e, nil
	}
	var cmd tea.Cmd
	switch e.fields[e.focus] {
	case editFieldStatus:
		switch msg.String() {
		case "left", "h":
			e.cycleStatus(-1)
		case "right", "l", " ":
			e.cycleStatus(1)
		}
	case editFieldProgress:
		switch msg.String() {
		case "+", "=":
			e.bumpProgress(1)
		case "-":
			e.bumpProgress(-1)
		default:
			e.progressInput, cmd = e.progressInput.Update(msg)
		}
	case editFieldScore:
		switch msg.String() {
		case "+", "=":
			e.bumpScore(0.5)
		case "-":
			e.bumpScore(-0.5)
		default:
			e.scoreInput, cmd = e.scoreInput.Update(msg)
		}
	case editFieldNotes:
		e.notesInput, cmd = e.notesInput.Update(msg)
	case editFieldSave:
		if msg.String() == "enter" {
			return e.save()
		}
	case editFieldDelete:
		if msg.String() == "enter" {
			e.confirmDelete = true
		}
	}
	return e, cmd
}

func editStatusLabel(status string, isAnime bool) string {
	switch status {
	case "CURRENT":
		if isAnime {
			return "Watching"
		}
		return "Reading"
	case "COMPLETED":
		return "Completed"
	case "PAUSED":
		return "On Hold"
	case "DROPPED":
		return "Dropped"
	case "PLANNING":
		if isAnime {
			return "Plan to Watch"
		}
		return "Plan to Read"
	case "REPEATING":
		if isAnime {
			return "Rewatching"
		}
		return "Rereading"
	}
	return status
}

func (e *EditEntry) fieldLabel(f editEntryField, text string) string {
	if e.fields[e.focus] == f {
		return core.AccentStyle.Render("▸ " + text)
	}
	return "  " + text
}

func (e *EditEntry) View() string {
	const panelWidth = 50
	isAnime := e.media.Type == "ANIME"
	focused := e.fields[e.focus]

	title := "Add to List"
	if e.editing() {
		title = "Edit Entry"
	}

	progressWord := "Chapter"
	if isAnime {
		progressWord = "Episode"
	}
	progressLabel := progressWord + " Progress"
	if total := e.progressTotal(); total > 0 {
		progressLabel += fmt.Sprintf(" / %d", total)
	}

	statusValue := "   " + editStatusLabel(e.status, isAnime)
	if focused == editFieldStatus {
		statusValue = core.SelectedStyle.Render(" ‹ " + editStatusLabel(e.status, isAnime) + " › ")
	}

	saveBtn := "[ Save ]"
	if focused == editFieldSave {
		saveBtn = core.SelectedStyle.Render(saveBtn)
	}
	buttonRow := saveBtn
	if e.editing() {
		deleteBtn := "[ Delete ]"
		if focused == editFieldDelete {
			deleteBtn = core.SelectedStyle.Render(deleteBtn)
		}
		buttonRow += "  " + deleteBtn
	}

	lines := []string{
		lipgloss.PlaceHorizontal(panelWidth-4, lipgloss.Center, core.TitleStyle.Render(title)),
		lipgloss.PlaceHorizontal(panelWidth-4, lipgloss.Center, core.SubtleStyle.Render(runeTruncate(e.media.DisplayTitle(), panelWidth-8))),
		"",
		e.fieldLabel(editFieldStatus, "Status"),
		statusValue,
		"",
		e.fieldLabel(editFieldProgress, progressLabel),
		e.progressInput.View(),
		"",
		e.fieldLabel(editFieldScore, "Score (0.0 – 10.0)"),
		e.scoreInput.View(),
		"",
		e.fieldLabel(editFieldNotes, "Notes"),
		e.notesInput.View(),
		"",
	}

	if e.saving {
		lines = append(lines, e.spinner.View()+" Saving…")
	} else if e.deleting {
		lines = append(lines, e.spinner.View()+" Deleting…")
	} else if e.confirmDelete {
		lines = append(lines, core.ErrorStyle.Render("Delete this entry? (y/n)"))
	} else {
		lines = append(lines, buttonRow)
	}
	if e.errMsg != "" {
		lines = append(lines, core.ErrorStyle.Render(e.errMsg))
	}
	lines = append(lines, core.HelpStyle.Render("tab next · ←/→ change status · +/- adjust · ctrl+s save · esc cancel"))

	panel := core.PanelStyle.Width(panelWidth).Render(strings.Join(lines, "\n"))
	if e.width > 0 && e.height > 0 {
		return lipgloss.Place(e.width, e.height, lipgloss.Center, lipgloss.Center, panel)
	}
	return panel
}
