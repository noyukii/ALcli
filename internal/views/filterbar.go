package views

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/noyukii/ALcli/internal/core"
)

type filterOption struct {
	label string
	value string
}

type filterField struct {
	label   string
	options []filterOption
	index   int
}

func (f *filterField) value() string {
	return f.options[f.index].value
}

const (
	fieldType = iota
	fieldGenre
	fieldStatus
	fieldFormat
	fieldSeason
	fieldYear
	fieldSort
	fieldCount
)

var animeGenres = []string{
	"Action", "Adventure", "Comedy", "Drama", "Ecchi", "Fantasy",
	"Horror", "Mahou Shoujo", "Mecha", "Music", "Mystery", "Psychological",
	"Romance", "Sci-Fi", "Slice of Life", "Sports", "Supernatural", "Thriller",
}

func genreOptionsFor(mediaType string) []filterOption {
	names := make([]string, len(animeGenres), len(animeGenres)+1)
	copy(names, animeGenres)
	if mediaType == "MANGA" {
		names = append(names, "Hentai")
	}
	opts := make([]filterOption, 0, len(names)+1)
	opts = append(opts, filterOption{"Any", ""})
	for _, g := range names {
		opts = append(opts, filterOption{g, g})
	}
	return opts
}

func formatOptionsFor(mediaType string) []filterOption {
	if mediaType == "MANGA" {
		return []filterOption{
			{"Any", ""},
			{"Manga", "MANGA"},
			{"Novel", "NOVEL"},
			{"One Shot", "ONE_SHOT"},
		}
	}
	return []filterOption{
		{"Any", ""},
		{"TV", "TV"},
		{"TV Short", "TV_SHORT"},
		{"Movie", "MOVIE"},
		{"Special", "SPECIAL"},
		{"OVA", "OVA"},
		{"ONA", "ONA"},
		{"Music", "MUSIC"},
	}
}

var statusOptions = []filterOption{
	{"Any", ""},
	{"Finished", "FINISHED"},
	{"Releasing", "RELEASING"},
	{"Not Yet Released", "NOT_YET_RELEASED"},
	{"Cancelled", "CANCELLED"},
	{"Hiatus", "HIATUS"},
}

var seasonOptions = []filterOption{
	{"Any", ""},
	{"Winter", "WINTER"},
	{"Spring", "SPRING"},
	{"Summer", "SUMMER"},
	{"Fall", "FALL"},
}

var sortOptions = []filterOption{
	{"Popularity", "POPULARITY_DESC"},
	{"Score", "SCORE_DESC"},
	{"Trending", "TRENDING_DESC"},
	{"Newest", "START_DATE_DESC"},
	{"Oldest", "START_DATE"},
	{"Title A-Z", "TITLE_ROMAJI"},
	{"Favourites", "FAVOURITES_DESC"},
}

func yearOptions() []filterOption {
	current := time.Now().Year()
	opts := make([]filterOption, 0, current+1-1989)
	opts = append(opts, filterOption{"Any", ""})
	for y := current + 1; y >= 1990; y-- {
		s := strconv.Itoa(y)
		opts = append(opts, filterOption{s, s})
	}
	return opts
}

type FilterBar struct {
	fields []filterField
	focus  int
}

func NewFilterBar() *FilterBar {
	return &FilterBar{
		fields: []filterField{
			{label: "Type", options: []filterOption{{"Anime", "ANIME"}, {"Manga", "MANGA"}}},
			{label: "Genre", options: genreOptionsFor("ANIME")},
			{label: "Status", options: statusOptions},
			{label: "Format", options: formatOptionsFor("ANIME")},
			{label: "Season", options: seasonOptions},
			{label: "Year", options: yearOptions()},
			{label: "Sort", options: sortOptions},
		},
	}
}

func (f *FilterBar) Update(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "tab", "right":
		f.focus = (f.focus + 1) % len(f.fields)
	case "shift+tab", "left":
		f.focus = (f.focus - 1 + len(f.fields)) % len(f.fields)
	case "up", "k":
		f.cycle(-1)
		return true
	case "down", "j", "enter":
		f.cycle(1)
		return true
	}
	return false
}

func (f *FilterBar) cycle(dir int) {
	field := &f.fields[f.focus]
	field.index = (field.index + dir + len(field.options)) % len(field.options)
	if f.focus == fieldType {
		mediaType := field.value()
		f.resetField(fieldGenre, genreOptionsFor(mediaType))
		f.resetField(fieldFormat, formatOptionsFor(mediaType))
	}
}

func (f *FilterBar) resetField(i int, opts []filterOption) {
	current := f.fields[i].value()
	f.fields[i].options = opts
	f.fields[i].index = 0
	for j, o := range opts {
		if o.value == current {
			f.fields[i].index = j
			break
		}
	}
}

func (f *FilterBar) Values() (mediaType, genre, status, format, season string, year int, sort []string) {
	mediaType = f.fields[fieldType].value()
	genre = f.fields[fieldGenre].value()
	status = f.fields[fieldStatus].value()
	format = f.fields[fieldFormat].value()
	season = f.fields[fieldSeason].value()
	if v := f.fields[fieldYear].value(); v != "" {
		year, _ = strconv.Atoi(v)
	}
	if s := f.fields[fieldSort].value(); s != "" {
		sort = []string{s}
	}
	return
}

func (f *FilterBar) View(active bool) string {
	parts := make([]string, len(f.fields))
	for i, field := range f.fields {
		label := field.label + ": "
		value := field.options[field.index].label
		if active && i == f.focus {
			parts[i] = core.SelectedStyle.Render(label + value)
		} else {
			parts[i] = core.SubtleStyle.Render(label) + value
		}
	}
	return strings.Join(parts, "  ")
}
