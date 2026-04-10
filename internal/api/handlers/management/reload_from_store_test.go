package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	storepkg "github.com/router-for-me/CLIProxyAPI/v6/internal/store"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type plainStore struct{}

func (plainStore) List(context.Context) ([]*coreauth.Auth, error)       { return nil, nil }
func (plainStore) Save(context.Context, *coreauth.Auth) (string, error) { return "", nil }
func (plainStore) Delete(context.Context, string) error                 { return nil }

type reloadableConfigStore struct {
	plainStore
	result *storepkg.ConfigReloadResult
	err    error
}

func (s reloadableConfigStore) ReloadConfigFromStore(context.Context) (*storepkg.ConfigReloadResult, error) {
	return s.result, s.err
}

type reloadableAuthStore struct {
	plainStore
	result *storepkg.AuthReloadResult
	err    error
}

func (s reloadableAuthStore) ReloadAuthFilesFromStore(context.Context) (*storepkg.AuthReloadResult, error) {
	return s.result, s.err
}

func TestPostReloadConfigFromStoreUnsupported(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithoutConfigFilePath(nil, nil)
	handler.tokenStore = plainStore{}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/config.yaml/reload-from-store", nil)

	handler.PostReloadConfigFromStore(ctx)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, recorder.Code)
	}
}

func TestPostReloadConfigFromStoreSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithoutConfigFilePath(nil, nil)
	handler.tokenStore = reloadableConfigStore{
		result: &storepkg.ConfigReloadResult{
			Store:   "postgres",
			Changed: true,
		},
	}

	reloadCalled := false
	handler.SetRuntimeReloadHooks(RuntimeReloadHooks{
		ReloadConfigFromDisk: func() error {
			reloadCalled = true
			return nil
		},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/config.yaml/reload-from-store", nil)

	handler.PostReloadConfigFromStore(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !reloadCalled {
		t.Fatalf("expected runtime config reload hook to be called")
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["target"] != "config" {
		t.Fatalf("expected target=config, got %#v", payload["target"])
	}
}

func TestPostReloadAuthFilesFromStoreSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithoutConfigFilePath(nil, nil)
	handler.tokenStore = reloadableAuthStore{
		result: &storepkg.AuthReloadResult{
			Store:     "postgres",
			Total:     3,
			Written:   2,
			Removed:   1,
			Unchanged: 0,
		},
	}

	reloadCalled := false
	handler.SetRuntimeReloadHooks(RuntimeReloadHooks{
		ReloadAuthFilesFromDisk: func() error {
			reloadCalled = true
			return nil
		},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/reload-from-store", nil)

	handler.PostReloadAuthFilesFromStore(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !reloadCalled {
		t.Fatalf("expected runtime auth reload hook to be called")
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["target"] != "auth-files" {
		t.Fatalf("expected target=auth-files, got %#v", payload["target"])
	}
}
