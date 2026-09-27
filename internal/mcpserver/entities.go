package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/noyukii/ALcli/internal/api"
)

type namedInput struct {
	ID      int    `json:"id,omitempty" jsonschema:"Positive ID for get or related-list filters"`
	Search  string `json:"search,omitempty" jsonschema:"Search text where the kind supports it"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"perPage,omitempty"`
}

type graphqlInput struct {
	Document  string         `json:"document" jsonschema:"Complete AniList GraphQL document"`
	Variables map[string]any `json:"variables,omitempty" jsonschema:"GraphQL variable values"`
}

func (s *Server) registerEntities(server *mcp.Server) {
	for _, kind := range api.NamedKinds() {
		kind := kind
		if kind != "messages" && kind != "notifications" && kind != "follows" {
			mcp.AddTool(server, readTool(kind+"_get", "Get one AniList "+kind+" record by ID"), func(ctx context.Context, req *mcp.CallToolRequest, in namedInput) (*mcp.CallToolResult, any, error) {
				client, err := s.publicClient()
				if err != nil {
					return nil, nil, err
				}
				value, err := client.Named(ctx, kind, "get", api.NamedInput{ID: in.ID})
				return nil, value, err
			})
		}
		mcp.AddTool(server, readTool(kind+"_list", "List or search AniList "+kind+" with pagination"), func(ctx context.Context, req *mcp.CallToolRequest, in namedInput) (*mcp.CallToolResult, any, error) {
			client, err := s.publicClient()
			if err != nil {
				return nil, nil, err
			}
			value, err := client.Named(ctx, kind, "list", api.NamedInput{ID: in.ID, Search: in.Search, Page: in.Page, PerPage: in.PerPage})
			return nil, value, err
		})
	}
}

func (s *Server) registerGraphQL(server *mcp.Server) {
	mcp.AddTool(server, writeTool("graphql_query", "Run an AniList GraphQL query when a named tool does not cover the needed fields; some AniList query arguments change account state", false), func(ctx context.Context, req *mcp.CallToolRequest, in graphqlInput) (*mcp.CallToolResult, any, error) {
		if api.DocumentKind(in.Document) != "query" {
			return nil, nil, errors.New("document must begin with a GraphQL query")
		}
		client, err := s.publicClient()
		if err != nil {
			return nil, nil, err
		}
		value, err := client.Execute(ctx, in.Document, in.Variables)
		return nil, value, err
	})
	mcp.AddTool(server, writeTool("graphql_mutation", "Run an authenticated AniList GraphQL mutation when a named tool does not cover the operation", true), func(ctx context.Context, req *mcp.CallToolRequest, in graphqlInput) (*mcp.CallToolResult, any, error) {
		if api.DocumentKind(in.Document) != "mutation" {
			return nil, nil, errors.New("document must begin with a GraphQL mutation")
		}
		client, err := s.client()
		if err != nil {
			return nil, nil, err
		}
		value, err := client.Execute(ctx, in.Document, in.Variables)
		return nil, value, err
	})
}
