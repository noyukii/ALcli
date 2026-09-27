package core

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/api"
)

type Route int

const (
	RouteHome Route = iota
	RouteSearch
	RouteMyList
	RouteProfile
)

type NavigateMsg struct{ Route Route }

type OpenDetailsMsg struct{ MediaID int }

type EditEntryMsg struct{ Media api.Media }

type PopViewMsg struct{}

type AuthSuccessMsg struct{ Token string }

type LoggedOutMsg struct{}

type ErrMsg struct{ Err error }

func ErrCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrMsg{Err: err}
	}
}
