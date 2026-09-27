package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/noyukii/ALcli/internal/config"
)

func Token() (string, error) {
	path := filepath.Join(config.Dir(), "mcp-token")
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	if data, err := os.ReadFile(path); err == nil {
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return "", errors.New("MCP token file must be a private regular file")
		}
		value := strings.TrimSpace(string(data))
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != 32 {
			return "", errors.New("MCP token file is invalid")
		}
		return value, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read MCP token: %w", err)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate MCP token: %w", err)
	}
	token := hex.EncodeToString(secret)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return Token()
	}
	if err != nil {
		return "", fmt.Errorf("create MCP token: %w", err)
	}
	if _, err := f.WriteString(token + "\n"); err != nil {
		f.Close()
		return "", fmt.Errorf("write MCP token: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close MCP token: %w", err)
	}
	return token, nil
}

func (s *Server) HTTPHandler(token, host string) http.Handler {
	server := s.mcp()
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, CrossOriginProtection: http.NewCrossOriginProtection()})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host || r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+host {
			http.Error(w, "invalid local origin", http.StatusForbidden)
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(provided) != len(token) || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "MCP bearer token required", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func (s *Server) RunHTTP(ctx context.Context, address string, infoOut io.Writer) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		return errors.New("HTTP MCP must listen on 127.0.0.1:PORT")
	}
	token, err := Token()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen for local MCP: %w", err)
	}
	httpServer := &http.Server{Handler: s.HTTPHandler(token, listener.Addr().String()), ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintf(infoOut, "ALcli MCP listening at http://%s/mcp\nUse `al mcp token` to show its local bearer token.\n", listener.Addr())
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve local MCP: %w", err)
	}
	return nil
}
