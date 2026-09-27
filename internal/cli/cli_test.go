package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/noyukii/ALcli/internal/config"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/noyukii/ALcli/internal/api"
)

func setupConfig(t *testing.T) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("ALCLI_CONFIG_DIR", configDir)
}

func TestExecuteNoArgsRequestsTUIAndPreservesImageOptions(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	err := Execute([]string{"--images=kitty", "--no-images"}, &out, &errOut)
	var request TUIRequest
	if !errors.As(err, &request) {
		t.Fatalf("Execute() error = %v, want TUIRequest", err)
	}
	if request.Images != "kitty" || !request.NoImages {
		t.Fatalf("request = %#v, want kitty with no-images", request)
	}
}

func TestHelpCommandShowsRootHelp(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"help"}, &out, &errOut); err != nil {
		t.Fatalf("Execute(help): %v", err)
	}
	for _, want := range []string{"Usage:", "media", "profile", "list", "auth"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help output missing %q:\n%s", want, out.String())
		}
	}
}

func TestExecuteRejectsInvalidOutputFormat(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	err := Execute([]string{"--output=yaml", "profile"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("Execute() error = %v, want unsupported output format", err)
	}
}

func TestExecuteValidatesMediaIDBeforeAuthentication(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	err := Execute([]string{"media", "get", "abc"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("Execute() error = %v, want media ID validation", err)
	}
}

func TestExecuteRequiresAuthenticationForReadCommand(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	err := Execute([]string{"profile"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("Execute() error = %v, want authentication guidance", err)
	}
}

func TestJSONUsesLowerCamelCase(t *testing.T) {
	var out bytes.Buffer
	state := commandState{out: &out, format: "json"}
	if err := state.emit(api.Media{ID: 12, TitleRomaji: "Example", TitleEnglish: stringPtr("Example EN")}); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(out.Bytes(), &value); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if value["id"] != float64(12) || value["titleRomaji"] != "Example" || value["titleEnglish"] != "Example EN" {
		t.Fatalf("JSON output = %s", out.String())
	}
	if _, ok := value["ID"]; ok {
		t.Fatalf("JSON output has capitalized field: %s", out.String())
	}
}

func TestAuthStatusWorksWithoutLogin(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"auth", "status"}, &out, &errOut); err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(out.String(), "Not logged in") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestJSONAuthStatusIsStructured(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--output", "json", "auth", "status"}, &out, &errOut); err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if strings.TrimSpace(out.String()) != "{\n  \"authenticated\": false\n}" {
		t.Fatalf("JSON output = %q", out.String())
	}
}

func TestPagingValidation(t *testing.T) {
	for _, tc := range []struct {
		page, size int
		wantErr    bool
	}{
		{1, 20, false}, {0, 20, true}, {1, 0, true}, {1, 51, true},
	} {
		err := validatePaging(tc.page, tc.size)
		if (err != nil) != tc.wantErr {
			t.Errorf("validatePaging(%d, %d) error = %v", tc.page, tc.size, err)
		}
	}
}

func TestDeleteConfirmation(t *testing.T) {
	if err := confirmDelete(strings.NewReader(""), io.Discard, 1, false, false); err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("non-interactive confirmation error = %v", err)
	}
	if err := confirmDelete(strings.NewReader(""), io.Discard, 1, true, false); err != nil {
		t.Fatalf("--yes confirmation: %v", err)
	}
	if err := confirmDelete(strings.NewReader("no\n"), io.Discard, 1, false, true); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("negative confirmation error = %v", err)
	}
}

func stringPtr(s string) *string { return &s }

func TestConfigFlagStillPrintsPath(t *testing.T) {
	setupConfig(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config"}, &out, &errOut); err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if !strings.Contains(out.String(), "config.json") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLogoutCommandRemovesSavedConfig(t *testing.T) {
	setupConfig(t)
	configFile := os.Getenv("ALCLI_CONFIG_DIR") + "/config.json"
	if err := os.WriteFile(configFile, []byte(`{"access_token":"token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := Execute([]string{"auth", "logout"}, &out, &errOut); err != nil {
		t.Fatalf("Execute(): %v", err)
	}
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		t.Fatalf("config still exists; stat error = %v", err)
	}
}

func TestGraphQLAndNamedActionsAgainstMock(t *testing.T) {
	setupConfig(t)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !strings.Contains(body.Query, "Viewer") && !strings.Contains(body.Query, "ToggleFollow") {
			t.Errorf("query = %q", body.Query)
		}
		_, _ = io.WriteString(w, `{"data":{"Viewer":{"id":7},"ToggleFollow":{"id":8}}}`)
	}))
	defer endpoint.Close()
	original := newAPIClient
	newAPIClient = func(token string) *api.Client { return api.NewClientAt(token, endpoint.URL) }
	defer func() { newAPIClient = original }()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AccessToken = "mock-token"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"graphql", "query", "--document", "query { Viewer { id } }"},
		{"action", "follows_toggle", "--variables", `{"userId":8}`},
	} {
		var out, errOut bytes.Buffer
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out.String(), "7") && !strings.Contains(out.String(), "8") {
			t.Errorf("output=%s", out.String())
		}
	}
}
