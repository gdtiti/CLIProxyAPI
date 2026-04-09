package cliproxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type authMaintenanceTrackingStore struct {
	mu      sync.Mutex
	deleted []string
	paths   map[string]string
}

func (s *authMaintenanceTrackingStore) List(context.Context) ([]*coreauth.Auth, error) {
	return nil, nil
}

func (s *authMaintenanceTrackingStore) Save(_ context.Context, auth *coreauth.Auth) (string, error) {
	if auth == nil {
		return "", nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paths == nil {
		s.paths = make(map[string]string)
	}
	path := strings.TrimSpace(authMaintenanceTestAuthPath(auth))
	if path != "" {
		s.paths[auth.ID] = path
	}
	if path == "" || auth.Metadata == nil {
		return auth.ID, nil
	}
	auth.Metadata["disabled"] = auth.Disabled
	raw, err := json.Marshal(auth.Metadata)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return auth.ID, nil
}

func (s *authMaintenanceTrackingStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, id)
	path := strings.TrimSpace(id)
	if mapped, ok := s.paths[id]; ok && strings.TrimSpace(mapped) != "" {
		path = strings.TrimSpace(mapped)
		delete(s.paths, id)
	}
	if path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *authMaintenanceTrackingStore) SetBaseDir(string) {}

func TestAuthMaintenanceHook_DisablesAuthFileOnCodexUsageLimitReached(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceTestService(t, config.Config{
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     2,
			UnauthorizedDeleteWindowSeconds: 600,
		},
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                        true,
			ScanIntervalSeconds:           1,
			DeleteIntervalSeconds:         1,
			DisableCodexUsageLimitReached: true,
		},
	})
	defer cleanup()
	_ = service

	service.handleAuthMaintenanceResult(context.Background(), coreauth.Result{
		AuthID:   filepath.Base(path),
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    `{"error":{"type":"usage_limit_reached","resets_in_seconds":60}}`,
		},
	})

	waitForCondition(t, 3*time.Second, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var metadata map[string]any
		if json.Unmarshal(raw, &metadata) != nil {
			return false
		}
		updated, ok := manager.GetByID(filepath.Base(path))
		return ok && updated != nil && updated.Disabled && metadata["disabled"] == true && metadata[coreauth.AuthMaintenanceAutoRecoveryMetadataKey] == true
	})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("auth file should remain after disable: %v", err)
	}
	updated, ok := manager.GetByID(filepath.Base(path))
	if !ok || updated == nil {
		t.Fatalf("expected auth record to remain addressable")
	}
	if !coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected disabled codex auth to carry auto-recovery marker")
	}
}

func TestAuthMaintenanceHook_DisablesAuthFileOnGenericCodex429(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceTestService(t, config.Config{
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     2,
			UnauthorizedDeleteWindowSeconds: 600,
		},
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                        true,
			ScanIntervalSeconds:           1,
			DeleteIntervalSeconds:         1,
			DisableCodexUsageLimitReached: true,
		},
	})
	defer cleanup()
	_ = service

	service.handleAuthMaintenanceResult(context.Background(), coreauth.Result{
		AuthID:   filepath.Base(path),
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    `{"detail":"Rate limit exceeded"}`,
		},
	})

	waitForCondition(t, 3*time.Second, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var metadata map[string]any
		if json.Unmarshal(raw, &metadata) != nil {
			return false
		}
		updated, ok := manager.GetByID(filepath.Base(path))
		return ok && updated != nil && updated.Disabled && metadata["disabled"] == true
	})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("generic 429 should disable auth file instead of removing it: %v", err)
	}
}

func TestAuthMaintenanceHook_DisablesAuthFileOnConfiguredStatusCode(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceTestService(t, config.Config{
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     2,
			UnauthorizedDeleteWindowSeconds: 600,
		},
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                true,
			ScanIntervalSeconds:   1,
			DeleteIntervalSeconds: 1,
			DisableStatusCodes:    []int{http.StatusTooManyRequests},
		},
	})
	defer cleanup()
	_ = service

	service.handleAuthMaintenanceResult(context.Background(), coreauth.Result{
		AuthID:   filepath.Base(path),
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    `{"detail":"Rate limit exceeded"}`,
		},
	})

	waitForCondition(t, 3*time.Second, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var metadata map[string]any
		if json.Unmarshal(raw, &metadata) != nil {
			return false
		}
		updated, ok := manager.GetByID(filepath.Base(path))
		return ok && updated != nil && updated.Disabled && metadata["disabled"] == true
	})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("configured disable should not remove auth file: %v", err)
	}
}

func TestAuthMaintenanceHook_DisableStatusCodeWinsOverDeleteStatusCode(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceTestService(t, config.Config{
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     2,
			UnauthorizedDeleteWindowSeconds: 600,
		},
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                true,
			ScanIntervalSeconds:   1,
			DeleteIntervalSeconds: 1,
			DeleteStatusCodes:     []int{http.StatusTooManyRequests},
			DisableStatusCodes:    []int{http.StatusTooManyRequests},
		},
	})
	defer cleanup()
	_ = service

	service.handleAuthMaintenanceResult(context.Background(), coreauth.Result{
		AuthID:   filepath.Base(path),
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    `{"detail":"Rate limit exceeded"}`,
		},
	})

	waitForCondition(t, 3*time.Second, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var metadata map[string]any
		if json.Unmarshal(raw, &metadata) != nil {
			return false
		}
		updated, ok := manager.GetByID(filepath.Base(path))
		return ok && updated != nil && updated.Disabled && metadata["disabled"] == true
	})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("disable should win over delete for overlapping status codes: %v", err)
	}
}

func TestAuthMaintenanceHook_DeleteStatusCode429StillDisablesAuth(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceTestService(t, config.Config{
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     2,
			UnauthorizedDeleteWindowSeconds: 600,
		},
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                true,
			ScanIntervalSeconds:   1,
			DeleteIntervalSeconds: 1,
			DeleteStatusCodes:     []int{http.StatusTooManyRequests},
		},
	})
	defer cleanup()
	_ = service

	service.handleAuthMaintenanceResult(context.Background(), coreauth.Result{
		AuthID:   filepath.Base(path),
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    `{"detail":"Rate limit exceeded"}`,
		},
	})

	waitForCondition(t, 3*time.Second, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var metadata map[string]any
		if json.Unmarshal(raw, &metadata) != nil {
			return false
		}
		updated, ok := manager.GetByID(filepath.Base(path))
		return ok && updated != nil && updated.Disabled && metadata["disabled"] == true
	})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("delete-status-codes=[429] should still disable instead of deleting: %v", err)
	}
}

func TestAuthEligibleForMaintenanceDelete_DoesNotDeleteQuotaExceededAuth(t *testing.T) {
	auth := &coreauth.Auth{
		Quota: coreauth.QuotaState{
			Exceeded:     true,
			BackoffLevel: 7,
		},
	}
	cfg := config.AuthMaintenanceConfig{
		DeleteQuotaExceeded:  true,
		QuotaStrikeThreshold: 6,
	}

	reason, ok := authEligibleForMaintenanceDisable(auth, nil, cfg)
	if !ok {
		t.Fatalf("quota exceeded auth should now be disabled instead of deleted")
	}
	if reason != "quota_strikes_7" {
		t.Fatalf("disable reason = %q, want %q", reason, "quota_strikes_7")
	}

	if deleteReason, deleteOK := authEligibleForMaintenanceDelete(auth, nil, cfg); deleteOK {
		t.Fatalf("quota exceeded auth should not be deleted, got delete reason %q", deleteReason)
	}
}

func TestAuthMaintenanceHook_DisablesAuthFileAfterUnauthorizedThreshold(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceTestService(t, config.Config{
		AuthRuntime: config.AuthRuntimeConfig{
			UnauthorizedDeleteThreshold:     2,
			UnauthorizedDeleteWindowSeconds: 600,
		},
		AuthMaintenance: config.AuthMaintenanceConfig{
			Enable:                        true,
			ScanIntervalSeconds:           1,
			DeleteIntervalSeconds:         1,
			DisableCodexUsageLimitReached: true,
		},
	})
	defer cleanup()
	_ = service

	result := coreauth.Result{
		AuthID:   filepath.Base(path),
		Provider: "codex",
		Success:  false,
		Error: &coreauth.Error{
			HTTPStatus: http.StatusUnauthorized,
			Message:    "unauthorized",
		},
	}
	service.handleAuthMaintenanceResult(context.Background(), result)
	service.handleAuthMaintenanceResult(context.Background(), result)

	waitForCondition(t, 3*time.Second, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		var metadata map[string]any
		if json.Unmarshal(raw, &metadata) != nil {
			return false
		}
		updated, ok := manager.GetByID(filepath.Base(path))
		return ok && updated != nil && updated.Disabled && updated.StatusMessage == "disabled after unauthorized threshold" && metadata["disabled"] == true
	})

	updated, ok := manager.GetByID(filepath.Base(path))
	if !ok || updated == nil {
		t.Fatalf("expected auth record to remain addressable")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after unauthorized threshold")
	}
	if updated.StatusMessage != "disabled after unauthorized threshold" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "disabled after unauthorized threshold")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("auth file should remain after automatic disable: %v", err)
	}
}

func TestSnapshotAuthMaintenanceConfig_PreservesCodexMaxRequestCount(t *testing.T) {
	service := &Service{
		cfg: &config.Config{
			AuthDir: "J:/tmp/auths",
			AuthMaintenance: config.AuthMaintenanceConfig{
				CodexMaxRequestCount: 9,
			},
		},
	}

	maintenance, authDir := service.snapshotAuthMaintenanceConfig()

	if maintenance.CodexMaxRequestCount != 9 {
		t.Fatalf("CodexMaxRequestCount = %d, want 9", maintenance.CodexMaxRequestCount)
	}
	if authDir != "J:/tmp/auths" {
		t.Fatalf("authDir = %q, want %q", authDir, "J:/tmp/auths")
	}
}

func TestSnapshotAuthMaintenanceConfig_PreservesCodexQuotaCheckRequestInterval(t *testing.T) {
	service := &Service{
		cfg: &config.Config{
			AuthDir: "J:/tmp/auths",
			AuthMaintenance: config.AuthMaintenanceConfig{
				CodexQuotaCheckRequestInterval: 6,
			},
		},
	}

	maintenance, authDir := service.snapshotAuthMaintenanceConfig()

	if maintenance.CodexQuotaCheckRequestInterval != 6 {
		t.Fatalf("CodexQuotaCheckRequestInterval = %d, want 6", maintenance.CodexQuotaCheckRequestInterval)
	}
	if authDir != "J:/tmp/auths" {
		t.Fatalf("authDir = %q, want %q", authDir, "J:/tmp/auths")
	}
}

type authMaintenanceRecoveryTestExecutor struct {
	refreshFn func(context.Context, *coreauth.Auth) (*coreauth.Auth, error)
	httpFn    func(context.Context, *coreauth.Auth, *http.Request) (*http.Response, error)
}

func (e *authMaintenanceRecoveryTestExecutor) Identifier() string {
	return "codex"
}

func (e *authMaintenanceRecoveryTestExecutor) Execute(context.Context, *coreauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *authMaintenanceRecoveryTestExecutor) ExecuteStream(context.Context, *coreauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, nil
}

func (e *authMaintenanceRecoveryTestExecutor) Refresh(ctx context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
	if e != nil && e.refreshFn != nil {
		return e.refreshFn(ctx, auth)
	}
	return auth, nil
}

func (e *authMaintenanceRecoveryTestExecutor) CountTokens(context.Context, *coreauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *authMaintenanceRecoveryTestExecutor) HttpRequest(ctx context.Context, auth *coreauth.Auth, req *http.Request) (*http.Response, error) {
	if e != nil && e.httpFn != nil {
		return e.httpFn(ctx, auth, req)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
}

func TestAuthMaintenanceRecoveryCandidateForAuth_RequiresSystemMarker(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceRecoveryTestService(t, map[string]any{
		"type":       "codex",
		"email":      "user@example.com",
		"disabled":   true,
		"api_key":    "token-1",
		"account_id": "acct-1",
	})
	defer cleanup()

	auth, ok := manager.GetByID(filepath.Base(path))
	if !ok || auth == nil {
		t.Fatalf("expected auth to exist")
	}
	if candidate, ok := service.authMaintenanceRecoveryCandidateForAuth(auth, filepath.Dir(path), time.Now()); ok {
		t.Fatalf("expected manual disabled auth to be skipped, got %+v", candidate)
	}
}

func TestRecoverAuthMaintenanceCandidate_RefreshesAndUnlocksAuth(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceRecoveryTestService(t, map[string]any{
		"type":          "codex",
		"email":         "user@example.com",
		"disabled":      true,
		"refresh_token": "refresh-token",
		"access_token":  "expired-token",
		"account_id":    "acct-1",
		"expired":       time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339Nano),
		coreauth.AuthMaintenanceAutoRecoveryMetadataKey:           true,
		coreauth.AuthMaintenanceAutoRecoveryReasonMetadataKey:     "disabled after usage_limit_reached",
		coreauth.AuthMaintenanceAutoRecoveryDisabledAtMetadataKey: time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339Nano),
	})
	defer cleanup()

	manager.RegisterExecutor(&authMaintenanceRecoveryTestExecutor{
		refreshFn: func(_ context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
			refreshed := auth.Clone()
			if refreshed.Metadata == nil {
				refreshed.Metadata = make(map[string]any)
			}
			refreshed.Metadata["access_token"] = "fresh-token"
			refreshed.Metadata["account_id"] = "acct-1"
			refreshed.Metadata["expired"] = time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano)
			return refreshed, nil
		},
		httpFn: func(_ context.Context, auth *coreauth.Auth, req *http.Request) (*http.Response, error) {
			if auth == nil {
				t.Fatalf("expected auth in probe")
			}
			if got := req.Header.Get("Chatgpt-Account-Id"); got != "acct-1" {
				t.Fatalf("Chatgpt-Account-Id = %q, want %q", got, "acct-1")
			}
			if got := authMaintenanceTokenValue(auth); got != "fresh-token" {
				t.Fatalf("access token = %q, want %q", got, "fresh-token")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"rate_limit":{"allowed":true}}`)),
			}, nil
		},
	})

	auth, ok := manager.GetByID(filepath.Base(path))
	if !ok || auth == nil {
		t.Fatalf("expected auth to exist")
	}
	candidate, ok := service.authMaintenanceRecoveryCandidateForAuth(auth, filepath.Dir(path), time.Now())
	if !ok {
		t.Fatalf("expected recovery candidate")
	}
	if err := service.recoverAuthMaintenanceCandidate(context.Background(), candidate); err != nil {
		t.Fatalf("recoverAuthMaintenanceCandidate() error = %v", err)
	}

	updated, ok := manager.GetByID(filepath.Base(path))
	if !ok || updated == nil {
		t.Fatalf("expected updated auth")
	}
	if updated.Disabled {
		t.Fatalf("expected auth to be unlocked")
	}
	if updated.Status != coreauth.StatusActive {
		t.Fatalf("Status = %q, want %q", updated.Status, coreauth.StatusActive)
	}
	if coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected auto-recovery marker to be cleared after unlock")
	}
	if updated.Metadata["access_token"] != "fresh-token" {
		t.Fatalf("expected refreshed access token to persist")
	}

	persisted := readAuthMaintenanceTestMetadata(t, path)
	if disabled, _ := persisted["disabled"].(bool); disabled {
		t.Fatalf("expected auth file disabled=false after unlock, metadata=%v", persisted)
	}
	if _, ok := persisted[coreauth.AuthMaintenanceAutoRecoveryMetadataKey]; ok {
		t.Fatalf("expected auth file marker cleared after unlock, metadata=%v", persisted)
	}

	records := service.authMaintenanceRecoveryRecentSnapshot()
	if !authMaintenanceRecentContains(records, "refresh") || !authMaintenanceRecentContains(records, "unlock") {
		t.Fatalf("expected refresh and unlock events in recent log, got %+v", records)
	}
	logPath := service.authMaintenanceRecoveryLogPath()
	rawLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read recovery log: %v", err)
	}
	if !strings.Contains(string(rawLog), `"event":"unlock"`) {
		t.Fatalf("expected unlock event in recovery log: %s", string(rawLog))
	}
}

func TestRecoverAuthMaintenanceCandidate_KeepsDisabledWhenQuotaNotRecovered(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceRecoveryTestService(t, map[string]any{
		"type":         "codex",
		"email":        "user@example.com",
		"disabled":     true,
		"access_token": "live-token",
		"account_id":   "acct-1",
		"expired":      time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano),
		coreauth.AuthMaintenanceAutoRecoveryMetadataKey:           true,
		coreauth.AuthMaintenanceAutoRecoveryReasonMetadataKey:     "disabled after usage_limit_reached",
		coreauth.AuthMaintenanceAutoRecoveryDisabledAtMetadataKey: time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339Nano),
	})
	defer cleanup()

	manager.RegisterExecutor(&authMaintenanceRecoveryTestExecutor{
		httpFn: func(_ context.Context, _ *coreauth.Auth, _ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body: io.NopCloser(strings.NewReader(`{
					"rate_limit": {
						"allowed": false,
						"limit_reached": true,
						"primary_window": {
							"limit_window_seconds": 18000,
							"used_percent": 100,
							"reset_after_seconds": 300
						}
					}
				}`)),
			}, nil
		},
	})

	auth, ok := manager.GetByID(filepath.Base(path))
	if !ok || auth == nil {
		t.Fatalf("expected auth to exist")
	}
	candidate, ok := service.authMaintenanceRecoveryCandidateForAuth(auth, filepath.Dir(path), time.Now())
	if !ok {
		t.Fatalf("expected recovery candidate")
	}
	if err := service.recoverAuthMaintenanceCandidate(context.Background(), candidate); err != nil {
		t.Fatalf("recoverAuthMaintenanceCandidate() error = %v", err)
	}

	updated, ok := manager.GetByID(filepath.Base(path))
	if !ok || updated == nil {
		t.Fatalf("expected updated auth")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to remain disabled")
	}
	if !coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected marker to remain while quota still limited")
	}
	if !updated.Quota.Exceeded {
		t.Fatalf("expected quota exceeded state to be retained")
	}
	if updated.Quota.Reason != "five_hour" {
		t.Fatalf("Quota.Reason = %q, want %q", updated.Quota.Reason, "five_hour")
	}
	if updated.Quota.NextRecoverAt.IsZero() {
		t.Fatalf("expected NextRecoverAt to be set")
	}
	nextCheckAt, ok := coreauth.AuthMaintenanceAutoRecoveryNextCheckAt(updated)
	if !ok || nextCheckAt.IsZero() {
		t.Fatalf("expected next recovery check timestamp")
	}

	persisted := readAuthMaintenanceTestMetadata(t, path)
	if disabled, _ := persisted["disabled"].(bool); !disabled {
		t.Fatalf("expected auth file to stay disabled, metadata=%v", persisted)
	}
	if _, ok := persisted[coreauth.AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey]; !ok {
		t.Fatalf("expected next check metadata to persist, metadata=%v", persisted)
	}

	records := service.authMaintenanceRecoveryRecentSnapshot()
	if !authMaintenanceRecentContains(records, "probe") || !authMaintenanceRecentContains(records, "keep_disabled") {
		t.Fatalf("expected probe and keep_disabled events, got %+v", records)
	}
}

func TestRecoverAuthMaintenanceCandidate_ClearsMarkerOnRefreshTokenReused(t *testing.T) {
	service, manager, path, cleanup := newAuthMaintenanceRecoveryTestService(t, map[string]any{
		"type":          "codex",
		"email":         "user@example.com",
		"disabled":      true,
		"refresh_token": "refresh-token",
		"access_token":  "expired-token",
		"account_id":    "acct-1",
		"expired":       time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339Nano),
		coreauth.AuthMaintenanceAutoRecoveryMetadataKey:           true,
		coreauth.AuthMaintenanceAutoRecoveryReasonMetadataKey:     "disabled after usage_limit_reached",
		coreauth.AuthMaintenanceAutoRecoveryDisabledAtMetadataKey: time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339Nano),
	})
	defer cleanup()

	manager.RegisterExecutor(&authMaintenanceRecoveryTestExecutor{
		refreshFn: func(_ context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
			refreshed := auth.Clone()
			if refreshed.Metadata == nil {
				refreshed.Metadata = make(map[string]any)
			}
			refreshed.Disabled = true
			refreshed.Status = coreauth.StatusDisabled
			refreshed.Metadata["refresh_disabled_reason"] = "refresh_token_reused"
			return refreshed, nil
		},
	})

	auth, ok := manager.GetByID(filepath.Base(path))
	if !ok || auth == nil {
		t.Fatalf("expected auth to exist")
	}
	candidate, ok := service.authMaintenanceRecoveryCandidateForAuth(auth, filepath.Dir(path), time.Now())
	if !ok {
		t.Fatalf("expected recovery candidate")
	}
	if err := service.recoverAuthMaintenanceCandidate(context.Background(), candidate); err != nil {
		t.Fatalf("recoverAuthMaintenanceCandidate() error = %v", err)
	}

	updated, ok := manager.GetByID(filepath.Base(path))
	if !ok || updated == nil {
		t.Fatalf("expected updated auth")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to remain disabled")
	}
	if coreauth.IsAuthMaintenanceAutoRecoverable(updated) {
		t.Fatalf("expected marker cleared after terminal refresh failure")
	}
	if updated.StatusMessage != "refresh token reused" {
		t.Fatalf("StatusMessage = %q, want %q", updated.StatusMessage, "refresh token reused")
	}

	persisted := readAuthMaintenanceTestMetadata(t, path)
	if _, ok := persisted[coreauth.AuthMaintenanceAutoRecoveryMetadataKey]; ok {
		t.Fatalf("expected auth file marker cleared after terminal disable, metadata=%v", persisted)
	}
	if disabled, _ := persisted["disabled"].(bool); !disabled {
		t.Fatalf("expected auth file to stay disabled, metadata=%v", persisted)
	}

	records := service.authMaintenanceRecoveryRecentSnapshot()
	if !authMaintenanceRecentContains(records, "terminal_disable") {
		t.Fatalf("expected terminal_disable event, got %+v", records)
	}
}

func TestEnqueueAuthMaintenanceCandidate_RejectsInFlightDuplicate(t *testing.T) {
	service := &Service{}
	candidate := authMaintenanceCandidate{
		Key:    authMaintenanceCandidateKey(authMaintenanceActionDelete, "C:/tmp/test.json"),
		Path:   "C:/tmp/test.json",
		IDs:    []string{"auth-1"},
		Reason: "http_401",
		Action: authMaintenanceActionDelete,
	}

	if !service.enqueueAuthMaintenanceCandidate(candidate) {
		t.Fatal("expected initial enqueue to succeed")
	}
	popped, _, ok := service.popAuthMaintenanceCandidate()
	if !ok {
		t.Fatal("expected candidate to move into in-flight state")
	}
	if service.enqueueAuthMaintenanceCandidate(candidate) {
		t.Fatal("expected duplicate enqueue to be rejected while in flight")
	}
	service.finishAuthMaintenanceCandidate(popped)
	if !service.enqueueAuthMaintenanceCandidate(candidate) {
		t.Fatal("expected enqueue to succeed again after in-flight candidate finished")
	}
}

func newAuthMaintenanceTestService(t *testing.T, cfg config.Config) (*Service, *coreauth.Manager, string, func()) {
	t.Helper()

	authDir := t.TempDir()
	path := filepath.Join(authDir, "codex-auth.json")
	if err := os.WriteFile(path, []byte(`{"type":"codex","email":"user@example.com"}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	prevStore := sdkAuth.GetTokenStore()
	store := &authMaintenanceTrackingStore{}
	sdkAuth.RegisterTokenStore(store)

	manager := coreauth.NewManager(store, nil, nil)
	service := &Service{
		cfg: &config.Config{
			AuthDir:         authDir,
			AuthRuntime:     cfg.AuthRuntime,
			AuthMaintenance: cfg.AuthMaintenance,
		},
		coreManager: manager,
	}

	auth := &coreauth.Auth{
		ID:         filepath.Base(path),
		Provider:   "codex",
		FileName:   filepath.Base(path),
		Attributes: map[string]string{"path": path},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	service.startAuthMaintenance(ctx)

	cleanup := func() {
		cancel()
		if service.maintenanceCancel != nil {
			service.maintenanceCancel()
		}
		sdkAuth.RegisterTokenStore(prevStore)
	}

	return service, manager, path, cleanup
}

func newAuthMaintenanceRecoveryTestService(t *testing.T, metadata map[string]any) (*Service, *coreauth.Manager, string, func()) {
	t.Helper()

	authDir := t.TempDir()
	t.Setenv("WRITABLE_PATH", t.TempDir())
	path := filepath.Join(authDir, "codex-auth.json")
	if metadata == nil {
		metadata = map[string]any{
			"type":  "codex",
			"email": "user@example.com",
		}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal auth file: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	prevStore := sdkAuth.GetTokenStore()
	store := &authMaintenanceTrackingStore{}
	sdkAuth.RegisterTokenStore(store)

	manager := coreauth.NewManager(store, nil, nil)
	service := &Service{
		cfg: &config.Config{
			AuthDir: authDir,
			AuthMaintenance: config.AuthMaintenanceConfig{
				Enable: true,
			},
		},
		coreManager: manager,
	}

	auth := &coreauth.Auth{
		ID:         filepath.Base(path),
		Provider:   "codex",
		FileName:   filepath.Base(path),
		Attributes: map[string]string{"path": path},
		Metadata:   cloneAuthMaintenanceMetadata(metadata),
	}
	if disabled, _ := metadata["disabled"].(bool); disabled {
		auth.Disabled = true
		auth.Status = coreauth.StatusDisabled
		auth.StatusMessage = maintenanceStatusMessage(coreauth.AuthMaintenanceAutoRecoveryReason(auth), "disabled via auth maintenance")
	} else {
		auth.Status = coreauth.StatusActive
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	cleanup := func() {
		sdkAuth.RegisterTokenStore(prevStore)
	}
	return service, manager, path, cleanup
}

func cloneAuthMaintenanceMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		panic(err)
	}
	var cloned map[string]any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		panic(err)
	}
	return cloned
}

func readAuthMaintenanceTestMetadata(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth file: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal auth file: %v", err)
	}
	return metadata
}

func authMaintenanceRecentContains(records []authMaintenanceRecoveryLogRecord, event string) bool {
	for _, record := range records {
		if record.Event == event {
			return true
		}
	}
	return false
}

func authMaintenanceTestAuthPath(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if path := strings.TrimSpace(auth.Attributes["path"]); path != "" {
			return path
		}
	}
	if fileName := strings.TrimSpace(auth.FileName); fileName != "" {
		return fileName
	}
	return strings.TrimSpace(auth.ID)
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}
