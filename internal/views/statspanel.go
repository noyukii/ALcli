package views

import (
	"fmt"
	"math"
	"strings"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/core"
)

func StatBar(label string, count, max, width int) string {
	if width < 1 {
		width = 1
	}
	filled := 0
	if max > 0 {
		filled = int(math.Round(float64(count) / float64(max) * float64(width)))
	}
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := core.AccentStyle.Render(strings.Repeat("█", filled)) +
		core.SubtleStyle.Render(strings.Repeat("░", width-filled))
	return fmt.Sprintf("%s %s %d", runePad(runeTruncate(label, 16), 16), bar, count)
}

func runeTruncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func runePad(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(r))
}

func formatWatchTime(minutes int) string {
	days := minutes / 1440
	hours := (minutes % 1440) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes%60)
}

func animeStatsLines(s *api.UserStats) []string {
	lines := []string{core.TitleStyle.Render("Anime Statistics")}
	lines = append(lines, fmt.Sprintf("Total: %d   Mean Score: %.1f   Time Watched: %s   Episodes: %d",
		s.AnimeCount, s.AnimeMeanScore, formatWatchTime(s.AnimeMinutesWatched), s.AnimeEpisodesWatched))
	lines = append(lines, "", core.SubtleStyle.Render("Status Breakdown:"))
	for _, row := range []struct {
		label string
		count int
	}{
		{"Watching", s.AnimeWatching},
		{"Completed", s.AnimeCompleted},
		{"On Hold", s.AnimePaused},
		{"Dropped", s.AnimeDropped},
		{"Planning", s.AnimePlanning},
	} {
		lines = append(lines, "  "+StatBar(row.label, row.count, s.AnimeCount, 25))
	}
	lines = append(lines, genreStatLines("Top Genres (Anime):", s.TopAnimeGenres)...)
	return lines
}

func mangaStatsLines(s *api.UserStats) []string {
	lines := []string{core.TitleStyle.Render("Manga Statistics")}
	lines = append(lines, fmt.Sprintf("Total: %d   Mean Score: %.1f   Chapters: %d   Volumes: %d",
		s.MangaCount, s.MangaMeanScore, s.MangaChaptersRead, s.MangaVolumesRead))
	lines = append(lines, "", core.SubtleStyle.Render("Status Breakdown:"))
	for _, row := range []struct {
		label string
		count int
	}{
		{"Reading", s.MangaReading},
		{"Completed", s.MangaCompleted},
		{"On Hold", s.MangaPaused},
		{"Dropped", s.MangaDropped},
		{"Planning", s.MangaPlanning},
	} {
		lines = append(lines, "  "+StatBar(row.label, row.count, s.MangaCount, 25))
	}
	lines = append(lines, genreStatLines("Top Genres (Manga):", s.TopMangaGenres)...)
	return lines
}

func genreStatLines(title string, genres []api.GenreStats) []string {
	if len(genres) == 0 {
		return nil
	}
	max := 0
	for _, g := range genres {
		if g.Count > max {
			max = g.Count
		}
	}
	lines := []string{"", core.SubtleStyle.Render(title)}
	for i, g := range genres {
		if i >= 8 {
			break
		}
		lines = append(lines, "  "+StatBar(g.Genre, g.Count, max, 20)+fmt.Sprintf(" (avg %.0f)", g.MeanScore))
	}
	return lines
}

func topTagsLines(tags []api.TagStats) []string {
	if len(tags) == 0 {
		return nil
	}
	max := 0
	for _, t := range tags {
		if t.Count > max {
			max = t.Count
		}
	}
	lines := []string{core.SubtleStyle.Render("Top Tags:")}
	for i, t := range tags {
		if i >= 10 {
			break
		}
		lines = append(lines, "  "+StatBar(t.Tag, t.Count, max, 20)+fmt.Sprintf(" (avg %.0f)", t.MeanScore))
	}
	return lines
}

func statsSeparator() string {
	return core.SubtleStyle.Render(strings.Repeat("─", 60))
}

// scoreDistLines renders a 10-bucket histogram of list scores (0–100 scale).
// Unscored entries are skipped; returns nil when nothing is scored.
func scoreDistLines(entries []api.MediaList) []string {
	var buckets [10]int
	count := 0
	for _, e := range entries {
		if e.Score <= 0 {
			continue
		}
		b := (int(e.Score) - 1) / 10
		if b < 0 {
			b = 0
		}
		if b > 9 {
			b = 9
		}
		buckets[b]++
		count++
	}
	if count == 0 {
		return nil
	}
	peak := 0
	for _, c := range buckets {
		if c > peak {
			peak = c
		}
	}
	lines := []string{"", core.SubtleStyle.Render("Score Distribution:")}
	for i, c := range buckets {
		lines = append(lines, "  "+StatBar(fmt.Sprintf("%d-%d", i*10+1, (i+1)*10), c, peak, 20))
	}
	return lines
}
