package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestAuthRuntimeMaintenanceHook_DisablesAuthFileAfterUnauthorizedThreshold(t *testing.T) {
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
			t.Fatalf("auth file disabled too early after %d failures: %v", i+1, err)
		}
	}

	manager.MarkResult(context.Background(), result)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected auth file to remain on disk, read err: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal updated auth file: %v", err)
	}
	if metadata["disabled"] != true {
		t.Fatalf("expected auth file to be written with disabled=true, metadata=%v", metadata)
	}
	if metadata[coreauth.AuthMaintenanceAutoRecoveryMetadataKey] != true {
		t.Fatalf("expected auth file to carry auto-recovery marker, metadata=%v", metadata)
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after unauthorized threshold")
	}
	if updated.StatusMessage != "disabled after unauthorized threshold" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "disabled after unauthorized threshold")
	}
	if !coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected runtime auth to carry auto-recovery marker")
	}
}

func TestAuthRuntimeMaintenanceHook_DisablesCodexAuthFileAfterMaxRequestCount(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:               true,
			CodexMaxRequestCount: 2,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	result := coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  true,
	}

	manager.MarkResult(context.Background(), result)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("auth file disabled too early after first request: %v", err)
	}

	manager.MarkResult(context.Background(), result)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected auth file to remain on disk after reaching max request count, read err: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal updated auth file: %v", err)
	}
	if metadata["disabled"] != true {
		t.Fatalf("expected auth file to be written with disabled=true, metadata=%v", metadata)
	}
	if metadata[coreauth.AuthMaintenanceAutoRecoveryMetadataKey] != true {
		t.Fatalf("expected auth file to carry auto-recovery marker, metadata=%v", metadata)
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after max request threshold")
	}
	if updated.StatusMessage != "disabled after codex_max_request_count" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "disabled after codex_max_request_count")
	}
	if !coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected runtime auth to carry auto-recovery marker")
	}
}

func TestAuthRuntimeMaintenanceHook_DisablesCodexAuthFileAfterQuotaProbeUnauthorized(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	var probeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeCalls.Add(1)
		if r.URL.Path != "/backend-api/wham/usage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Chatgpt-Account-Id"); got != "acct-1" {
			t.Fatalf("Chatgpt-Account-Id = %q, want %q", got, "acct-1")
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                         true,
			CodexQuotaCheckRequestInterval: 2,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	auth, ok := manager.GetByID("codex-auth.json")
	if !ok || auth == nil {
		t.Fatalf("expected auth to exist after reload")
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["access_token"] = "test-token"
	auth.Metadata["account_id"] = "acct-1"
	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	auth.Attributes["base_url"] = server.URL + "/backend-api/codex"
	if _, err := manager.Update(context.Background(), auth); err != nil {
		t.Fatalf("manager.Update() error = %v", err)
	}

	result := coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  true,
	}

	manager.MarkResult(context.Background(), result)
	if got := probeCalls.Load(); got != 0 {
		t.Fatalf("probeCalls after first request = %d, want 0", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("auth file disabled too early after first request: %v", err)
	}

	manager.MarkResult(context.Background(), result)
	if got := probeCalls.Load(); got != 1 {
		t.Fatalf("probeCalls after second request = %d, want 1", got)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected auth file to remain on disk after quota probe 401, read err: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal updated auth file: %v", err)
	}
	if metadata["disabled"] != true {
		t.Fatalf("expected auth file to be written with disabled=true, metadata=%v", metadata)
	}
	if metadata[coreauth.AuthMaintenanceAutoRecoveryMetadataKey] != true {
		t.Fatalf("expected auth file to carry auto-recovery marker, metadata=%v", metadata)
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after quota probe 401")
	}
	if updated.StatusMessage != "disabled after codex_quota_probe_401" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "disabled after codex_quota_probe_401")
	}
	if !coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected runtime auth to carry auto-recovery marker")
	}
}

func TestAuthRuntimeMaintenanceHook_QuotaProbeIgnoresAuthFileProxyOverrideWhenConfigured(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	var usageCalls atomic.Int32
	usageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usageCalls.Add(1)
		if r.URL.Path != "/backend-api/wham/usage" {
			t.Fatalf("unexpected usage path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Chatgpt-Account-Id"); got != "acct-1" {
			t.Fatalf("Chatgpt-Account-Id = %q, want %q", got, "acct-1")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"allowed":true}}`))
	}))
	defer usageServer.Close()

	var proxyCalls atomic.Int32
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls.Add(1)
		http.Error(w, "proxy should be ignored", http.StatusBadGateway)
	}))
	defer proxyServer.Close()

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		SDKConfig: config.SDKConfig{
			IgnoreAuthFileProxyURL: true,
		},
		AuthDir: authDir,
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                         true,
			CodexQuotaCheckRequestInterval: 2,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	auth, ok := manager.GetByID("codex-auth.json")
	if !ok || auth == nil {
		t.Fatalf("expected auth to exist after reload")
	}
	auth.ProxyURL = proxyServer.URL
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["access_token"] = "test-token"
	auth.Metadata["account_id"] = "acct-1"
	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	auth.Attributes["base_url"] = usageServer.URL + "/backend-api/codex"
	if _, err := manager.Update(context.Background(), auth); err != nil {
		t.Fatalf("manager.Update() error = %v", err)
	}

	result := coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  true,
	}

	manager.MarkResult(context.Background(), result)
	manager.MarkResult(context.Background(), result)

	if got := usageCalls.Load(); got != 1 {
		t.Fatalf("usageCalls = %d, want 1", got)
	}
	if got := proxyCalls.Load(); got != 0 {
		t.Fatalf("proxyCalls = %d, want 0", got)
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain in manager")
	}
	if updated.Disabled {
		t.Fatalf("expected auth to remain enabled after successful quota probe")
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
	if metadata[coreauth.AuthMaintenanceAutoRecoveryMetadataKey] != true {
		t.Fatalf("expected auth file to carry auto-recovery marker, metadata=%v", metadata)
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
	if !coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected runtime auth to carry auto-recovery marker")
	}
}

func TestAuthRuntimeMaintenanceHook_DisablesAuthFileOnGenericCodex429(t *testing.T) {
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
	if disabled, _ := metadata["disabled"].(bool); !disabled {
		t.Fatalf("expected generic 429 to disable auth file")
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after generic 429")
	}
}

func TestAuthRuntimeMaintenanceHook_DisablesAuthFileOnConfiguredStatusCode(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:             true,
			DisableStatusCodes: []int{http.StatusTooManyRequests},
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
		t.Fatalf("read auth file after configured disable: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal auth file after configured disable: %v", err)
	}
	if disabled, _ := metadata["disabled"].(bool); !disabled {
		t.Fatalf("expected configured status code to disable auth file")
	}

	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after configured status code")
	}
	if updated.StatusMessage != "http_429" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "http_429")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("configured disable should not remove auth file: %v", err)
	}
}

func TestManagedProtectiveDisableReason_UsesQuotaStrikeReason(t *testing.T) {
	auth := &coreauth.Auth{
		Quota: coreauth.QuotaState{
			Exceeded:     true,
			BackoffLevel: 5,
		},
	}

	reason, ok := managedProtectiveDisableReason(auth, coreauth.Result{Success: false})
	if !ok {
		t.Fatalf("quota exceeded auth should produce a disable reason")
	}
	if reason != "quota_strikes_5" {
		t.Fatalf("reason = %q, want %q", reason, "quota_strikes_5")
	}
}

func TestAuthRuntimeMaintenanceHook_DoesNotRemoveCodexAuthFileWhenMaxRequestCountDisabled(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:               true,
			CodexMaxRequestCount: 0,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	result := coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  true,
	}
	for i := 0; i < 3; i++ {
		manager.MarkResult(context.Background(), result)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected auth file to remain when max request count disabled: %v", err)
	}
	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if updated.Disabled {
		t.Fatalf("expected auth to remain enabled when max request count disabled")
	}
}

func TestAuthRuntimeMaintenanceHook_DoesNotRemoveCodexAuthFileWhenQuotaProbeIsNotUnauthorized(t *testing.T) {
	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	data := []byte(`{"type":"codex","email":"user@example.com"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	var probeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rate_limit":{"allowed":true}}`))
	}))
	defer server.Close()

	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(&config.Config{
		AuthDir: authDir,
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                         true,
			CodexQuotaCheckRequestInterval: 2,
		},
	}, manager)
	handler.tokenStore = &memoryAuthStore{}

	if err := handler.reloadAuthFile(context.Background(), path, data); err != nil {
		t.Fatalf("reloadAuthFile() error = %v", err)
	}

	auth, ok := manager.GetByID("codex-auth.json")
	if !ok || auth == nil {
		t.Fatalf("expected auth to exist after reload")
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["access_token"] = "test-token"
	auth.Metadata["account_id"] = "acct-1"
	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	auth.Attributes["base_url"] = server.URL + "/backend-api/codex"
	if _, err := manager.Update(context.Background(), auth); err != nil {
		t.Fatalf("manager.Update() error = %v", err)
	}

	result := coreauth.Result{
		AuthID:   "codex-auth.json",
		Provider: "codex",
		Success:  true,
	}

	manager.MarkResult(context.Background(), result)
	manager.MarkResult(context.Background(), result)

	if got := probeCalls.Load(); got != 1 {
		t.Fatalf("probeCalls after two requests = %d, want 1", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected auth file to remain after non-401 quota probe: %v", err)
	}
	updated, ok := manager.GetByID("codex-auth.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain addressable in manager")
	}
	if updated.Disabled {
		t.Fatalf("expected auth to remain enabled after non-401 quota probe")
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
