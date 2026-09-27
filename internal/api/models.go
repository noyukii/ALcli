package api

import "fmt"

type Character struct {
	ID         int
	NameFull   string
	NameNative *string
	ImageURL   *string
	Role       string
}

// Trailer is a video trailer hosted on an external site (usually YouTube).
type Trailer struct {
	ID   string
	Site string
}

// Relation is a related-work edge (sequel, prequel, side story, …).
type Relation struct {
	RelationType string
	Media        Media
}

type Media struct {
	ID                   int
	TitleRomaji          string
	TitleEnglish         *string
	TitleNative          *string
	Type                 string
	Format               *string
	Status               *string
	Description          *string
	Episodes             *int
	Chapters             *int
	Volumes              *int
	Duration             *int
	Score                *float64
	Genres               []string
	Tags                 []string
	CoverImageMedium     *string
	CoverImageLarge      *string
	CoverImageExtraLarge *string
	BannerImage          *string
	Season               *string
	SeasonYear           *int
	Year                 *int
	Popularity           int
	Favourites           int
	Studios              []string
	Characters           []Character
	Trailer              *Trailer
	Relations            []Relation
	Recommendations      []Media
	Source               *string
	Country              *string
	IsAdult              bool
	SiteURL              *string
	ListEntryID          *int
	ListStatus           *string
	ListProgress         *int
	ListScore            *float64
	ListNotes            *string
}

func (m Media) DisplayTitle() string {
	if m.TitleEnglish != nil && *m.TitleEnglish != "" {
		return *m.TitleEnglish
	}
	return m.TitleRomaji
}

func (m Media) DisplayScore() string {
	if m.Score == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.1f", *m.Score/10)
}

func (m Media) CoverURL() string {
	for _, u := range []*string{m.CoverImageExtraLarge, m.CoverImageLarge, m.CoverImageMedium} {
		if u != nil && *u != "" {
			return *u
		}
	}
	return ""
}

func (m Media) EpisodeOrChapterCount() string {
	if m.Type == "ANIME" {
		if m.Episodes != nil && *m.Episodes > 0 {
			return fmt.Sprintf("%d", *m.Episodes)
		}
		return "?"
	}
	if m.Chapters != nil && *m.Chapters > 0 {
		return fmt.Sprintf("%d", *m.Chapters)
	}
	return "?"
}

func (m Media) ProgressLabel() string {
	if m.Type == "ANIME" {
		return "Episodes"
	}
	return "Chapters"
}

type MediaList struct {
	ID        int
	Media     Media
	Status    string
	Progress  int
	Score     float64
	Notes     *string
	Repeat    int
	Private   bool
	UpdatedAt *int
}

func (ml MediaList) DisplayScore() string {
	if ml.Score == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", ml.Score/10)
}

func (ml MediaList) StatusDisplay() string {
	isAnime := ml.Media.Type == "ANIME"
	switch ml.Status {
	case "CURRENT":
		if isAnime {
			return "Watching"
		}
		return "Reading"
	case "COMPLETED":
		return "Completed"
	case "PAUSED":
		return "On Hold"
	case "DROPPED":
		return "Dropped"
	case "PLANNING":
		if isAnime {
			return "Plan to Watch"
		}
		return "Plan to Read"
	case "REPEATING":
		if isAnime {
			return "Rewatching"
		}
		return "Rereading"
	}
	return ml.Status
}

type MediaListGroup struct {
	Name    string
	Status  string
	Entries []MediaList
}

type GenreStats struct {
	Genre          string
	Count          int
	MeanScore      float64
	MinutesWatched int
	ChaptersRead   int
}

type TagStats struct {
	Tag       string
	Count     int
	MeanScore float64
}

type UserStats struct {
	AnimeCount           int
	AnimeMeanScore       float64
	AnimeMinutesWatched  int
	AnimeEpisodesWatched int
	MangaCount           int
	MangaMeanScore       float64
	MangaChaptersRead    int
	MangaVolumesRead     int
	AnimeWatching        int
	AnimeCompleted       int
	AnimePaused          int
	AnimeDropped         int
	AnimePlanning        int
	MangaReading         int
	MangaCompleted       int
	MangaPaused          int
	MangaDropped         int
	MangaPlanning        int
	TopAnimeGenres       []GenreStats
	TopMangaGenres       []GenreStats
	TopTags              []TagStats
}

func (s UserStats) AnimeDaysWatched() float64 {
	return float64(s.AnimeMinutesWatched) / 1440
}

type User struct {
	ID           int
	Name         string
	AvatarMedium *string
	AvatarLarge  *string
	BannerImage  *string
	About        *string
	SiteURL      *string
	CreatedAt    *int
	UpdatedAt    *int
	Stats        *UserStats
}

func (u User) AvatarURL() string {
	if u.AvatarLarge != nil && *u.AvatarLarge != "" {
		return *u.AvatarLarge
	}
	if u.AvatarMedium != nil {
		return *u.AvatarMedium
	}
	return ""
}
