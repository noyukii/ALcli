package core

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit        key.Binding
	Back        key.Binding
	Home        key.Binding
	Search      key.Binding
	MyList      key.Binding
	Profile     key.Binding
	Refresh     key.Binding
	Help        key.Binding
	Edit        key.Binding
	Up          key.Binding
	Down        key.Binding
	Select      key.Binding
	SearchSlash key.Binding
}

var Keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Home: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "home"),
	),
	Search: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "search"),
	),
	MyList: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "my list"),
	),
	Profile: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "profile"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	SearchSlash: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
}

// ShortHelp / FullHelp satisfy the bubbles/help.KeyMap interface, so a
// help.Model can render the footer straight from these bindings.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Back, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Home, k.Search, k.MyList, k.Profile},
		{k.Up, k.Down, k.Select, k.Edit},
		{k.Refresh, k.Help, k.Back, k.Quit},
	}
}
