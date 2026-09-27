package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
