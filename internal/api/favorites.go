package api

import (
	"context"
	"fmt"
)

// FavoritesList returns a page of the signed-in user's favorites for one
// AniList category. The category selects a fixed GraphQL field and shape.
func (c *Client) FavoritesList(ctx context.Context, kind string, page, perPage int) (map[string]any, error) {
	if c.token == "" {
		return nil, fmt.Errorf("favorites require AniList login")
	}
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 20
	}
	if page < 1 || perPage < 1 || perPage > 25 || page > 5000/perPage {
		return nil, fmt.Errorf("page must be positive, perPage 1–25, and page × perPage no more than 5000")
	}
	field := map[string]string{"anime": "anime", "manga": "manga", "characters": "characters", "staff": "staff", "studios": "studios"}[kind]
	if field == "" {
		return nil, fmt.Errorf("favorite kind must be anime, manga, characters, staff, or studios")
	}
	selection := "id name { full native }"
	switch kind {
	case "anime", "manga":
		selection = "id title { romaji english }"
	case "studios":
		selection = "id name"
	}
	doc := fmt.Sprintf("query($page: Int, $perPage: Int) { Viewer { favourites { %s(page: $page, perPage: $perPage) { nodes { %s } pageInfo { hasNextPage } } } } }", field, selection)
	return c.Execute(ctx, doc, map[string]any{"page": page, "perPage": perPage})
}
