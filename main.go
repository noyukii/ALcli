package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/app"
	"github.com/noyukii/ALcli/internal/cli"
	"github.com/noyukii/ALcli/internal/config"
	"github.com/noyukii/ALcli/internal/views"
)

func resolveImageMode(pref string, noImages bool) views.RenderMode {
	if noImages {
		return views.ModeOff
	}
	switch strings.ToLower(pref) {
	case "off", "none":
		return views.ModeOff
	case "halfblock", "half", "blocks":
		return views.ModeHalfblock
	case "kitty", "graphics":
		return views.ModeKitty
	case "auto", "":
		if views.KittyCapable() {
			return views.ModeKitty
		}
		return views.ModeHalfblock
	default:
		return views.ModeHalfblock
	}
}

func main() {
	err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr)
	var tui cli.TUIRequest
	if err != nil && !errors.As(err, &tui) {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if err == nil {
		return
	}
	if err := runTUI(tui); err != nil {
		fmt.Fprintln(os.Stderr, "Error running program:", err)
		os.Exit(1)
	}
}

func runTUI(request cli.TUIRequest) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	app.SetImageMode(resolveImageMode(request.Images, request.NoImages))
	a := app.New(cfg)
	if request.Login {
		a = app.NewLogin(cfg)
	}
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	return nil
}
