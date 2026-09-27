package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mediaSearchInput struct {
	Query   string `json:"query,omitempty" jsonschema:"Anime or manga title"`
	Type    string `json:"type,omitempty" jsonschema:"ANIME or MANGA"`
	Genre   string `json:"genre,omitempty"`
	Status  string `json:"status,omitempty"`
	Format  string `json:"format,omitempty"`
	Season  string `json:"season,omitempty"`
	Year    int    `json:"year,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
}

type mediaIDInput struct {
	ID int `json:"id" jsonschema:"Positive AniList media ID"`
}

type browseInput struct {
	Type    string `json:"type,omitempty"`
	Season  string `json:"season,omitempty"`
	Year    int    `json:"year,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
}

type listInput struct {
	Type string `json:"type,omitempty" jsonschema:"ANIME or MANGA"`
}

type listSetInput struct {
	MediaID  int     `json:"mediaId" jsonschema:"Positive AniList media ID"`
	Status   string  `json:"status,omitempty" jsonschema:"PLANNING, CURRENT, COMPLETED, PAUSED, DROPPED, or REPEATING"`
	Progress int     `json:"progress,omitempty"`
	Score    float64 `json:"score,omitempty"`
	Notes    string  `json:"notes,omitempty"`
}

type listDeleteInput struct {
	EntryID int `json:"entryId" jsonschema:"Media list entry ID, not media ID"`
}

func pageValues(page, perPage int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 20
	}
	if page < 1 || perPage < 1 || perPage > 50 || page > 5000/perPage {
		return 0, 0, errors.New("page must be positive, perPage must be 1–50, and page × perPage must not exceed 5000")
	}
	return page, perPage, nil
}

func (s *Server) registerMedia(server *mcp.Server) {
	mcp.AddTool(server, readTool("media_search", "Search anime and manga by title and filters"), func(ctx context.Context, req *mcp.CallToolRequest, in mediaSearchInput) (*mcp.CallToolResult, any, error) {
		page, count, err := pageValues(in.Page, in.PerPage)
		if err != nil {
			return nil, nil, err
		}
		client, err := s.publicClient()
		if err != nil {
			return nil, nil, err
		}
		items, next, err := client.SearchMedia(in.Query, strings.ToUpper(in.Type), in.Genre, strings.ToUpper(in.Status), strings.ToUpper(in.Format), nil, strings.ToUpper(in.Season), in.Year, page, count, false)
		return nil, map[string]any{"items": items, "hasNextPage": next}, err
	})
	mcp.AddTool(server, readTool("media_get", "Get anime or manga details by media ID"), func(ctx context.Context, req *mcp.CallToolRequest, in mediaIDInput) (*mcp.CallToolResult, any, error) {
		if in.ID < 1 {
			return nil, nil, errors.New("media ID must be positive")
		}
		client, err := s.publicClient()
		if err != nil {
			return nil, nil, err
		}
		item, err := client.GetMediaDetails(in.ID)
		return nil, item, err
	})
	for _, kind := range []string{"trending", "popular", "seasonal"} {
		kind := kind
		mcp.AddTool(server, readTool("media_"+kind, "Browse "+kind+" anime or manga"), func(ctx context.Context, req *mcp.CallToolRequest, in browseInput) (*mcp.CallToolResult, any, error) {
			page, count, err := pageValues(in.Page, in.PerPage)
			if err != nil {
				return nil, nil, err
			}
			client, err := s.publicClient()
			if err != nil {
				return nil, nil, err
			}
			switch kind {
			case "trending":
				items, next, err := client.GetTrending(strings.ToUpper(in.Type), page, count)
				return nil, map[string]any{"items": items, "hasNextPage": next}, err
			case "popular":
				items, err := client.GetPopular(strings.ToUpper(in.Type), page, count)
				return nil, items, err
			default:
				if in.Season == "" || in.Year < 1 {
					return nil, nil, errors.New("season and year are required")
				}
				items, err := client.GetSeasonal(strings.ToUpper(in.Season), in.Year, page, count)
				return nil, items, err
			}
		})
	}
	mcp.AddTool(server, readTool("profile", "View the authenticated user's profile and statistics"), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		client, err := s.client()
		if err != nil {
			return nil, nil, err
		}
		user, err := client.GetViewer()
		if err != nil {
			return nil, nil, err
		}
		stats, err := client.GetUserStats(user.ID)
		user.Stats = &stats
		return nil, user, err
	})
	mcp.AddTool(server, readTool("favorites", "List the authenticated user's favorite anime"), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		client, err := s.client()
		if err != nil {
			return nil, nil, err
		}
		user, err := client.GetViewer()
		if err != nil {
			return nil, nil, err
		}
		items, err := client.GetUserFavourites(user.ID)
		return nil, items, err
	})
	mcp.AddTool(server, readTool("list_show", "Show the authenticated user's anime or manga list"), func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		client, err := s.client()
		if err != nil {
			return nil, nil, err
		}
		user, err := client.GetViewer()
		if err != nil {
			return nil, nil, err
		}
		kind := strings.ToUpper(in.Type)
		if kind == "" {
			kind = "ANIME"
		}
		groups, err := client.GetMediaList(user.ID, kind)
		return nil, groups, err
	})
	mcp.AddTool(server, writeTool("list_set", "Add or update a media list entry by media ID", false), func(ctx context.Context, req *mcp.CallToolRequest, in listSetInput) (*mcp.CallToolResult, any, error) {
		if in.MediaID < 1 || in.Progress < 0 || in.Score < 0 || in.Score > 10 {
			return nil, nil, errors.New("mediaId must be positive, progress nonnegative, and score 0–10")
		}
		client, err := s.client()
		if err != nil {
			return nil, nil, err
		}
		status := strings.ToUpper(in.Status)
		if status == "" {
			status = "PLANNING"
		}
		entry, err := client.SaveListEntry(in.MediaID, status, in.Progress, in.Score, in.Notes, nil, 0, false)
		return nil, entry, err
	})
	mcp.AddTool(server, writeTool("list_delete", "Delete a media list entry by its entry ID", true), func(ctx context.Context, req *mcp.CallToolRequest, in listDeleteInput) (*mcp.CallToolResult, any, error) {
		if in.EntryID < 1 {
			return nil, nil, errors.New("entryId must be positive")
		}
		client, err := s.client()
		if err != nil {
			return nil, nil, err
		}
		deleted, err := client.DeleteListEntry(in.EntryID)
		return nil, map[string]bool{"deleted": deleted}, err
	})
}
