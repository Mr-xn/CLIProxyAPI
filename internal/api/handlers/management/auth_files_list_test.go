package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestListAuthFiles_NoFilterReturnsAllEntriesSorted(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "beta.json",
		provider: "claude",
		email:    "beta@example.com",
		note:     "ops",
	})
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "alpha.json",
		provider: "codex",
		email:    "alpha@example.com",
		note:     "starter",
	})
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "gamma.json",
		provider: "codex",
		email:    "gamma@example.com",
		note:     "plus",
	})

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	names := listAuthFileNames(t, h, "/v0/management/auth-files")
	expected := []string{"alpha.json", "beta.json", "gamma.json"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d auth files, got %d: %#v", len(expected), len(names), names)
	}
	for i, want := range expected {
		if names[i] != want {
			t.Fatalf("expected auth file %d to be %q, got %q", i, want, names[i])
		}
	}
}

func TestListAuthFiles_FilterByQuery(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "alpha.json",
		provider: "codex",
		email:    "alpha@example.com",
		note:     "starter",
	})
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "gamma.json",
		provider: "codex",
		email:    "gamma@example.com",
		note:     "plus-plan",
	})

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	names := listAuthFileNames(t, h, "/v0/management/auth-files?q=plus-plan")
	if len(names) != 1 || names[0] != "gamma.json" {
		t.Fatalf("expected filtered auth files [gamma.json], got %#v", names)
	}
}

func TestListAuthFiles_FilterByMultipleConditions(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "alpha.json",
		provider: "codex",
		email:    "alpha@example.com",
		note:     "ops",
	})
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "beta.json",
		provider: "claude",
		email:    "beta@example.com",
		note:     "ops",
		disabled: true,
	})
	registerListAuthTestFile(t, manager, authDir, authListSeed{
		name:     "gamma.json",
		provider: "claude",
		email:    "gamma@example.com",
		note:     "support",
		disabled: true,
	})

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	names := listAuthFileNames(t, h, "/v0/management/auth-files?provider=CLAUDE&disabled=true&q=ops")
	if len(names) != 1 || names[0] != "beta.json" {
		t.Fatalf("expected filtered auth files [beta.json], got %#v", names)
	}
}

func TestListAuthFilesFromDisk_Filtered(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	writeListAuthJSONFile(t, authDir, authListSeed{
		name:     "alpha.json",
		provider: "codex",
		email:    "alpha@example.com",
		note:     "starter",
	})
	writeListAuthJSONFile(t, authDir, authListSeed{
		name:     "beta.json",
		provider: "claude",
		email:    "beta@example.com",
		note:     "ops",
	})

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, nil)
	names := listAuthFileNames(t, h, "/v0/management/auth-files?type=claude&q=ops")
	if len(names) != 1 || names[0] != "beta.json" {
		t.Fatalf("expected disk-filtered auth files [beta.json], got %#v", names)
	}
}

type authListSeed struct {
	name     string
	provider string
	email    string
	note     string
	disabled bool
}

func registerListAuthTestFile(t *testing.T, manager *coreauth.Manager, authDir string, seed authListSeed) {
	t.Helper()

	path := writeListAuthJSONFile(t, authDir, seed)
	record := &coreauth.Auth{
		ID:       "auth/" + seed.name,
		FileName: seed.name,
		Provider: seed.provider,
		Status:   coreauth.StatusActive,
		Disabled: seed.disabled,
		Attributes: map[string]string{
			"path": path,
		},
		Metadata: map[string]any{
			"email": seed.email,
			"note":  seed.note,
		},
	}
	if seed.disabled {
		record.Status = coreauth.StatusDisabled
	}
	if _, err := manager.Register(context.Background(), record); err != nil {
		t.Fatalf("failed to register auth %s: %v", seed.name, err)
	}
}

func writeListAuthJSONFile(t *testing.T, authDir string, seed authListSeed) string {
	t.Helper()

	path := filepath.Join(authDir, seed.name)
	content := map[string]any{
		"type":  seed.provider,
		"email": seed.email,
	}
	if seed.note != "" {
		content["note"] = seed.note
	}
	if seed.disabled {
		content["disabled"] = true
	}
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal auth JSON for %s: %v", seed.name, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write auth file %s: %v", seed.name, err)
	}
	return path
}

func listAuthFileNames(t *testing.T, h *Handler, requestPath string) []string {
	t.Helper()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, requestPath, nil)

	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode list payload: %v", err)
	}

	names := make([]string, 0, len(payload.Files))
	for _, file := range payload.Files {
		name, _ := file["name"].(string)
		names = append(names, name)
	}
	return names
}
