package api

import (
	"context"
	"fmt"
)

// NamedInput is the shared input for the CLI and MCP's named AniList reads.
type NamedInput struct {
	ID      int
	Search  string
	Page    int
	PerPage int
}

type namedSpec struct {
	root      string
	pageField string
	selection string
	filter    string
	needsAuth bool
	fixedArgs string
}

var namedQueries = map[string]namedSpec{
	"characters":      {"Character", "characters", "id name { full native } image { large } description siteUrl", "search", false, ""},
	"staff":           {"Staff", "staff", "id name { full native } image { large } description siteUrl", "search", false, ""},
	"studios":         {"Studio", "studios", "id name isAnimationStudio siteUrl", "search", false, ""},
	"users":           {"User", "users", "id name avatar { large } about siteUrl", "search", false, ""},
	"airing":          {"AiringSchedule", "airingSchedules", "id airingAt episode media { id title { romaji english } }", "mediaId", false, ""},
	"activities":      {"Activity", "activities", "__typename ... on TextActivity { id text user { id name } } ... on ListActivity { id status progress media { id title { romaji } } } ... on MessageActivity { id message messenger { id name } }", "userId", false, ""},
	"messages":        {"", "activities", "__typename ... on MessageActivity { id message messenger { id name } recipient { id name } }", "messengerId", true, "type: MESSAGE"},
	"follows":         {"", "following", "id name avatar { large } siteUrl", "userId", false, ""},
	"forums":          {"Thread", "threads", "id title body user { id name }", "search", false, ""},
	"reviews":         {"Review", "reviews", "id summary body score user { id name } media { id title { romaji } }", "mediaId", false, ""},
	"recommendations": {"Recommendation", "recommendations", "id rating media { id title { romaji } } mediaRecommendation { id title { romaji } }", "mediaId", false, ""},
	"notifications":   {"", "notifications", "__typename ... on AiringNotification { id createdAt media { id title { romaji } } } ... on FollowingNotification { id createdAt user { id name } }", "", true, ""},
}

// NamedKinds returns the stable names exposed by both command and MCP surfaces.
func NamedKinds() []string {
	return []string{"characters", "staff", "studios", "users", "airing", "activities", "messages", "follows", "forums", "reviews", "recommendations", "notifications"}
}

// Named runs a fixed AniList query. action is "get" or "list"; the latter uses
// a search term for searchable kinds and an ID filter for related collections.
func (c *Client) Named(ctx context.Context, kind, action string, in NamedInput) (map[string]any, error) {
	spec, ok := namedQueries[kind]
	if !ok {
		return nil, fmt.Errorf("unknown AniList kind %q", kind)
	}
	if spec.needsAuth && c.token == "" {
		return nil, fmt.Errorf("%s requires AniList login", kind)
	}
	if action == "get" {
		if spec.root == "" {
			return nil, fmt.Errorf("%s has no individual get query; use list", kind)
		}
		if in.ID < 1 {
			return nil, fmt.Errorf("ID must be positive")
		}
		doc := fmt.Sprintf("query($id: Int) { %s(id: $id) { %s } }", spec.root, spec.selection)
		return c.Execute(ctx, doc, map[string]any{"id": in.ID})
	}
	if action != "list" {
		return nil, fmt.Errorf("unknown action %q", action)
	}
	if in.Page == 0 {
		in.Page = 1
	}
	if in.PerPage == 0 {
		in.PerPage = 20
	}
	if in.Page < 1 || in.PerPage < 1 || in.PerPage > 50 || in.Page > 5000/in.PerPage {
		return nil, fmt.Errorf("page must be positive, perPage must be 1–50, and page × perPage must not exceed 5000")
	}
	args := ""
	variables := map[string]any{"page": in.Page, "perPage": in.PerPage}
	declaration := ""
	switch spec.filter {
	case "search":
		if in.Search != "" {
			declaration = ", $search: String"
			args = "(search: $search)"
			variables["search"] = in.Search
		}
	case "mediaId", "userId", "messengerId":
		if in.ID > 0 {
			declaration = ", $id: Int"
			if kind == "follows" {
				declaration = ", $id: Int!"
			}
			args = "(" + spec.filter + ": $id)"
			variables["id"] = in.ID
		}
	}
	if spec.fixedArgs != "" {
		if args == "" {
			args = "(" + spec.fixedArgs + ")"
		} else {
			args = args[:len(args)-1] + ", " + spec.fixedArgs + ")"
		}
	}
	if kind == "follows" && in.ID < 1 {
		return nil, fmt.Errorf("follows list requires a user ID")
	}
	doc := fmt.Sprintf("query($page: Int, $perPage: Int%s) { Page(page: $page, perPage: $perPage) { %s%s { %s } pageInfo { hasNextPage } } }", declaration, spec.pageField, args, spec.selection)
	return c.Execute(ctx, doc, variables)
}
