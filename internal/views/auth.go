package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/config"
	"github.com/noyukii/ALcli/internal/core"
)

type authExchangedMsg struct {
	token string
	err   error
}

type Auth struct {
	cfg     *config.Config
	input   textinput.Model
	spinner spinner.Model

	useCode bool // client secret present: exchange code for token
	authURL string
	loading bool
	errMsg  string

	width  int
	height int
}

func NewAuth(cfg *config.Config) *Auth {
	clientID := cfg.ClientIDValue()
	useCode := strings.TrimSpace(cfg.ClientSecret) != ""

	in := textinput.New()
	in.EchoMode = textinput.EchoPassword
	in.EchoCharacter = '•'
	in.Focus()
	if useCode {
		in.Placeholder = "Paste authorization code…"
		in.Prompt = "code ‹ "
	} else {
		in.Placeholder = "Paste access token…"
		in.Prompt = "token ‹ "
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = core.AccentStyle

	authURL := api.BuildImplicitAuthURL(clientID)
	if useCode {
		authURL = api.BuildAuthURL(clientID)
	}

	return &Auth{
		cfg:     cfg,
		input:   in,
		spinner: sp,
		useCode: useCode,
		authURL: authURL,
	}
}

func (a *Auth) Init() tea.Cmd {
	return textinput.Blink
}

func (a *Auth) submit() tea.Cmd {
	val := strings.TrimSpace(a.input.Value())
	if val == "" {
		a.errMsg = "Nothing entered."
		return nil
	}
	a.errMsg = ""

	if !a.useCode {
		// Implicit grant: pasted value is the token itself.
		token := val
		return func() tea.Msg { return core.AuthSuccessMsg{Token: token} }
	}

	a.loading = true
	clientID := a.cfg.ClientIDValue()
	secret := a.cfg.ClientSecret
	return tea.Batch(a.spinner.Tick, func() tea.Msg {
		token, err := api.ExchangeCodeForToken(val, clientID, secret)
		return authExchangedMsg{token: token, err: err}
	})
}

func (a *Auth) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case core.SizedMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.input.Width = max(20, msg.Width-16)
		return a, nil

	case spinner.TickMsg:
		if !a.loading {
			return a, nil
		}
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case authExchangedMsg:
		a.loading = false
		if msg.err != nil {
			a.errMsg = msg.err.Error()
			return a, nil
		}
		token := msg.token
		return a, func() tea.Msg { return core.AuthSuccessMsg{Token: token} }

	case tea.KeyMsg:
		if a.loading {
			return a, nil
		}
		if msg.Type == tea.KeyEnter {
			return a, a.submit()
		}
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *Auth) View() string {
	var b strings.Builder
	b.WriteString(core.TitleStyle.Render("Login to AniList"))
	b.WriteString("\n\n")

	b.WriteString(core.SubtleStyle.Render("1. Open this URL in your browser and authorize:"))
	b.WriteString("\n")
	b.WriteString(core.AccentStyle.Render(a.authURL))
	b.WriteString("\n\n")
	if a.useCode {
		b.WriteString(core.SubtleStyle.Render("2. Copy the authorization code shown, paste it below."))
	} else {
		b.WriteString(core.SubtleStyle.Render("2. Copy the access token shown, paste it below."))
	}
	b.WriteString("\n\n")
	b.WriteString(a.input.View())
	b.WriteString("\n\n")

	switch {
	case a.loading:
		b.WriteString(a.spinner.View() + " Exchanging code…")
	case a.errMsg != "":
		b.WriteString(core.ErrorStyle.Render("Error: " + a.errMsg))
	default:
		b.WriteString(core.HelpStyle.Render("enter submit · ctrl+c quit"))
	}
	return b.String()
}
