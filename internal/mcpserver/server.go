package mcpserver

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/config"
)

//go:embed assets/alcli.png
var icon []byte

type Server struct {
	auth      *browserAuth
	newClient func(string) *api.Client
}

func New() *Server {
	return &Server{auth: newBrowserAuth(), newClient: api.NewClient}
}

func (s *Server) client() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if !cfg.IsAuthenticated() {
		return nil, errors.New("not logged in; use auth_login or run `al auth login`")
	}
	return s.newClient(cfg.AccessToken), nil
}

func (s *Server) publicClient() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return s.newClient(cfg.AccessToken), nil
}

func (s *Server) mcp() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "alcli",
		Title:   "ALcli for AniList (unofficial)",
		Version: "0.1.3",
		Icons: []mcp.Icon{{
			Source:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(icon),
			MIMEType: "image/png",
			Sizes:    []string{"1000x1000"},
		}},
	}, nil)
	s.registerMedia(server)
	s.registerFavorites(server)
	s.registerEntities(server)
	s.registerGraphQL(server)
	s.registerMutations(server)
	s.registerAccount(server)
	return server
}

func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcp().Run(ctx, &mcp.StdioTransport{})
}

func readTool(name, description string) *mcp.Tool {
	return &mcp.Tool{Name: name, Description: description, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}
}

func writeTool(name, description string, destructive bool) *mcp.Tool {
	return &mcp.Tool{Name: name, Description: description, Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive}}
}
