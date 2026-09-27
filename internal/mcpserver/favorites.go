package mcpserver

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type favoritesInput struct {
	Kind    string `json:"kind" jsonschema:"anime, manga, characters, staff, or studios"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
}

func (s *Server) registerFavorites(server *mcp.Server) {
	mcp.AddTool(server, readTool("favorites_list", "List the signed-in user's favorite anime, manga, characters, staff, or studios"), func(ctx context.Context, req *mcp.CallToolRequest, in favoritesInput) (*mcp.CallToolResult, any, error) {
		client, err := s.client()
		if err != nil {
			return nil, nil, err
		}
		value, err := client.FavoritesList(ctx, in.Kind, in.Page, in.PerPage)
		return nil, value, err
	})
}
