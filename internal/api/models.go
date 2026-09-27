package api

import "fmt"

type Character struct {
	ID         int     `json:"id"`
	NameFull   string  `json:"nameFull"`
	NameNative *string `json:"nameNative,omitempty"`
	ImageURL   *string `json:"imageUrl,omitempty"`
	Role       string  `json:"role"`
}

// Trailer is a video trailer hosted on an external site (usually YouTube).
type Trailer struct {
	ID   string `json:"id"`
	Site string `json:"site"`
}

// Relation is a related-work edge (sequel, prequel, side story, …).
type Relation struct {
	RelationType string `json:"relationType"`
	Media        Media  `json:"media"`
}

type Media struct {
	ID                   int         `json:"id"`
	TitleRomaji          string      `json:"titleRomaji"`
	TitleEnglish         *string     `json:"titleEnglish,omitempty"`
	TitleNative          *string     `json:"titleNative,omitempty"`
	Type                 string      `json:"type"`
	Format               *string     `json:"format,omitempty"`
	Status               *string     `json:"status,omitempty"`
	Description          *string     `json:"description,omitempty"`
	Episodes             *int        `json:"episodes,omitempty"`
	Chapters             *int        `json:"chapters,omitempty"`
	Volumes              *int        `json:"volumes,omitempty"`
	Duration             *int        `json:"duration,omitempty"`
	Score                *float64    `json:"score,omitempty"`
	Genres               []string    `json:"genres"`
	Tags                 []string    `json:"tags"`
	CoverImageMedium     *string     `json:"coverImageMedium,omitempty"`
	CoverImageLarge      *string     `json:"coverImageLarge,omitempty"`
	CoverImageExtraLarge *string     `json:"coverImageExtraLarge,omitempty"`
	BannerImage          *string     `json:"bannerImage,omitempty"`
	Season               *string     `json:"season,omitempty"`
	SeasonYear           *int        `json:"seasonYear,omitempty"`
	Year                 *int        `json:"year,omitempty"`
	Popularity           int         `json:"popularity"`
	Favourites           int         `json:"favourites"`
	Studios              []string    `json:"studios"`
	Characters           []Character `json:"characters"`
	Trailer              *Trailer    `json:"trailer,omitempty"`
	Relations            []Relation  `json:"relations"`
	Recommendations      []Media     `json:"recommendations"`
	Source               *string     `json:"source,omitempty"`
	Country              *string     `json:"country,omitempty"`
	IsAdult              bool        `json:"isAdult"`
	SiteURL              *string     `json:"siteUrl,omitempty"`
	ListEntryID          *int        `json:"listEntryId,omitempty"`
	ListStatus           *string     `json:"listStatus,omitempty"`
	ListProgress         *int        `json:"listProgress,omitempty"`
	ListScore            *float64    `json:"listScore,omitempty"`
	ListNotes            *string     `json:"listNotes,omitempty"`
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
	ID        int     `json:"id"`
	Media     Media   `json:"media"`
	Status    string  `json:"status"`
	Progress  int     `json:"progress"`
	Score     float64 `json:"score"`
	Notes     *string `json:"notes,omitempty"`
	Repeat    int     `json:"repeat"`
	Private   bool    `json:"private"`
	UpdatedAt *int    `json:"updatedAt,omitempty"`
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
	Name    string      `json:"name"`
	Status  string      `json:"status"`
	Entries []MediaList `json:"entries"`
}

type GenreStats struct {
	Genre          string  `json:"genre"`
	Count          int     `json:"count"`
	MeanScore      float64 `json:"meanScore"`
	MinutesWatched int     `json:"minutesWatched"`
	ChaptersRead   int     `json:"chaptersRead"`
}

type TagStats struct {
	Tag       string  `json:"tag"`
	Count     int     `json:"count"`
	MeanScore float64 `json:"meanScore"`
}

type UserStats struct {
	AnimeCount           int          `json:"animeCount"`
	AnimeMeanScore       float64      `json:"animeMeanScore"`
	AnimeMinutesWatched  int          `json:"animeMinutesWatched"`
	AnimeEpisodesWatched int          `json:"animeEpisodesWatched"`
	MangaCount           int          `json:"mangaCount"`
	MangaMeanScore       float64      `json:"mangaMeanScore"`
	MangaChaptersRead    int          `json:"mangaChaptersRead"`
	MangaVolumesRead     int          `json:"mangaVolumesRead"`
	AnimeWatching        int          `json:"animeWatching"`
	AnimeCompleted       int          `json:"animeCompleted"`
	AnimePaused          int          `json:"animePaused"`
	AnimeDropped         int          `json:"animeDropped"`
	AnimePlanning        int          `json:"animePlanning"`
	MangaReading         int          `json:"mangaReading"`
	MangaCompleted       int          `json:"mangaCompleted"`
	MangaPaused          int          `json:"mangaPaused"`
	MangaDropped         int          `json:"mangaDropped"`
	MangaPlanning        int          `json:"mangaPlanning"`
	TopAnimeGenres       []GenreStats `json:"topAnimeGenres"`
	TopMangaGenres       []GenreStats `json:"topMangaGenres"`
	TopTags              []TagStats   `json:"topTags"`
}

func (s UserStats) AnimeDaysWatched() float64 {
	return float64(s.AnimeMinutesWatched) / 1440
}

type User struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	AvatarMedium *string    `json:"avatarMedium,omitempty"`
	AvatarLarge  *string    `json:"avatarLarge,omitempty"`
	BannerImage  *string    `json:"bannerImage,omitempty"`
	About        *string    `json:"about,omitempty"`
	SiteURL      *string    `json:"siteUrl,omitempty"`
	CreatedAt    *int       `json:"createdAt,omitempty"`
	UpdatedAt    *int       `json:"updatedAt,omitempty"`
	Stats        *UserStats `json:"stats,omitempty"`
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
