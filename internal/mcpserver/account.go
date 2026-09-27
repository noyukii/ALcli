package mcpserver

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/noyukii/ALcli/internal/config"
)

func BrowserLogin(ctx context.Context, out io.Writer) error {
	flow := newBrowserAuth()
	url, err := flow.Start()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Open this URL to authorize ALcli:", url)
	var opener *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		opener = exec.Command("open", url)
	case "linux":
		opener = exec.Command("xdg-open", url)
	}
	if opener != nil && opener.Start() == nil {
		go func() { _ = opener.Wait() }()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, completed, lastError := flow.Status()
		if completed {
			fmt.Fprintln(out, "Logged in successfully.")
			return nil
		}
		if lastError != "" {
			return fmt.Errorf("browser login: %s", lastError)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("browser login: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (s *Server) registerAccount(server *mcp.Server) {
	mcp.AddTool(server, writeTool("auth_login", "Start local browser authentication with AniList and return its authorization URL", false), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		url, err := s.auth.Start()
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"authorizationUrl": url, "next": "Authorize in your browser, then call auth_status."}, nil
	})
	mcp.AddTool(server, readTool("auth_status", "Check whether ALcli is authenticated and whether browser login has completed"), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		cfg, err := config.Load()
		if err != nil {
			return nil, nil, err
		}
		pending, completed, lastError := s.auth.Status()
		return nil, map[string]any{"authenticated": cfg.IsAuthenticated(), "pending": pending, "completed": completed, "error": lastError}, nil
	})
	mcp.AddTool(server, readTool("auth_login_status", "Check progress of a browser login started with auth_login"), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		pending, completed, lastError := s.auth.Status()
		return nil, map[string]any{"pending": pending, "completed": completed, "error": lastError}, nil
	})
	mcp.AddTool(server, writeTool("auth_logout", "Remove the locally saved AniList access token", true), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		cfg, err := config.Load()
		if err != nil {
			return nil, nil, err
		}
		wasLoggedIn := cfg.IsAuthenticated()
		cfg.AccessToken = ""
		if err := cfg.Save(); err != nil {
			return nil, nil, err
		}
		return nil, map[string]bool{"loggedOut": wasLoggedIn}, nil
	})
	mcp.AddTool(server, readTool("auth_config_path", "Show the local ALcli config file path without revealing its contents"), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		return nil, map[string]string{"path": config.File()}, nil
	})
}
