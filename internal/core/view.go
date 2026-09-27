package core

import tea "github.com/charmbracelet/bubbletea"

type View interface {
	tea.Model
}

type SizedMsg = tea.WindowSizeMsg
