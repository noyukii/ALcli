package app

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/config"
	"github.com/noyukii/ALcli/internal/core"
	"github.com/noyukii/ALcli/internal/views"
)

type viewerFetchedMsg struct {
	user api.User
}

type App struct {
	cfg    *config.Config
	client *api.Client
	viewer *api.User

	stack   []core.View
	route   core.Route
	help    help.Model
	width   int
	height  int
	lastErr string
}

// SetImageMode selects the cover render mode across views. Call before New.
func SetImageMode(m views.RenderMode) {
	views.ApplyMode(m)
}

func New(cfg *config.Config) *App {
	a := &App{cfg: cfg, help: help.New()}
	if cfg.IsAuthenticated() {
		a.client = api.NewClient(cfg.AccessToken)
		a.route = core.RouteHome
		a.push(views.NewHome(a.client))
	} else {
		a.push(views.NewAuth(cfg))
	}
	return a
}

// NewLogin opens the authentication screen even when a token is already saved.
func NewLogin(cfg *config.Config) *App {
	a := &App{cfg: cfg, help: help.New()}
	a.push(views.NewAuth(cfg))
	return a
}

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if a.client != nil {
		cmds = append(cmds, a.fetchViewer())
	}
	if top := a.top(); top != nil {
		cmds = append(cmds, top.Init())
	}
	return tea.Batch(cmds...)
}

func (a *App) fetchViewer() tea.Cmd {
	client := a.client
	return func() tea.Msg {
		user, err := client.GetViewer()
		if err != nil {
			return core.ErrMsg{Err: err}
		}
		return viewerFetchedMsg{user: user}
	}
}

func (a *App) top() core.View {
	if len(a.stack) == 0 {
		return nil
	}
	return a.stack[len(a.stack)-1]
}

// childSize is the window size given to child views, minus the rows reserved
// for the tab bar and help footer.
func (a *App) childSize() tea.WindowSizeMsg {
	h := a.height
	if h > 3 {
		h -= 3
	}
	return tea.WindowSizeMsg{Width: a.width, Height: h}
}

func (a *App) push(v core.View) tea.Cmd {
	a.stack = append(a.stack, v)
	var sizeCmd tea.Cmd
	if a.width > 0 {
		_, sizeCmd = v.Update(a.childSize())
	}
	return tea.Batch(v.Init(), sizeCmd)
}

func (a *App) pop() {
	if len(a.stack) > 0 {
		a.stack = a.stack[:len(a.stack)-1]
	}
}

func (a *App) replaceRoot(v core.View) tea.Cmd {
	a.stack = []core.View{v}
	var sizeCmd tea.Cmd
	if a.width > 0 {
		_, sizeCmd = v.Update(a.childSize())
	}
	return tea.Batch(v.Init(), sizeCmd)
}

// navRoot switches the root view to the given route, tracking it for the tab
// bar. Returns nil (leaving the current route untouched) if the view can't be
// built yet, e.g. viewer data not loaded.
func (a *App) navRoot(route core.Route) tea.Cmd {
	v := a.rootForRoute(route)
	if v == nil {
		return nil
	}
	a.route = route
	return a.replaceRoot(v)
}

func (a *App) rootForRoute(route core.Route) core.View {
	switch route {
	case core.RouteSearch:
		return views.NewSearch(a.client)
	case core.RouteMyList:
		if a.viewer == nil {
			a.lastErr = "User data not loaded yet."
			return nil
		}
		return views.NewMyList(a.client, a.viewer.ID)
	case core.RouteProfile:
		if a.viewer == nil {
			a.lastErr = "User data not loaded yet."
			return nil
		}
		return views.NewProfile(a.client, a.viewer.ID)
	default:
		return views.NewHome(a.client)
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width
		// Reserve rows for the tab bar and help footer so views size correctly.
		childMsg := msg
		if childMsg.Height > 3 {
			childMsg.Height -= 3
		}
		cmds := make([]tea.Cmd, 0, len(a.stack))
		for _, v := range a.stack {
			if _, cmd := v.Update(childMsg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			return a, tea.Quit
		}
		if len(a.stack) <= 1 && a.client != nil {
			switch {
			case key.Matches(msg, core.Keys.Home):
				return a, a.navRoot(core.RouteHome)
			case key.Matches(msg, core.Keys.Search):
				return a, a.navRoot(core.RouteSearch)
			case key.Matches(msg, core.Keys.MyList):
				return a, a.navRoot(core.RouteMyList)
			case key.Matches(msg, core.Keys.Profile):
				return a, a.navRoot(core.RouteProfile)
			}
		}
		switch {
		case key.Matches(msg, core.Keys.Help):
			return a, a.push(views.NewHelp())
		case key.Matches(msg, core.Keys.Back):
			if len(a.stack) > 1 {
				a.pop()
				return a, nil
			}
		case key.Matches(msg, core.Keys.Quit):
			if len(a.stack) <= 1 {
				return a, tea.Quit
			}
		}

	case core.NavigateMsg:
		return a, a.navRoot(msg.Route)

	case core.OpenDetailsMsg:
		if a.client != nil {
			return a, a.push(views.NewDetails(a.client, msg.MediaID))
		}
		return a, nil

	case core.EditEntryMsg:
		if a.client != nil {
			return a, a.push(views.NewEditEntry(a.client, msg.Media))
		}
		return a, nil

	case core.PopViewMsg:
		a.pop()
		return a, nil

	case tea.MouseMsg:
		if a.client != nil {
			if msg.Y <= 1 {
				// Line 0 is the tab bar (clickable when no view is stacked);
				// line 1 is the blank separator.
				if msg.Y == 0 && len(a.stack) <= 1 && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
					if r, ok := core.TabAt(msg.X); ok {
						return a, a.navRoot(r)
					}
				}
				return a, nil
			}
			msg.Y -= 2 // strip tab bar + separator so children get body coords
		}
		if top := a.top(); top != nil {
			_, cmd := top.Update(msg)
			return a, cmd
		}
		return a, nil

	case core.AuthSuccessMsg:
		a.cfg.AccessToken = msg.Token
		if err := a.cfg.Save(); err != nil {
			a.lastErr = err.Error()
			return a, nil
		}
		a.client = api.NewClient(msg.Token)
		a.route = core.RouteHome
		return a, tea.Batch(a.fetchViewer(), a.replaceRoot(views.NewHome(a.client)))

	case core.LoggedOutMsg:
		a.cfg.AccessToken = ""
		_ = a.cfg.Save()
		a.client = nil
		a.viewer = nil
		return a, a.replaceRoot(views.NewAuth(a.cfg))

	case viewerFetchedMsg:
		a.viewer = &msg.user
		return a, nil

	case core.ErrMsg:
		if msg.Err != nil {
			a.lastErr = msg.Err.Error()
		}
		return a, nil
	}

	if top := a.top(); top != nil {
		_, cmd := top.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) View() string {
	top := a.top()
	if top == nil {
		return ""
	}

	var footer string
	if a.lastErr != "" {
		footer = core.ErrorStyle.Render(a.lastErr)
	} else {
		footer = a.help.View(core.Keys)
	}

	// The tab bar is only meaningful once signed in.
	var body string
	if a.client == nil {
		body = lipgloss.JoinVertical(lipgloss.Left, top.View(), footer)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left, core.TabBar(a.route, len(a.stack) <= 1), "", top.View(), footer)
	}
	// Flush pending kitty image transmits ahead of the frame: the heavy base64
	// payload reaches the terminal exactly once, before the placeholder cells
	// in the body that reference it, and stays out of the per-frame diff.
	return views.DrainKittyTransmits() + body
}
