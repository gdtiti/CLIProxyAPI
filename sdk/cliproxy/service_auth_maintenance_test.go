package cliproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type authMaintenanceTrackingStore struct {
	deleted []string
}

func (s *authMaintenanceTrackingStore) List(context.Context) ([]*coreauth.Auth, error) {
	return nil, nil
}

func (s *authMaintenanceTrackingStore) Save(_ context.Context, auth *coreauth.Auth) (string, error) {
	if auth == nil {
		return "", nil
	}
	return auth.ID, nil
}

func (s *authMaintenanceTrackingStore) Delete(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
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
		return ok && updated != nil && updated.Disabled && metadata["disabled"] == true
	})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("auth file should remain after disable: %v", err)
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

func TestAuthMaintenanceHook_RemovesAuthFileAfterUnauthorizedThreshold(t *testing.T) {
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
		_, err := os.Stat(path)
		return os.IsNotExist(err)
	})

	updated, ok := manager.GetByID(filepath.Base(path))
	if !ok || updated == nil {
		t.Fatalf("expected auth record to remain addressable")
	}
	if !updated.Disabled {
		t.Fatalf("expected auth to be disabled after file removal")
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
