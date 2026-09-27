package views

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

var profileHTMLTagRe = regexp.MustCompile(`<[^>]+>`)

const (
	profileAvatarSize = 12
	profileTwoColMin  = 110
	profileMaxRecent  = 12
)

type profileViewerMsg struct {
	user api.User
	err  error
}

type profileStatsMsg struct {
	stats api.UserStats
	err   error
}

type profileListsMsg struct {
	anime []api.MediaListGroup
	manga []api.MediaListGroup
	err   error
}

type profileFavouritesMsg struct {
	favourites []api.Media
	err        error
}

// profileStrip records where a card strip landed in the rendered content, for
// mouse hit-testing.
type profileStrip struct {
	top   int
	media []api.Media
}

type Profile struct {
	client *api.Client
	userID int
	width  int
	height int

	spinner spinner.Model
	vp      viewport.Model
	vpReady bool

	loading  bool
	pending  int
	user     *api.User
	stats    *api.UserStats
	recent   []api.MediaList
	animeLog []api.MediaList
	faves    []api.Media
	errMsg   string

	strips []profileStrip
}

func NewProfile(client *api.Client, userID int) *Profile {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = core.AccentStyle
	return &Profile{
		client:  client,
		userID:  userID,
		spinner: sp,
		loading: true,
	}
}

func (p *Profile) Init() tea.Cmd {
	p.pending = 4
	return tea.Batch(p.spinner.Tick, p.viewerCmd(), p.statsCmd(), p.listsCmd(), p.favouritesCmd())
}

func (p *Profile) viewerCmd() tea.Cmd {
	client := p.client
	return func() tea.Msg {
		user, err := client.GetViewer()
		return profileViewerMsg{user: user, err: err}
	}
}

func (p *Profile) statsCmd() tea.Cmd {
	client := p.client
	userID := p.userID
	return func() tea.Msg {
		stats, err := client.GetUserStats(userID)
		return profileStatsMsg{stats: stats, err: err}
	}
}

// listsCmd fetches the full anime and manga collections; the profile derives
// the recently-updated strip and the score histogram from them client-side.
func (p *Profile) listsCmd() tea.Cmd {
	client := p.client
	userID := p.userID
	return func() tea.Msg {
		anime, err := client.GetMediaList(userID, "ANIME")
		if err != nil {
			return profileListsMsg{err: err}
		}
		manga, err := client.GetMediaList(userID, "MANGA")
		if err != nil {
			return profileListsMsg{err: err}
		}
		return profileListsMsg{anime: anime, manga: manga}
	}
}

func (p *Profile) favouritesCmd() tea.Cmd {
	client := p.client
	userID := p.userID
	return func() tea.Msg {
		faves, err := client.GetUserFavourites(userID)
		return profileFavouritesMsg{favourites: faves, err: err}
	}
}

func (p *Profile) refetch() tea.Cmd {
	p.loading = true
	p.errMsg = ""
	p.user = nil
	p.stats = nil
	p.recent = nil
	p.animeLog = nil
	p.faves = nil
	return p.Init()
}

// finish settles once all four fetches answered, then builds content.
func (p *Profile) finish() {
	if p.pending > 0 {
		return
	}
	p.loading = false
	if p.errMsg != "" || p.user == nil || p.stats == nil {
		return
	}
	if p.vpReady {
		p.vp.SetContent(p.content())
	}
}

// settled reports whether every fetch answered; used to fire fetchCovers once,
// regardless of arrival order.
func (p *Profile) settled() bool {
	return p.pending == 0
}

// fetchCovers warms the avatar and every strip card, skipping cached entries.
func (p *Profile) fetchCovers() tea.Cmd {
	if !ShowImages {
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
	if p.user != nil {
		want(p.user.AvatarURL(), profileAvatarSize, profileAvatarSize)
	}
	for _, e := range p.recent {
		want(e.Media.CoverURL(), coverCellWidth, coverCellHeight)
	}
	for _, m := range p.faves {
		want(m.CoverURL(), coverCellWidth, coverCellHeight)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (p *Profile) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		h := msg.Height - 1
		if h < 3 {
			h = 3
		}
		if !p.vpReady {
			p.vp = viewport.New(msg.Width, h)
			p.vpReady = true
		} else {
			p.vp.Width = msg.Width
			p.vp.Height = h
		}
		if !p.loading && p.errMsg == "" {
			p.vp.SetContent(p.content())
		}
		return p, nil
	case spinner.TickMsg:
		if p.loading {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return p, cmd
		}
		return p, nil
	case profileViewerMsg:
		if msg.err != nil {
			p.errMsg = msg.err.Error()
		} else {
			user := msg.user
			p.user = &user
		}
		p.pending--
		p.finish()
		if p.settled() && p.errMsg == "" {
			return p, p.fetchCovers()
		}
		return p, nil
	case profileStatsMsg:
		if msg.err != nil {
			p.errMsg = msg.err.Error()
		} else {
			stats := msg.stats
			p.stats = &stats
		}
		p.pending--
		p.finish()
		if p.settled() && p.errMsg == "" {
			return p, p.fetchCovers()
		}
		return p, nil
	case profileListsMsg:
		if msg.err != nil {
			p.errMsg = msg.err.Error()
		} else {
			p.recent = recentEntries(msg.anime, msg.manga, profileMaxRecent)
			p.animeLog = flatEntries(msg.anime)
		}
		p.pending--
		p.finish()
		if p.settled() && p.errMsg == "" {
			return p, p.fetchCovers()
		}
		return p, nil
	case profileFavouritesMsg:
		if msg.err != nil {
			p.errMsg = msg.err.Error()
		} else {
			p.faves = msg.favourites
		}
		p.pending--
		p.finish()
		if p.settled() && p.errMsg == "" {
			return p, p.fetchCovers()
		}
		return p, nil
	case CoverReadyMsg:
		storeCover(msg.URL, msg.W, msg.H, msg.Rendered)
		if !p.loading && p.errMsg == "" {
			p.vp.SetContent(p.content())
		}
		return p, coverSettle()
	case coverSettleMsg:
		return p, nil
	case tea.KeyMsg:
		return p.handleKey(msg)
	case tea.MouseMsg:
		return p.handleMouse(msg)
	}
	return p, nil
}

func (p *Profile) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if p.loading {
		return p, nil
	}
	if key.Matches(msg, core.Keys.Refresh) {
		return p, p.refetch()
	}
	if p.errMsg != "" {
		return p, nil
	}
	switch {
	case key.Matches(msg, core.Keys.Up):
		p.vp.LineUp(1)
		return p, nil
	case key.Matches(msg, core.Keys.Down):
		p.vp.LineDown(1)
		return p, nil
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

// handleMouse opens cards clicked in the recent/favourites strips; the wheel
// goes to the viewport.
func (p *Profile) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if p.loading || p.errMsg != "" {
		return p, nil
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		line := msg.Y + p.vp.YOffset
		for _, st := range p.strips {
			if line < st.top+1 || line >= st.top+1+gridCardHeight {
				continue
			}
			stride := gridCellWidth + gridColGap
			if msg.X >= 0 && msg.X%stride < gridCellWidth {
				if idx := msg.X / stride; idx < len(st.media) {
					id := st.media[idx].ID
					return p, func() tea.Msg { return core.OpenDetailsMsg{MediaID: id} }
				}
			}
			return p, nil
		}
		return p, nil
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

func (p *Profile) View() string {
	if p.loading {
		return "\n" + p.spinner.View() + " Loading profile…"
	}
	if p.errMsg != "" {
		return "\n" + core.ErrorStyle.Render("Failed to load profile: "+p.errMsg) + "\n\n" + core.HelpStyle.Render("r retry")
	}
	if !p.vpReady {
		return ""
	}
	return p.vp.View()
}

// recentEntries flattens both collections, sorts by update time (newest
// first), dedupes by media id, and caps the result.
func recentEntries(anime, manga []api.MediaListGroup, limit int) []api.MediaList {
	all := append(flatEntries(anime), flatEntries(manga)...)
	sort.Slice(all, func(i, j int) bool {
		var ti, tj int64
		if all[i].UpdatedAt != nil {
			ti = int64(*all[i].UpdatedAt)
		}
		if all[j].UpdatedAt != nil {
			tj = int64(*all[j].UpdatedAt)
		}
		return ti > tj
	})
	seen := map[int]bool{}
	out := make([]api.MediaList, 0, limit)
	for _, e := range all {
		if e.Media.ID == 0 || seen[e.Media.ID] {
			continue
		}
		seen[e.Media.ID] = true
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func flatEntries(groups []api.MediaListGroup) []api.MediaList {
	out := []api.MediaList{}
	for _, g := range groups {
		out = append(out, g.Entries...)
	}
	return out
}

// recentLabel is the strip caption: list status plus progress when known.
func recentLabel(e api.MediaList) string {
	label := e.StatusDisplay()
	total := 0
	if e.Media.Type == "ANIME" && e.Media.Episodes != nil {
		total = *e.Media.Episodes
	}
	if e.Media.Type != "ANIME" && e.Media.Chapters != nil {
		total = *e.Media.Chapters
	}
	switch {
	case e.Progress > 0 && total > 0:
		label += fmt.Sprintf(" · %d/%d", e.Progress, total)
	case e.Progress > 0:
		label += fmt.Sprintf(" · %d", e.Progress)
	}
	return label
}

func (p *Profile) content() string {
	user, stats := p.user, p.stats
	p.strips = nil
	var b strings.Builder

	// Header: avatar (when images are on) next to name/url/about.
	var head strings.Builder
	head.WriteString(core.TitleStyle.Render(user.Name) + "\n")
	if user.SiteURL != nil && *user.SiteURL != "" {
		head.WriteString(core.SubtleStyle.Render(*user.SiteURL) + "\n")
	}
	if user.About != nil && *user.About != "" {
		about := strings.TrimSpace(profileHTMLTagRe.ReplaceAllString(*user.About, ""))
		if about != "" {
			head.WriteString(core.SubtleStyle.Render(runeTruncate(about, 200)) + "\n")
		}
	}
	avatar := ""
	if ShowImages {
		if rendered, ok := cachedCover(user.AvatarURL(), profileAvatarSize, profileAvatarSize); ok && rendered != "" {
			avatar = rendered
		} else {
			avatar = CoverPlaceholder(profileAvatarSize, profileAvatarSize, user.Name)
		}
	}
	if avatar != "" {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, avatar, "  ", head.String()) + "\n\n")
	} else {
		b.WriteString(head.String() + "\n")
	}

	// Fresh account: skip the walls of empty bars.
	if stats.AnimeCount == 0 && stats.MangaCount == 0 {
		b.WriteString(core.PanelStyle.Render("No anime or manga tracked yet.") + "\n\n")
		b.WriteString(core.HelpStyle.Render("1 browse trending · 2 search · add something and it shows up here") + "\n")
		return b.String()
	}

	// Recently updated strip.
	if len(p.recent) > 0 {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(core.AccentColor).Render("Recently Updated") + "\n\n")
		p.strips = append(p.strips, profileStrip{top: strings.Count(b.String(), "\n")})
		items := make([]api.Media, len(p.recent))
		labels := make([]string, len(p.recent))
		for i, e := range p.recent {
			items[i] = e.Media
			labels[i] = recentLabel(e)
		}
		strip, shown := cardStrip(items, labels, p.width)
		p.strips[len(p.strips)-1].media = shown
		b.WriteString(strip + "\n\n")
	}

	// Favourites strip.
	if len(p.faves) > 0 {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(core.AccentColor).Render("Favourite Anime") + "\n\n")
		p.strips = append(p.strips, profileStrip{top: strings.Count(b.String(), "\n")})
		strip, shown := cardStrip(p.faves, nil, p.width)
		p.strips[len(p.strips)-1].media = shown
		b.WriteString(strip + "\n\n")
	}

	// Stats: two columns on wide terminals, stacked otherwise.
	animeSec := strings.Join(append(animeStatsLines(stats), scoreDistLines(p.animeLog)...), "\n")
	mangaSec := strings.Join(mangaStatsLines(stats), "\n")
	if p.width >= profileTwoColMin {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(58).Render(animeSec),
			"    ",
			lipgloss.NewStyle().Width(p.width-62).Render(mangaSec),
		) + "\n")
	} else {
		b.WriteString(animeSec + "\n\n" + statsSeparator() + "\n\n" + mangaSec + "\n")
	}

	if tags := topTagsLines(stats.TopTags); len(tags) > 0 {
		b.WriteString("\n" + strings.Join(tags, "\n") + "\n")
	}
	return b.String()
}
