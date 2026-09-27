package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/app"
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
	noImages := flag.Bool("no-images", false, "Disable image rendering (alias for --images=off).")
	imagesMode := flag.String("images", "auto", "Cover render mode: auto | halfblock | kitty | off.")
	logout := flag.Bool("logout", false, "Remove stored authentication token and exit.")
	showConfig := flag.Bool("config", false, "Show the path to the configuration file and exit.")
	flag.Parse()

	if *showConfig {
		fmt.Println("Config file:", config.File())
		os.Exit(0)
	}

	if *logout {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error loading config:", err)
			os.Exit(1)
		}
		if !cfg.IsAuthenticated() {
			fmt.Println("You are not currently logged in.")
			os.Exit(0)
		}
		if err := config.Delete(); err != nil {
			fmt.Fprintln(os.Stderr, "Error removing config:", err)
			os.Exit(1)
		}
		fmt.Println("Logged out successfully. Your access token has been removed.")
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading config:", err)
		os.Exit(1)
	}

	app.SetImageMode(resolveImageMode(*imagesMode, *noImages))
	a := app.New(cfg)

	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error running program:", err)
		os.Exit(1)
	}
}
