package management

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestAuthRuntimeMaintenanceHook_RemovesAuthFileAfterUnauthorizedThreshold(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     3,
			UnauthorizedDeleteWindowSeconds: 600,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	result := coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  false,
		Error:    &coreauth.Error{HTTPStatus: http.StatusUnauthorized, Message: "unauthorized"},
	}
	for i := 0; i < 2; i++ {
		manager.MarkResult(context.Background(), result)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("auth file removed too early after %d failures: %v", i+1, err)
		}
	}

	manager.MarkResult(context.Background(), result)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected auth file to be removed, stat err: %v", err)
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after cleanup")
	}
	if updated.StatusMessage != "removed via management API" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "removed via management API")
	}
}

func TestAuthRuntimeMaintenanceHook_RemovesOnlyGeminiVirtualProjectOnUnauthorizedThreshold(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "gemini-auth.json")
	data := []byte(`{"type":"gemini","email":"user@example.com","project_id":"project-a,project-b"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     3,
			UnauthorizedDeleteWindowSeconds: 600,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	virtualID := managedGeminiVirtualID("gemini-auth.json", "project-a")
	result := coreauth.Result{
		AuthID:   virtualID,
		Provider: "gemini-cli",
		Success:  false,
		Error:    &coreauth.Error{HTTPStatus: http.StatusUnauthorized, Message: "unauthorized"},
	}
	for i := 0; i < 2; i++ {
		manager.MarkResult(context.Background(), result)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read auth file after %d failures: %v", i+1, err)
		}
		if string(raw) != string(data) {
			t.Fatalf("auth file changed too early after %d failures: %s", i+1, string(raw))
		}
	}

	manager.MarkResult(context.Background(), result)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth file after cleanup: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal updated auth file: %v", err)
	}
	if got, _ := metadata["project_id"].(string); got != "project-b" {
		t.Fatalf("project_id = %q, want %q", got, "project-b")
	}

	primary, ok := manager.GetByID("gemini-auth.json")
	if !ok || primary == nil {
		t.Fatalf("expected primary auth to exist")
	}
	if primary.Disabled {
		t.Fatalf("expected primary auth to remain enabled with one project left")
	}

	virtual, ok := manager.GetByID(virtualID)
	if !ok || virtual == nil {
		t.Fatalf("expected removed virtual auth to remain addressable as disabled record")
	}
	if !virtual.Disabled {
		t.Fatalf("expected removed virtual auth to be disabled")
	}
}

func TestAuthRuntimeMaintenanceHook_DisablesAuthFileOnCodexUsageLimitReached(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     3,
			UnauthorizedDeleteWindowSeconds: 600,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	manager.MarkResult(context.Background(), coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    `{"error":{"type":"usage_limit_reached","plan_type":"free","resets_in_seconds":60}}`,
		},
	})

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth file after disable: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal disabled auth file: %v", err)
	}
	if disabled, _ := metadata["disabled"].(bool); !disabled {
		t.Fatalf("expected auth file disabled=true, got %#v", metadata["disabled"])
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after usage_limit_reached")
	}
	if updated.StatusMessage != "disabled after usage_limit_reached" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "disabled after usage_limit_reached")
	}
}

func TestAuthRuntimeMaintenanceHook_DoesNotDisableAuthFileOnGenericCodex429(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     3,
			UnauthorizedDeleteWindowSeconds: 600,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	manager.MarkResult(context.Background(), coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    `{"detail":"Rate limit exceeded"}`,
		},
	})

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth file after generic 429: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal auth file after generic 429: %v", err)
	}
	if disabled, _ := metadata["disabled"].(bool); disabled {
		t.Fatalf("expected generic 429 not to disable auth file")
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if updated.Disabled {
		t.Fatalf("expected auth to remain enabled after generic 429")
	}
}

func managedGeminiVirtualID(baseID, projectID string) string {
	project := strings.TrimSpace(projectID)
	if project == "" {
		project = "project"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_")
	return baseID + "::" + replacer.Replace(project)
}
