package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const GraphQLURL = "https://graphql.anilist.co"

type AnilistError struct {
	Message    string
	StatusCode int
}

func (e *AnilistError) Error() string {
	return e.Message
}

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) request(query string, variables map[string]any) (map[string]any, error) {
	payload := map[string]any{"query": query}
	if len(variables) > 0 {
		filtered := map[string]any{}
		for k, v := range variables {
			if v != nil {
				filtered[k] = v
			}
		}
		payload["variables"] = filtered
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, GraphQLURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &AnilistError{Message: fmt.Sprintf("Network error: %v", err)}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &AnilistError{Message: fmt.Sprintf("Network error: %v", err)}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &AnilistError{Message: "Rate limited by AniList API. Please wait a moment.", StatusCode: 429}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &AnilistError{Message: "Unauthorized. Your access token may be invalid or expired.", StatusCode: 401}
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, &AnilistError{Message: fmt.Sprintf("Invalid JSON response (HTTP %d)", resp.StatusCode)}
	}
	if errs, ok := data["errors"].([]any); ok && len(errs) > 0 {
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			if em, ok := e.(map[string]any); ok {
				if m, ok := em["message"].(string); ok {
					msgs = append(msgs, m)
					continue
				}
			}
			msgs = append(msgs, "Unknown error")
		}
		return nil, &AnilistError{Message: strings.Join(msgs, "; "), StatusCode: resp.StatusCode}
	}
	result, _ := data["data"].(map[string]any)
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func (c *Client) GetViewer() (User, error) {
	data, err := c.request(GetViewerQuery, nil)
	if err != nil {
		return User{}, err
	}
	return parseUser(getMap(data, "Viewer")), nil
}

func (c *Client) GetUserStats(userID int) (UserStats, error) {
	data, err := c.request(GetUserStatisticsQuery, map[string]any{"userId": userID})
	if err != nil {
		return UserStats{}, err
	}
	return parseUserStats(getMap(getMap(data, "User"), "statistics")), nil
}

// GetUserFavourites returns the user's favourite anime (cover cards only).
func (c *Client) GetUserFavourites(userID int) ([]Media, error) {
	data, err := c.request(GetUserFavouritesQuery, map[string]any{"id": userID})
	if err != nil {
		return nil, err
	}
	nodes := getSlice(getMap(getMap(getMap(data, "User"), "favourites"), "anime"), "nodes")
	return parseMediaList(nodes), nil
}

func (c *Client) GetTrending(mediaType string, page, perPage int) ([]Media, bool, error) {
	data, err := c.request(GetTrendingQuery, map[string]any{
		"type": mediaType, "page": page, "perPage": perPage,
	})
	if err != nil {
		return nil, false, err
	}
	pageData := getMap(data, "Page")
	media := parseMediaList(getSlice(pageData, "media"))
	hasNext := getBool(getMap(pageData, "pageInfo"), "hasNextPage")
	return media, hasNext, nil
}

func (c *Client) GetPopular(mediaType string, page, perPage int) ([]Media, error) {
	data, err := c.request(GetPopularQuery, map[string]any{
		"type": mediaType, "page": page, "perPage": perPage,
	})
	if err != nil {
		return nil, err
	}
	return parseMediaList(getSlice(getMap(data, "Page"), "media")), nil
}

func (c *Client) GetSeasonal(season string, year, page, perPage int) ([]Media, error) {
	data, err := c.request(GetSeasonalQuery, map[string]any{
		"season": season, "seasonYear": year, "page": page, "perPage": perPage,
	})
	if err != nil {
		return nil, err
	}
	return parseMediaList(getSlice(getMap(data, "Page"), "media")), nil
}

func (c *Client) SearchMedia(search, mediaType, genre, status, format string, sort []string, season string, seasonYear, page, perPage int, isAdult bool) ([]Media, bool, error) {
	if len(sort) == 0 {
		if search != "" {
			sort = []string{"SEARCH_MATCH"}
		} else {
			sort = []string{"POPULARITY_DESC"}
		}
	}
	variables := map[string]any{
		"page":    page,
		"perPage": perPage,
		"sort":    sort,
	}
	if search != "" {
		variables["search"] = search
	}
	if mediaType != "" {
		variables["type"] = mediaType
	}
	if genre != "" {
		variables["genre"] = genre
	}
	if status != "" {
		variables["status"] = status
	}
	if format != "" {
		variables["format"] = format
	}
	if season != "" {
		variables["season"] = season
	}
	if seasonYear != 0 {
		variables["seasonYear"] = seasonYear
	}
	if isAdult {
		variables["isAdult"] = isAdult
	}
	data, err := c.request(SearchMediaQuery, variables)
	if err != nil {
		return nil, false, err
	}
	pageData := getMap(data, "Page")
	media := parseMediaList(getSlice(pageData, "media"))
	hasNext := getBool(getMap(pageData, "pageInfo"), "hasNextPage")
	return media, hasNext, nil
}

func (c *Client) GetMediaDetails(id int) (Media, error) {
	data, err := c.request(GetMediaDetailsQuery, map[string]any{"id": id})
	if err != nil {
		return Media{}, err
	}
	return parseMediaDetailed(getMap(data, "Media")), nil
}

func (c *Client) GetMediaList(userID int, mediaType string) ([]MediaListGroup, error) {
	data, err := c.request(GetMediaListCollectionQuery, map[string]any{
		"userId": userID, "type": mediaType,
	})
	if err != nil {
		return nil, err
	}
	statusOrder := []string{"CURRENT", "REPEATING", "COMPLETED", "PAUSED", "DROPPED", "PLANNING"}
	byStatus := map[string]MediaListGroup{}
	for _, l := range getSlice(getMap(data, "MediaListCollection"), "lists") {
		lst, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if getBool(lst, "isCustomList") {
			continue
		}
		status := getString(lst, "status")
		if status == "" {
			status = "PLANNING"
		}
		entries := make([]MediaList, 0)
		for _, e := range getSlice(lst, "entries") {
			if em, ok := e.(map[string]any); ok {
				entries = append(entries, parseMediaListEntry(em))
			}
		}
		byStatus[status] = MediaListGroup{Name: getString(lst, "name"), Status: status, Entries: entries}
	}
	groups := make([]MediaListGroup, 0, len(byStatus))
	for _, s := range statusOrder {
		if g, ok := byStatus[s]; ok {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

func (c *Client) SaveListEntry(mediaID int, status string, progress int, score float64, notes string, entryID *int, repeat int, private bool) (MediaList, error) {
	variables := map[string]any{
		"mediaId":  mediaID,
		"status":   status,
		"progress": progress,
		"score":    score,
		"repeat":   repeat,
		"private":  private,
	}
	if notes != "" {
		variables["notes"] = notes
	}
	if entryID != nil {
		variables["id"] = *entryID
	}
	data, err := c.request(SaveMediaListEntryMutation, variables)
	if err != nil {
		return MediaList{}, err
	}
	return parseSavedEntry(getMap(data, "SaveMediaListEntry")), nil
}

func (c *Client) DeleteListEntry(entryID int) (bool, error) {
	data, err := c.request(DeleteMediaListEntryMutation, map[string]any{"id": entryID})
	if err != nil {
		return false, err
	}
	return getBool(getMap(data, "DeleteMediaListEntry"), "deleted"), nil
}

func getMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	if v, ok := m[key].(map[string]any); ok && v != nil {
		return v
	}
	return map[string]any{}
}

func getSlice(m map[string]any, key string) []any {
	if v, ok := m[key].([]any); ok {
		return v
	}
	return nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStringPtr(m map[string]any, key string) *string {
	if v, ok := m[key].(string); ok {
		return &v
	}
	return nil
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func getIntPtr(m map[string]any, key string) *int {
	if v, ok := m[key].(float64); ok {
		i := int(v)
		return &i
	}
	return nil
}

func getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getFloatPtr(m map[string]any, key string) *float64 {
	if v, ok := m[key].(float64); ok {
		return &v
	}
	return nil
}

func getStringList(m map[string]any, key string) []string {
	out := []string{}
	for _, v := range getSlice(m, key) {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func parseMediaList(items []any) []Media {
	out := make([]Media, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, parseMedia(m))
		}
	}
	return out
}

func parseMedia(m map[string]any) Media {
	title := getMap(m, "title")
	cover := getMap(m, "coverImage")
	studios := []string{}
	for _, s := range getSlice(getMap(m, "studios"), "nodes") {
		if sm, ok := s.(map[string]any); ok {
			studios = append(studios, getString(sm, "name"))
		}
	}
	tags := []string{}
	for _, t := range getSlice(m, "tags") {
		if tm, ok := t.(map[string]any); ok {
			if getBool(tm, "isMediaSpoiler") {
				continue
			}
			tags = append(tags, getString(tm, "name"))
		}
	}
	start := getMap(m, "startDate")

	romaji := getString(title, "romaji")
	if romaji == "" {
		romaji = "Unknown"
	}
	mediaType := getString(m, "type")
	if mediaType == "" {
		mediaType = "ANIME"
	}

	return Media{
		ID:                   getInt(m, "id"),
		TitleRomaji:          romaji,
		TitleEnglish:         getStringPtr(title, "english"),
		TitleNative:          getStringPtr(title, "native"),
		Type:                 mediaType,
		Format:               getStringPtr(m, "format"),
		Status:               getStringPtr(m, "status"),
		Description:          getStringPtr(m, "description"),
		Episodes:             getIntPtr(m, "episodes"),
		Chapters:             getIntPtr(m, "chapters"),
		Volumes:              getIntPtr(m, "volumes"),
		Duration:             getIntPtr(m, "duration"),
		Score:                getFloatPtr(m, "meanScore"),
		Genres:               getStringList(m, "genres"),
		Tags:                 tags,
		CoverImageMedium:     getStringPtr(cover, "medium"),
		CoverImageLarge:      getStringPtr(cover, "large"),
		CoverImageExtraLarge: getStringPtr(cover, "extraLarge"),
		BannerImage:          getStringPtr(m, "bannerImage"),
		Season:               getStringPtr(m, "season"),
		SeasonYear:           getIntPtr(m, "seasonYear"),
		Year:                 getIntPtr(start, "year"),
		Popularity:           getInt(m, "popularity"),
		Favourites:           getInt(m, "favourites"),
		Studios:              studios,
		Source:               getStringPtr(m, "source"),
		Country:              getStringPtr(m, "countryOfOrigin"),
		IsAdult:              getBool(m, "isAdult"),
		SiteURL:              getStringPtr(m, "siteUrl"),
	}
}

func parseMediaDetailed(m map[string]any) Media {
	media := parseMedia(m)

	characters := []Character{}
	for _, e := range getSlice(getMap(m, "characters"), "edges") {
		edge, ok := e.(map[string]any)
		if !ok {
			continue
		}
		node := getMap(edge, "node")
		name := getMap(node, "name")
		img := getMap(node, "image")
		imageURL := getStringPtr(img, "large")
		if imageURL == nil {
			imageURL = getStringPtr(img, "medium")
		}
		role := getString(edge, "role")
		if role == "" {
			role = "SUPPORTING"
		}
		characters = append(characters, Character{
			ID:         getInt(node, "id"),
			NameFull:   getString(name, "full"),
			NameNative: getStringPtr(name, "native"),
			ImageURL:   imageURL,
			Role:       role,
		})
	}
	media.Characters = characters

	if tr := getMap(m, "trailer"); getString(tr, "id") != "" {
		media.Trailer = &Trailer{ID: getString(tr, "id"), Site: getString(tr, "site")}
	}

	for _, e := range getSlice(getMap(m, "relations"), "edges") {
		edge, ok := e.(map[string]any)
		if !ok {
			continue
		}
		node := getMap(edge, "node")
		if getInt(node, "id") == 0 {
			continue
		}
		media.Relations = append(media.Relations, Relation{
			RelationType: getString(edge, "relationType"),
			Media:        parseMedia(node),
		})
	}

	for _, n := range getSlice(getMap(m, "recommendations"), "nodes") {
		nm, ok := n.(map[string]any)
		if !ok {
			continue
		}
		rec := getMap(nm, "mediaRecommendation")
		if getInt(rec, "id") == 0 {
			continue
		}
		media.Recommendations = append(media.Recommendations, parseMedia(rec))
	}

	if entry, ok := m["mediaListEntry"].(map[string]any); ok && entry != nil {
		media.ListEntryID = getIntPtr(entry, "id")
		media.ListStatus = getStringPtr(entry, "status")
		media.ListProgress = getIntPtr(entry, "progress")
		media.ListScore = getFloatPtr(entry, "score")
		media.ListNotes = getStringPtr(entry, "notes")
	}

	return media
}

func parseMediaListEntry(e map[string]any) MediaList {
	return MediaList{
		ID:        getInt(e, "id"),
		Media:     parseMedia(getMap(e, "media")),
		Status:    orDefault(getString(e, "status"), "PLANNING"),
		Progress:  getInt(e, "progress"),
		Score:     getFloat(e, "score"),
		Notes:     getStringPtr(e, "notes"),
		Repeat:    getInt(e, "repeat"),
		Private:   getBool(e, "private"),
		UpdatedAt: getIntPtr(e, "updatedAt"),
	}
}

func parseSavedEntry(e map[string]any) MediaList {
	mediaData := getMap(e, "media")
	title := getMap(mediaData, "title")
	media := Media{
		ID:           getInt(mediaData, "id"),
		TitleRomaji:  getString(title, "romaji"),
		TitleEnglish: getStringPtr(title, "english"),
		Type:         orDefault(getString(mediaData, "type"), "ANIME"),
		Episodes:     getIntPtr(mediaData, "episodes"),
		Chapters:     getIntPtr(mediaData, "chapters"),
	}
	return MediaList{
		ID:       getInt(e, "id"),
		Media:    media,
		Status:   orDefault(getString(e, "status"), "PLANNING"),
		Progress: getInt(e, "progress"),
		Score:    getFloat(e, "score"),
		Notes:    getStringPtr(e, "notes"),
		Repeat:   getInt(e, "repeat"),
		Private:  getBool(e, "private"),
	}
}

func parseUser(u map[string]any) User {
	avatar := getMap(u, "avatar")
	return User{
		ID:           getInt(u, "id"),
		Name:         getString(u, "name"),
		AvatarMedium: getStringPtr(avatar, "medium"),
		AvatarLarge:  getStringPtr(avatar, "large"),
		BannerImage:  getStringPtr(u, "bannerImage"),
		About:        getStringPtr(u, "about"),
		SiteURL:      getStringPtr(u, "siteUrl"),
		CreatedAt:    getIntPtr(u, "createdAt"),
		UpdatedAt:    getIntPtr(u, "updatedAt"),
	}
}

func parseUserStats(stats map[string]any) UserStats {
	anime := getMap(stats, "anime")
	manga := getMap(stats, "manga")

	statusCount := func(statuses []any, name string) int {
		for _, s := range statuses {
			if sm, ok := s.(map[string]any); ok && getString(sm, "status") == name {
				return getInt(sm, "count")
			}
		}
		return 0
	}
	animeStatuses := getSlice(anime, "statuses")
	mangaStatuses := getSlice(manga, "statuses")

	topAnimeGenres := []GenreStats{}
	for _, g := range firstN(getSlice(anime, "genres"), 10) {
		if gm, ok := g.(map[string]any); ok {
			topAnimeGenres = append(topAnimeGenres, GenreStats{
				Genre:          getString(gm, "genre"),
				Count:          getInt(gm, "count"),
				MeanScore:      getFloat(gm, "meanScore"),
				MinutesWatched: getInt(gm, "minutesWatched"),
			})
		}
	}
	topMangaGenres := []GenreStats{}
	for _, g := range firstN(getSlice(manga, "genres"), 10) {
		if gm, ok := g.(map[string]any); ok {
			topMangaGenres = append(topMangaGenres, GenreStats{
				Genre:        getString(gm, "genre"),
				Count:        getInt(gm, "count"),
				MeanScore:    getFloat(gm, "meanScore"),
				ChaptersRead: getInt(gm, "chaptersRead"),
			})
		}
	}
	topTags := []TagStats{}
	for _, t := range firstN(getSlice(anime, "tags"), 10) {
		if tm, ok := t.(map[string]any); ok {
			topTags = append(topTags, TagStats{
				Tag:       getString(getMap(tm, "tag"), "name"),
				Count:     getInt(tm, "count"),
				MeanScore: getFloat(tm, "meanScore"),
			})
		}
	}

	return UserStats{
		AnimeCount:           getInt(anime, "count"),
		AnimeMeanScore:       getFloat(anime, "meanScore"),
		AnimeMinutesWatched:  getInt(anime, "minutesWatched"),
		AnimeEpisodesWatched: getInt(anime, "episodesWatched"),
		MangaCount:           getInt(manga, "count"),
		MangaMeanScore:       getFloat(manga, "meanScore"),
		MangaChaptersRead:    getInt(manga, "chaptersRead"),
		MangaVolumesRead:     getInt(manga, "volumesRead"),
		AnimeWatching:        statusCount(animeStatuses, "CURRENT"),
		AnimeCompleted:       statusCount(animeStatuses, "COMPLETED"),
		AnimePaused:          statusCount(animeStatuses, "PAUSED"),
		AnimeDropped:         statusCount(animeStatuses, "DROPPED"),
		AnimePlanning:        statusCount(animeStatuses, "PLANNING"),
		MangaReading:         statusCount(mangaStatuses, "CURRENT"),
		MangaCompleted:       statusCount(mangaStatuses, "COMPLETED"),
		MangaPaused:          statusCount(mangaStatuses, "PAUSED"),
		MangaDropped:         statusCount(mangaStatuses, "DROPPED"),
		MangaPlanning:        statusCount(mangaStatuses, "PLANNING"),
		TopAnimeGenres:       topAnimeGenres,
		TopMangaGenres:       topMangaGenres,
		TopTags:              topTags,
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func firstN(items []any, n int) []any {
	if len(items) > n {
		return items[:n]
	}
	return items
}
