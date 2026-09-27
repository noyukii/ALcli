package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/config"
)

type commandState struct {
	config *config.Config
	out    io.Writer
	errOut io.Writer
	format string
}

type TUIRequest struct {
	Images   string
	NoImages bool
	Login    bool
}

func (TUIRequest) Error() string { return "launch TUI" }

func Execute(args []string, out, errOut io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	state := &commandState{config: cfg, out: out, errOut: errOut, format: "human"}
	root := state.root()
	root.SetArgs(args)
	return root.Execute()
}

func (s *commandState) root() *cobra.Command {
	root := &cobra.Command{
		Use:           "al",
		Short:         "AniList in the terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			showConfig, _ := cmd.Flags().GetBool("config")
			logout, _ := cmd.Flags().GetBool("logout")
			if showConfig {
				fmt.Fprintln(s.out, "Config file:", config.File())
				return nil
			}
			if logout {
				if !s.config.IsAuthenticated() {
					fmt.Fprintln(s.out, "You are not currently logged in.")
					return nil
				}
				if err := config.Delete(); err != nil {
					return fmt.Errorf("remove config: %w", err)
				}
				fmt.Fprintln(s.out, "Logged out successfully.")
				return nil
			}
			images, _ := cmd.Flags().GetString("images")
			noImages, _ := cmd.Flags().GetBool("no-images")
			return TUIRequest{Images: images, NoImages: noImages}
		},
	}
	root.SetOut(s.out)
	root.SetErr(s.errOut)
	root.PersistentFlags().StringVar(&s.format, "output", "human", "Output format: human or json.")
	root.PersistentFlags().Bool("config", false, "Show the configuration file path and exit.")
	root.PersistentFlags().Bool("logout", false, "Remove stored authentication token and exit.")
	root.PersistentFlags().String("images", "auto", "Cover render mode for the TUI: auto | halfblock | kitty | off.")
	root.PersistentFlags().Bool("no-images", false, "Disable image rendering in the TUI.")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if s.format != "human" && s.format != "json" {
			return fmt.Errorf("unsupported output format %q (use human or json)", s.format)
		}
		return nil
	}
	root.AddCommand(s.mediaCommand(), s.profileCommand(), s.listCommand(), s.favoritesCommand(), s.authCommand())
	root.Flags().BoolP("help", "h", false, "Help for al")
	root.SetHelpCommand(&cobra.Command{Use: "help [command]", Short: "Help about any command", Args: cobra.ArbitraryArgs, Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = root.Help()
			return
		}
		if target, _, err := root.Find(args); err == nil {
			_ = target.Help()
		} else {
			fmt.Fprintln(s.errOut, err)
		}
	}})
	return root
}

func (s *commandState) client() (*api.Client, error) {
	if !s.config.IsAuthenticated() {
		return nil, errors.New("not logged in; run `al auth login` first")
	}
	return api.NewClient(s.config.AccessToken), nil
}

func (s *commandState) emit(v any) error {
	if s.format == "json" {
		enc := json.NewEncoder(s.out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return fmt.Errorf("write JSON output: %w", err)
		}
		return nil
	}
	return nil
}

func (s *commandState) mediaCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "media", Short: "Discover and inspect anime and manga"}
	var mediaType, genre, status, format, season string
	var year, page, perPage int
	var query string
	search := &cobra.Command{Use: "search [title]", Short: "Search anime and manga", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validatePaging(page, perPage); err != nil {
			return err
		}
		if len(args) == 1 {
			query = args[0]
		}
		c, err := s.client()
		if err != nil {
			return err
		}
		items, _, err := c.SearchMedia(query, strings.ToUpper(mediaType), genre, strings.ToUpper(status), strings.ToUpper(format), nil, strings.ToUpper(season), year, page, perPage, false)
		if err != nil {
			return err
		}
		if s.format == "json" {
			return s.emit(items)
		}
		printMedia(s.out, items)
		return nil
	}}
	search.Flags().StringVar(&query, "query", "", "Search title.")
	search.Flags().StringVar(&mediaType, "type", "", "Media type: anime or manga.")
	search.Flags().StringVar(&genre, "genre", "", "Filter by genre.")
	search.Flags().StringVar(&status, "status", "", "Filter by media status.")
	search.Flags().StringVar(&format, "format", "", "Filter by media format.")
	search.Flags().StringVar(&season, "season", "", "Filter anime by season.")
	search.Flags().IntVar(&year, "year", 0, "Filter by season year.")
	search.Flags().IntVar(&page, "page", 1, "Result page.")
	search.Flags().IntVar(&perPage, "per-page", 20, "Results per page (1–50).")
	cmd.AddCommand(search)

	for _, kind := range []string{"trending", "popular", "seasonal"} {
		kind := kind
		var typ, seas string
		var yr, pg, count int
		browse := &cobra.Command{Use: kind, Short: "Browse " + kind + " anime or manga", RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validatePaging(pg, count); err != nil {
				return err
			}
			c, err := s.client()
			if err != nil {
				return err
			}
			var items []api.Media
			switch kind {
			case "trending":
				items, _, err = c.GetTrending(strings.ToUpper(typ), pg, count)
			case "popular":
				items, err = c.GetPopular(strings.ToUpper(typ), pg, count)
			case "seasonal":
				items, err = c.GetSeasonal(strings.ToUpper(seas), yr, pg, count)
			}
			if err != nil {
				return err
			}
			if s.format == "json" {
				return s.emit(items)
			}
			printMedia(s.out, items)
			return nil
		}}
		browse.Flags().StringVar(&typ, "type", "ANIME", "Media type: anime or manga.")
		browse.Flags().StringVar(&seas, "season", "", "Season: winter, spring, summer, or fall.")
		browse.Flags().IntVar(&yr, "year", 0, "Season year.")
		browse.Flags().IntVar(&pg, "page", 1, "Result page.")
		browse.Flags().IntVar(&count, "per-page", 20, "Results per page.")
		if kind == "seasonal" {
			browse.MarkFlagRequired("season")
			browse.MarkFlagRequired("year")
		}
		cmd.AddCommand(browse)
	}

	details := &cobra.Command{Use: "get <id>", Short: "Show media details", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return errors.New("media ID must be a positive integer")
		}
		c, err := s.client()
		if err != nil {
			return err
		}
		item, err := c.GetMediaDetails(id)
		if err != nil {
			return err
		}
		if s.format == "json" {
			return s.emit(item)
		}
		fmt.Fprintf(s.out, "%s (#%d)\nType: %s\n", item.DisplayTitle(), item.ID, item.Type)
		if item.Description != nil {
			fmt.Fprintf(s.out, "\n%s\n", plain(*item.Description))
		}
		return nil
	}}
	cmd.AddCommand(details)
	return cmd
}

func (s *commandState) profileCommand() *cobra.Command {
	return &cobra.Command{Use: "profile", Short: "Show the authenticated user's profile and statistics", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := s.client()
		if err != nil {
			return err
		}
		user, err := c.GetViewer()
		if err != nil {
			return err
		}
		stats, err := c.GetUserStats(user.ID)
		if err != nil {
			return err
		}
		user.Stats = &stats
		if s.format == "json" {
			return s.emit(user)
		}
		fmt.Fprintf(s.out, "%s (ID %d)\n", user.Name, user.ID)
		fmt.Fprintf(s.out, "Anime: %d titles, %.1f days watched\n", stats.AnimeCount, stats.AnimeDaysWatched())
		fmt.Fprintf(s.out, "Manga: %d titles, %d chapters read\n", stats.MangaCount, stats.MangaChaptersRead)
		return nil
	}}
}

func (s *commandState) listCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "View and manage your anime and manga list"}
	var typ string
	view := &cobra.Command{Use: "show", Short: "Show your list", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := s.client()
		if err != nil {
			return err
		}
		user, err := c.GetViewer()
		if err != nil {
			return err
		}
		groups, err := c.GetMediaList(user.ID, strings.ToUpper(typ))
		if err != nil {
			return err
		}
		if s.format == "json" {
			return s.emit(groups)
		}
		for _, group := range groups {
			fmt.Fprintf(s.out, "%s\n", group.Name)
			for _, entry := range group.Entries {
				fmt.Fprintf(s.out, "  %-48s %3d  %s\n", entry.Media.DisplayTitle(), entry.Progress, entry.StatusDisplay())
			}
		}
		return nil
	}}
	view.Flags().StringVar(&typ, "type", "ANIME", "List type: anime or manga.")
	cmd.AddCommand(view)

	var status string
	var progress int
	var score float64
	var notes string
	update := &cobra.Command{Use: "set <media-id>", Short: "Add or update a list entry", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return errors.New("media ID must be a positive integer")
		}
		c, err := s.client()
		if err != nil {
			return err
		}
		entry, err := c.SaveListEntry(id, strings.ToUpper(status), progress, score, notes, nil, 0, false)
		if err != nil {
			return err
		}
		if s.format == "json" {
			return s.emit(entry)
		}
		fmt.Fprintf(s.out, "Saved %s as %s (%d progress).\n", entry.Media.DisplayTitle(), entry.StatusDisplay(), entry.Progress)
		return nil
	}}
	update.Flags().StringVar(&status, "status", "PLANNING", "Entry status: planning, current, completed, paused, dropped, repeating.")
	update.Flags().IntVar(&progress, "progress", 0, "Episodes watched or chapters read.")
	update.Flags().Float64Var(&score, "score", 0, "Score in AniList's 0–10 scale.")
	update.Flags().StringVar(&notes, "notes", "", "Entry notes.")
	cmd.AddCommand(update)

	remove := &cobra.Command{Use: "delete <entry-id>", Short: "Delete a list entry", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return errors.New("entry ID must be a positive integer")
		}
		yes, _ := cmd.Flags().GetBool("yes")
		if err := confirmDelete(os.Stdin, s.errOut, id, yes, isTerminal(os.Stdin)); err != nil {
			return err
		}
		c, err := s.client()
		if err != nil {
			return err
		}
		deleted, err := c.DeleteListEntry(id)
		if err != nil {
			return err
		}
		if s.format == "json" {
			return s.emit(map[string]bool{"deleted": deleted})
		}
		if deleted {
			fmt.Fprintln(s.out, "List entry deleted.")
		} else {
			fmt.Fprintln(s.out, "No entry was deleted.")
		}
		return nil
	}}
	remove.Flags().Bool("yes", false, "Skip the confirmation prompt.")
	cmd.AddCommand(remove)
	return cmd
}

func (s *commandState) favoritesCommand() *cobra.Command {
	return &cobra.Command{Use: "favorites", Short: "Show your favorite anime", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := s.client()
		if err != nil {
			return err
		}
		user, err := c.GetViewer()
		if err != nil {
			return err
		}
		items, err := c.GetUserFavourites(user.ID)
		if err != nil {
			return err
		}
		if s.format == "json" {
			return s.emit(items)
		}
		printMedia(s.out, items)
		return nil
	}}
}

func (s *commandState) authCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage AniList authentication"}
	cmd.AddCommand(&cobra.Command{Use: "status", Short: "Show authentication status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if s.format == "json" {
			return s.emit(map[string]bool{"authenticated": s.config.IsAuthenticated()})
		}
		if s.config.IsAuthenticated() {
			fmt.Fprintln(s.out, "Logged in.")
		} else {
			fmt.Fprintln(s.out, "Not logged in.")
		}
		return nil
	}})
	cmd.AddCommand(&cobra.Command{Use: "logout", Short: "Remove the saved access token", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !s.config.IsAuthenticated() {
			if s.format == "json" {
				return s.emit(map[string]bool{"loggedOut": false})
			}
			fmt.Fprintln(s.out, "You are not currently logged in.")
			return nil
		}
		if err := config.Delete(); err != nil {
			return fmt.Errorf("remove config: %w", err)
		}
		if s.format == "json" {
			return s.emit(map[string]bool{"loggedOut": true})
		}
		fmt.Fprintln(s.out, "Logged out successfully.")
		return nil
	}})
	cmd.AddCommand(&cobra.Command{Use: "login", Short: "Start the AniList login flow", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return TUIRequest{Images: "auto", Login: true}
	}})
	return cmd
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func confirmDelete(in io.Reader, out io.Writer, id int, yes, terminal bool) error {
	if yes {
		return nil
	}
	if !terminal {
		return errors.New("confirmation required; rerun with --yes from an interactive terminal or provide --yes")
	}
	fmt.Fprintf(out, "Delete list entry %d? [y/N] ", id)
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(answer), "y") && !strings.EqualFold(strings.TrimSpace(answer), "yes") {
		return errors.New("deletion cancelled")
	}
	return nil
}

func validatePaging(page, perPage int) error {
	if page < 1 {
		return errors.New("page must be at least 1")
	}
	if perPage < 1 || perPage > 50 {
		return errors.New("per-page must be between 1 and 50")
	}
	return nil
}

func printMedia(out io.Writer, items []api.Media) {
	for _, item := range items {
		fmt.Fprintf(out, "%-48s  #%d  %s  %s\n", item.DisplayTitle(), item.ID, item.Type, item.DisplayScore())
	}
}

func plain(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	return strings.TrimSpace(s)
}
