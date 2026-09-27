package views

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

type EntrySavedMsg struct{}

type detailsLoadedMsg struct{ media api.Media }
type detailsErrMsg struct{ err error }

const (
	detailsCoverWidth  = 24
	detailsCoverHeight = 15
	detailsTwoColMin   = 110
	detailsLeftCol     = 30
)

type detailsTab int

const (
	tabOverview detailsTab = iota
	tabRelations
	tabRecommendations
)

type Details struct {
	client   *api.Client
	mediaID  int
	media    *api.Media
	err      error
	loading  bool
	spinner  spinner.Model
	viewport viewport.Model
	width    int
	height   int
	tab      detailsTab

	// Mouse hit-testing for the relations/recommendations card strip.
	stripTop   int
	stripMedia []api.Media
}

func NewDetails(client *api.Client, mediaID int) *Details {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = core.AccentStyle
	return &Details{
		client:  client,
		mediaID: mediaID,
		loading: true,
		spinner: sp,
	}
}

func (d *Details) Init() tea.Cmd {
	return tea.Batch(d.spinner.Tick, d.fetch())
}

func (d *Details) fetch() tea.Cmd {
	client := d.client
	id := d.mediaID
	return func() tea.Msg {
		media, err := client.GetMediaDetails(id)
		if err != nil {
			return detailsErrMsg{err: err}
		}
		return detailsLoadedMsg{media: media}
	}
}

// fetchCovers loads the large header cover plus the grid-size covers used by
// the relations/recommendations strips, skipping anything already cached.
func (d *Details) fetchCovers() tea.Cmd {
	if !ShowImages || d.media == nil {
		return nil
	}
	cmds := []tea.Cmd{}
	want := func(url string, w, h int) {
		if url == "" {
			return
		}
		if _, ok := cachedCover(url, w, h); ok {
			return
		}
		cmds = append(cmds, FetchCoverCmd(url, w, h))
	}
	want(d.media.CoverURL(), detailsCoverWidth, detailsCoverHeight)
	for _, r := range d.media.Relations {
		want(r.Media.CoverURL(), coverCellWidth, coverCellHeight)
	}
	for _, r := range d.media.Recommendations {
		want(r.CoverURL(), coverCellWidth, coverCellHeight)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (d *Details) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.SizedMsg:
		d.width = msg.Width
		d.height = msg.Height
		d.resizeViewport()
		return d, nil

	case detailsLoadedMsg:
		d.loading = false
		d.err = nil
		d.media = &msg.media
		d.rebuildContent()
		return d, d.fetchCovers()

	case detailsErrMsg:
		d.loading = false
		d.err = msg.err
		return d, nil

	case CoverReadyMsg:
		storeCover(msg.URL, msg.W, msg.H, msg.Rendered)
		if d.media != nil {
			d.viewport.SetContent(d.content()) // swap placeholders for images
		}
		return d, coverSettle()

	case coverSettleMsg:
		return d, nil

	case EntrySavedMsg:
		d.loading = true
		return d, tea.Batch(d.spinner.Tick, d.fetch())

	case spinner.TickMsg:
		if !d.loading {
			return d, nil
		}
		var cmd tea.Cmd
		d.spinner, cmd = d.spinner.Update(msg)
		return d, cmd

	case tea.KeyMsg:
		return d.handleKey(msg)

	case tea.MouseMsg:
		return d.handleMouse(msg)
	}
	return d, nil
}

func (d *Details) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if d.err != nil {
		if key.Matches(msg, core.Keys.Refresh) {
			d.loading = true
			d.err = nil
			return d, tea.Batch(d.spinner.Tick, d.fetch())
		}
		return d, nil
	}
	if d.loading {
		return d, nil
	}
	switch {
	case msg.String() == "tab":
		d.switchTab((d.tab + 1) % 3)
		return d, nil
	case msg.String() == "shift+tab":
		d.switchTab((d.tab + 2) % 3)
		return d, nil
	case msg.String() == "t":
		if url := d.trailerURL(); url != "" {
			return d, openURL(url)
		}
		return d, nil
	case msg.String() == "o":
		if d.media != nil && d.media.SiteURL != nil && *d.media.SiteURL != "" {
			return d, openURL(*d.media.SiteURL)
		}
		return d, nil
	case key.Matches(msg, core.Keys.Edit):
		if d.media != nil {
			m := *d.media
			return d, func() tea.Msg { return core.EditEntryMsg{Media: m} }
		}
		return d, nil
	case key.Matches(msg, core.Keys.Refresh):
		d.loading = true
		return d, tea.Batch(d.spinner.Tick, d.fetch())
	case key.Matches(msg, core.Keys.Back):
		return d, func() tea.Msg { return core.PopViewMsg{} }
	}
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

// handleMouse feeds the wheel to the viewport and opens cards clicked in the
// relations/recommendations strip.
func (d *Details) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if d.loading || d.media == nil {
		return d, nil
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && len(d.stripMedia) > 0 {
		line := msg.Y + d.viewport.YOffset
		if line >= d.stripTop+1 && line < d.stripTop+1+gridCardHeight {
			stride := gridCellWidth + gridColGap
			if msg.X >= 0 && msg.X%stride < gridCellWidth {
				if idx := msg.X / stride; idx < len(d.stripMedia) {
					id := d.stripMedia[idx].ID
					return d, func() tea.Msg { return core.OpenDetailsMsg{MediaID: id} }
				}
			}
		}
		return d, nil
	}
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

func (d *Details) switchTab(t detailsTab) {
	d.tab = t
	d.viewport.SetContent(d.content())
	d.viewport.GotoTop()
}

func (d *Details) trailerURL() string {
	if d.media == nil || d.media.Trailer == nil {
		return ""
	}
	switch strings.ToLower(d.media.Trailer.Site) {
	case "youtube":
		return "https://www.youtube.com/watch?v=" + d.media.Trailer.ID
	case "dailymotion":
		return "https://www.dailymotion.com/video/" + d.media.Trailer.ID
	}
	return ""
}

// openURL opens the URL in the system browser, suspending the TUI meanwhile.
func openURL(url string) tea.Cmd {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return tea.ExecProcess(cmd, nil)
}

func (d *Details) resizeViewport() {
	h := max(3, d.height-2)
	w := max(20, d.width)
	if d.viewport.Width == 0 {
		d.viewport = viewport.New(w, h)
	} else {
		d.viewport.Width = w
		d.viewport.Height = h
	}
	if d.media != nil {
		d.viewport.SetContent(d.content())
	}
}

func (d *Details) rebuildContent() {
	if d.viewport.Width == 0 && d.width > 0 {
		d.resizeViewport()
	}
	d.viewport.SetContent(d.content())
	d.viewport.GotoTop()
}

func (d *Details) View() string {
	if d.media == nil {
		if d.err != nil {
			return "\n " + core.ErrorStyle.Render("Failed to load details: "+d.err.Error()) +
				"\n\n " + core.HelpStyle.Render("r retry · esc back")
		}
		return "\n " + d.spinner.View() + " Loading details…"
	}

	keys := "j/k scroll · tab section · e edit · r refresh · esc back"
	if d.trailerURL() != "" {
		keys = strings.Replace(keys, "e edit", "e edit · t trailer", 1)
	}
	if d.media.SiteURL != nil && *d.media.SiteURL != "" {
		keys = strings.Replace(keys, "r refresh", "o open · r refresh", 1)
	}
	if pct := d.viewport.ScrollPercent(); pct < 0.99 {
		keys += fmt.Sprintf(" · %d%%", int(pct*100))
	}
	return d.viewport.View() + "\n" + core.HelpStyle.Render(keys)
}

var (
	htmlBreakRe = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe   = regexp.MustCompile(`<[^>]+>`)
)

func stripHTMLDesc(s string) string {
	s = htmlBreakRe.ReplaceAllString(s, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func humanizeLabel(s string) string {
	words := strings.Fields(strings.ReplaceAll(strings.ToLower(s), "_", " "))
	for i, w := range words {
		switch w {
		case "tv", "ova", "ona":
			words[i] = strings.ToUpper(w)
		default:
			if w != "" {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
	}
	return strings.Join(words, " ")
}

func scoreStyled(score *float64) string {
	if score == nil {
		return core.SubtleStyle.Render("N/A")
	}
	v := *score / 10
	color := lipgloss.Color("#FF6B6B")
	switch {
	case v >= 7.5:
		color = lipgloss.Color("#7BD88F")
	case v >= 5.0:
		color = lipgloss.Color("#FFD166")
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(fmt.Sprintf("%.1f", v))
}

func listStatusDisplay(status, mediaType string) string {
	isAnime := mediaType == "ANIME"
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

// bar renders a small solid-fill progress bar for the entry panel.
func bar(pct float64, width int) string {
	p := progress.New(progress.WithSolidFill(string(core.AccentColor)))
	p.Width = width
	p.ShowPercentage = false
	return p.ViewAs(pct)
}

func (d *Details) content() string {
	m := d.media
	w := d.viewport.Width
	if w <= 0 {
		w = 80
	}
	d.stripMedia = nil
	switch d.tab {
	case tabRelations:
		return d.stripContent("Relations", m, w)
	case tabRecommendations:
		return d.stripContent("Recommendations", m, w)
	default:
		return d.overviewContent(m, w)
	}
}

// stripContent renders the relations/recommendations tab: a header plus one
// horizontal row of media cards (with a label line above each), truncated to
// the viewport width.
func (d *Details) stripContent(title string, m *api.Media, w int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(core.AccentColor)
	var b strings.Builder
	b.WriteString(core.TitleStyle.Render(m.DisplayTitle()) + "\n")
	b.WriteString(headerStyle.Render(title) + "\n\n")

	items := make([]api.Media, 0)
	labels := make([]string, 0)
	if d.tab == tabRelations {
		for _, r := range m.Relations {
			items = append(items, r.Media)
			labels = append(labels, humanizeLabel(r.RelationType))
		}
	} else {
		for _, r := range m.Recommendations {
			items = append(items, r)
			labels = append(labels, "★ "+r.DisplayScore())
		}
	}
	if len(items) == 0 {
		b.WriteString(core.SubtleStyle.Render("Nothing here yet.") + "\n")
		return b.String()
	}

	d.stripTop = strings.Count(b.String(), "\n") // label line of the strip
	strip, shown := cardStrip(items, labels, w)
	d.stripMedia = shown
	b.WriteString(strip + "\n")
	return b.String()
}

func (d *Details) overviewContent(m *api.Media, w int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(core.AccentColor)

	format := m.Type
	if m.Format != nil && *m.Format != "" {
		format = *m.Format
	}

	var b strings.Builder
	banner := lipgloss.NewStyle().
		Background(core.AccentColor).
		Foreground(lipgloss.Color("#0B1622")).
		Bold(true).
		Width(w-2).
		Padding(0, 1)
	b.WriteString(banner.Render(fmt.Sprintf("%s · %s", m.Type, humanizeLabel(format))))
	b.WriteString("\n\n")

	// Title block.
	var title strings.Builder
	title.WriteString(lipgloss.NewStyle().Bold(true).Render(m.DisplayTitle()) + "\n")
	if m.TitleRomaji != "" && m.TitleRomaji != m.DisplayTitle() {
		title.WriteString(core.SubtleStyle.Render(m.TitleRomaji) + "\n")
	}
	if m.TitleNative != nil && *m.TitleNative != "" {
		title.WriteString(core.SubtleStyle.Render(*m.TitleNative) + "\n")
	}

	// Facts, one per line (left column) or inline (narrow layout).
	facts := []struct{ k, v string }{{"Format", humanizeLabel(format)}}
	if m.Status != nil {
		facts = append(facts, struct{ k, v string }{"Status", humanizeLabel(*m.Status)})
	}
	if m.Type == "ANIME" {
		facts = append(facts, struct{ k, v string }{"Episodes", m.EpisodeOrChapterCount()})
		if m.Duration != nil && *m.Duration > 0 {
			facts = append(facts, struct{ k, v string }{"Duration", fmt.Sprintf("%d min/ep", *m.Duration)})
		}
	} else {
		facts = append(facts, struct{ k, v string }{"Chapters", m.EpisodeOrChapterCount()})
		if m.Volumes != nil && *m.Volumes > 0 {
			facts = append(facts, struct{ k, v string }{"Volumes", fmt.Sprintf("%d", *m.Volumes)})
		}
	}
	if m.Season != nil && m.SeasonYear != nil {
		facts = append(facts, struct{ k, v string }{"Season", fmt.Sprintf("%s %d", humanizeLabel(*m.Season), *m.SeasonYear)})
	} else if m.Year != nil {
		facts = append(facts, struct{ k, v string }{"Year", fmt.Sprintf("%d", *m.Year)})
	}
	if len(m.Studios) > 0 {
		studios := m.Studios
		if len(studios) > 2 {
			studios = studios[:2]
		}
		facts = append(facts, struct{ k, v string }{"Studio", strings.Join(studios, ", ")})
	}
	if m.Source != nil && *m.Source != "" {
		facts = append(facts, struct{ k, v string }{"Source", humanizeLabel(*m.Source)})
	}
	if m.Country != nil && *m.Country != "" {
		facts = append(facts, struct{ k, v string }{"Country", *m.Country})
	}

	var factsBlock strings.Builder
	for _, f := range facts {
		factsBlock.WriteString(core.SubtleStyle.Render(f.k+": ") + f.v + "\n")
	}
	factsBlock.WriteString(core.SubtleStyle.Render("Score: ") + scoreStyled(m.Score) + "\n")
	factsBlock.WriteString(core.SubtleStyle.Render("Popularity: ") + fmt.Sprintf("%d", m.Popularity) + "\n")
	factsBlock.WriteString(core.SubtleStyle.Render("Favourites: ") + fmt.Sprintf("%d", m.Favourites))

	// Your Entry panel with progress/score bars.
	entry := ""
	if m.ListStatus != nil {
		lines := []string{core.SubtleStyle.Render("Status: ") + listStatusDisplay(*m.ListStatus, m.Type)}
		if m.ListProgress != nil {
			total := 0
			if m.Type == "ANIME" && m.Episodes != nil {
				total = *m.Episodes
			}
			if m.Type != "ANIME" && m.Chapters != nil {
				total = *m.Chapters
			}
			label := fmt.Sprintf("%d", *m.ListProgress)
			if total > 0 {
				label = fmt.Sprintf("%d/%d", *m.ListProgress, total)
			}
			lines = append(lines, core.SubtleStyle.Render("Progress: ")+label)
			if total > 0 {
				lines = append(lines, bar(float64(*m.ListProgress)/float64(total), 18))
			}
		}
		if m.ListScore != nil && *m.ListScore > 0 {
			lines = append(lines, core.SubtleStyle.Render("Score: ")+fmt.Sprintf("%.1f", *m.ListScore/10))
			lines = append(lines, bar(*m.ListScore/100, 18))
		}
		if m.ListNotes != nil && *m.ListNotes != "" {
			lines = append(lines, core.SubtleStyle.Render("Notes: ")+*m.ListNotes)
		}
		entry = headerStyle.Render("Your Entry") + "\n" + core.PanelStyle.Width(detailsLeftCol-6).Render(strings.Join(lines, "\n"))
	}

	// Cover block (hidden entirely when images are off).
	cover := ""
	if ShowImages {
		if rendered, ok := cachedCover(m.CoverURL(), detailsCoverWidth, detailsCoverHeight); ok && rendered != "" {
			cover = rendered
		} else {
			cover = CoverPlaceholder(detailsCoverWidth, detailsCoverHeight, m.DisplayTitle())
		}
	}

	// Genres / synopsis / tags / characters, sized for the right column.
	rightW := w
	if w >= detailsTwoColMin {
		rightW = w - detailsLeftCol - 2
	}

	var right strings.Builder
	if len(m.Genres) > 0 {
		chip := lipgloss.NewStyle().
			Background(core.AccentColor).
			Foreground(lipgloss.Color("#0B1622")).
			Padding(0, 1)
		chips := make([]string, len(m.Genres))
		for i, g := range m.Genres {
			chips[i] = chip.Render(g)
		}
		right.WriteString(headerStyle.Render("Genres") + "\n" + strings.Join(chips, " ") + "\n\n")
	}
	if m.Description != nil && *m.Description != "" {
		desc := stripHTMLDesc(*m.Description)
		if len(desc) > 2000 {
			desc = desc[:2000]
		}
		right.WriteString(headerStyle.Render("Synopsis") + "\n" +
			lipgloss.NewStyle().Width(rightW-1).Render(desc) + "\n\n")
	}
	if len(m.Tags) > 0 {
		tagChip := lipgloss.NewStyle().
			Background(lipgloss.Color("#1E2A38")).
			Foreground(lipgloss.Color("#6B7A8C")).
			Padding(0, 1)
		tags := m.Tags
		if len(tags) > 15 {
			tags = tags[:15]
		}
		chips := make([]string, len(tags))
		for i, t := range tags {
			chips[i] = tagChip.Render(t)
		}
		right.WriteString(headerStyle.Render("Tags") + "\n" + strings.Join(chips, " ") + "\n\n")
	}
	if len(m.Characters) > 0 {
		right.WriteString(headerStyle.Render("Characters") + "\n" + characterList(m.Characters, rightW))
	}

	if w < detailsTwoColMin {
		b.WriteString(title.String() + "\n")
		if cover != "" {
			b.WriteString(cover + "\n\n")
		}
		b.WriteString(factsBlock.String() + "\n")
		if entry != "" {
			b.WriteString(entry + "\n\n")
		}
		b.WriteString(right.String())
		return b.String()
	}

	left := title.String() + "\n" + factsBlock.String()
	if entry != "" {
		left += "\n" + entry
	}
	if cover != "" {
		left = cover + "\n\n" + left
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(detailsLeftCol).Render(left),
		"  ",
		lipgloss.NewStyle().Width(rightW).Render(right.String()),
	))
	b.WriteString("\n")
	return b.String()
}

// characterList renders up to 12 characters with role-colored labels, in two
// columns when there's room.
func characterList(characters []api.Character, width int) string {
	if len(characters) > 12 {
		characters = characters[:12]
	}
	rows := make([]string, len(characters))
	for i, c := range characters {
		name := c.NameFull
		if c.NameNative != nil && *c.NameNative != "" {
			name += " " + core.SubtleStyle.Render("("+*c.NameNative+")")
		}
		role := core.SubtleStyle.Render(" — " + humanizeLabel(c.Role))
		if c.Role == "MAIN" {
			role = core.AccentStyle.Render(" — Main")
		}
		rows[i] = "  " + name + role
	}
	if len(rows) < 4 || width < 64 {
		return strings.Join(rows, "\n")
	}
	colW := (width - 2) / 2
	half := (len(rows) + 1) / 2
	left := make([]string, half)
	right := make([]string, 0, half)
	for i, r := range rows {
		if i < half {
			left[i] = r
		} else {
			right = append(right, r)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colW).Render(strings.Join(left, "\n")),
		"  ",
		lipgloss.NewStyle().Width(colW).Render(strings.Join(right, "\n")),
	)
}
