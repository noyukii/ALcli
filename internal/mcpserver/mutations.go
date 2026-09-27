package mcpserver

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/noyukii/ALcli/internal/api"
)

type mutationInput struct {
	Variables map[string]any `json:"variables" jsonschema:"AniList mutation arguments; see tool description"`
}

func (s *Server) registerMutations(server *mcp.Server) {
	for _, name := range api.NamedMutationNames() {
		name := name
		mcp.AddTool(server, writeTool(name, api.NamedMutationHelp(name), true), func(ctx context.Context, req *mcp.CallToolRequest, in mutationInput) (*mcp.CallToolResult, any, error) {
			client, err := s.client()
			if err != nil {
				return nil, nil, err
			}
			value, err := client.NamedMutation(ctx, name, in.Variables)
			return nil, value, err
		})
	}
}
