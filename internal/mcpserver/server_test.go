package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/noyukii/ALcli/internal/api"
	"github.com/noyukii/ALcli/internal/config"
)

func testServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	t.Setenv("ALCLI_CONFIG_DIR", t.TempDir())
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer saved-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if !strings.Contains(payload.Query, "Viewer") && !strings.Contains(payload.Query, "DeleteActivity") {
			t.Errorf("query = %q", payload.Query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"Viewer":{"id":7,"name":"Demo"},"DeleteActivity":{"deleted":true}}}`)
	}))
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AccessToken = "saved-token"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	s := New()
	s.newClient = func(token string) *api.Client { return api.NewClientAt(token, endpoint.URL) }
	return s, endpoint
}

func TestMCPDiscoveryAndGraphQLCalls(t *testing.T) {
	s, endpoint := testServer(t)
	defer endpoint.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	st, ct := mcp.NewInMemoryTransports()
	serverSession, err := s.mcp().Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, tool := range tools.Tools {
		found[tool.Name] = true
	}
	for _, name := range []string{"media_search", "favorites_list", "graphql_query", "graphql_mutation", "auth_login", "auth_login_status", "follows_list", "notifications_list"} {
		if !found[name] {
			t.Errorf("missing tool %s", name)
		}
	}
	for _, tc := range []struct{ name, doc string }{
		{"graphql_query", "query { Viewer { id name } }"},
		{"graphql_mutation", "mutation { DeleteActivity(id: 2) { deleted } }"},
	} {
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: map[string]any{"document": tc.doc}})
		if err != nil || result.IsError {
			t.Fatalf("%s: result=%#v err=%v", tc.name, result, err)
		}
	}
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "activities_delete", Arguments: map[string]any{"variables": map[string]any{"id": 2}}})
	if err != nil || result.IsError {
		t.Fatalf("activities_delete: result=%#v err=%v", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["DeleteActivity"] == nil {
		t.Fatalf("activities_delete structured result=%#v", result.StructuredContent)
	}
}

func TestLocalHTTPAccessControls(t *testing.T) {
	s, endpoint := testServer(t)
	defer endpoint.Close()
	handler := s.HTTPHandler("secret", "127.0.0.1:3333")
	cases := []struct {
		host, origin, bearer string
		want                 int
	}{
		{"127.0.0.1:3333", "", "", 401},
		{"evil.test", "", "Bearer secret", 403},
		{"127.0.0.1:3333", "https://evil.test", "Bearer secret", 403},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("POST", "http://127.0.0.1:3333/mcp", strings.NewReader(`{}`))
		req.Host = tc.host
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Authorization", tc.bearer)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("host=%s origin=%s: %d, want %d", tc.host, tc.origin, w.Code, tc.want)
		}
	}
}

type bearerTransport struct{ base http.RoundTripper }

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer secret")
	return t.base.RoundTrip(clone)
}

func TestLocalHTTPMCPCall(t *testing.T) {
	s, endpoint := testServer(t)
	defer endpoint.Close()
	var host string
	var handler http.Handler
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	defer proxy.Close()
	host = strings.TrimPrefix(proxy.URL, "http://")
	handler = s.HTTPHandler("secret", host)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: proxy.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{http.DefaultTransport}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "auth_status", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("auth_status result=%#v err=%v", result, err)
	}
}

func TestBrowserLoginCallbackSavesToken(t *testing.T) {
	t.Setenv("ALCLI_CONFIG_DIR", t.TempDir())
	t.Setenv("ALCLI_OAUTH_CLIENT_ID", "")
	flow := newBrowserAuth()
	flow.address = "127.0.0.1:0"
	authURL, err := flow.Start()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(authURL, "client_id=52200") || !strings.Contains(authURL, "redirect_uri=") || !strings.Contains(authURL, "state=") {
		t.Fatalf("URL = %s", authURL)
	}
	callbackHost := flow.listener.Addr().String()
	response, err := http.Get("http://" + callbackHost + "/callback")
	if err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(page), "X-ALcli-Nonce':'")
	if len(parts) < 2 {
		t.Fatal("nonce missing from callback page")
	}
	nonce := strings.Split(parts[1], "'")[0]
	bad, err := http.NewRequest("POST", "http://"+callbackHost+"/complete", strings.NewReader(`{"token":"attacker-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	bad.Header.Set("Origin", "https://other.example")
	bad.Header.Set("X-ALcli-Nonce", nonce)
	denied, err := http.DefaultClient.Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin callback status = %d", denied.StatusCode)
	}
	req, err := http.NewRequest("POST", "http://"+callbackHost+"/complete", strings.NewReader(`{"token":"saved-browser-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://"+callbackHost)
	req.Header.Set("X-ALcli-Nonce", nonce)
	completed, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	completed.Body.Close()
	if completed.StatusCode != http.StatusNoContent {
		t.Fatalf("callback status = %d", completed.StatusCode)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessToken != "saved-browser-token" {
		t.Fatal("browser token was not saved")
	}
}

func TestStdioBinaryDiscovery(t *testing.T) {
	binary := os.Getenv("ALCLI_TEST_BIN")
	if binary == "" {
		t.Skip("set ALCLI_TEST_BIN to a built ALcli binary")
	}
	t.Setenv("ALCLI_CONFIG_DIR", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(binary, "mcp", "stdio")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range listed.Tools {
		if tool.Name == "auth_status" {
			found = true
		}
	}
	if !found {
		t.Fatal("auth_status missing from stdio MCP discovery")
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "auth_status", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("auth_status result=%#v err=%v", result, err)
	}
}

func TestHTTPTokenIsPrivateAndRejectsCorruption(t *testing.T) {
	t.Setenv("ALCLI_CONFIG_DIR", t.TempDir())
	token, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Fatalf("token length=%d", len(token))
	}
	path := filepath.Join(config.Dir(), "mcp-token")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token mode=%o", info.Mode().Perm())
	}
	second, err := Token()
	if err != nil || second != token {
		t.Fatalf("second token=%q err=%v", second, err)
	}
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Token(); err == nil {
		t.Fatal("accepted empty token file")
	}
}

func TestBrowserLoginExpiryIsReported(t *testing.T) {
	t.Setenv("ALCLI_OAUTH_CLIENT_ID", "")
	flow := newBrowserAuth()
	flow.address = "127.0.0.1:0"
	if _, err := flow.Start(); err != nil {
		t.Fatal(err)
	}
	flow.stop(flow.server)
	pending, completed, lastError := flow.Status()
	if pending || completed || lastError != "browser login expired" {
		t.Fatalf("status: pending=%t completed=%t error=%q", pending, completed, lastError)
	}
}
