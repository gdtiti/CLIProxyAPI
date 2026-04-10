package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestPatchAuthFileFields_PersistsAndListsEditableFields(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	fileName := "codex-user@example.com-plus.json"
	filePath := filepath.Join(authDir, fileName)
	if err := os.WriteFile(filePath, []byte(`{"type":"codex","email":"user@example.com"}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = sdkAuth.NewFileTokenStore()

	authID := h.authIDForPath(filePath)
	if _, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:       authID,
		FileName: fileName,
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"path": filePath,
		},
		Metadata: map[string]any{
			"type":  "codex",
			"email": "user@example.com",
		},
	}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	patchRec := httptest.NewRecorder()
	patchCtx, _ := gin.CreateTestContext(patchRec)
	patchReq := httptest.NewRequest(
		http.MethodPatch,
		"/v0/management/auth-files/fields",
		strings.NewReader(`{"name":"`+fileName+`","prefix":"team-a","proxy_url":"https://proxy.example.com","priority":7}`),
	)
	patchReq.Header.Set("Content-Type", "application/json")
	patchCtx.Request = patchReq
	h.PatchAuthFileFields(patchCtx)

	if patchRec.Code != http.StatusOK {
		t.Fatalf("PatchAuthFileFields status = %d, want %d, body=%s", patchRec.Code, http.StatusOK, patchRec.Body.String())
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read auth file: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal auth file: %v", err)
	}
	if got := metadata["prefix"]; got != "team-a" {
		t.Fatalf("file prefix = %v, want %q", got, "team-a")
	}
	if got := metadata["proxy_url"]; got != "https://proxy.example.com" {
		t.Fatalf("file proxy_url = %v, want %q", got, "https://proxy.example.com")
	}
	if got := metadata["priority"]; got != float64(7) {
		t.Fatalf("file priority = %v, want %v", got, float64(7))
	}

	updated, ok := manager.GetByID(authID)
	if !ok || updated == nil {
		t.Fatalf("expected updated auth %q in manager", authID)
	}
	if updated.Prefix != "team-a" {
		t.Fatalf("manager prefix = %q, want %q", updated.Prefix, "team-a")
	}
	if updated.ProxyURL != "https://proxy.example.com" {
		t.Fatalf("manager proxy_url = %q, want %q", updated.ProxyURL, "https://proxy.example.com")
	}
	if got := updated.Attributes["priority"]; got != "7" {
		t.Fatalf("manager priority = %q, want %q", got, "7")
	}

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	h.ListAuthFiles(listCtx)

	if listRec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	entry := payload.Files[0]
	if got := entry["prefix"]; got != "team-a" {
		t.Fatalf("list prefix = %v, want %q", got, "team-a")
	}
	if got := entry["proxy_url"]; got != "https://proxy.example.com" {
		t.Fatalf("list proxy_url = %v, want %q", got, "https://proxy.example.com")
	}
	if got := entry["priority"]; got != float64(7) {
		t.Fatalf("list priority = %v, want %v", got, float64(7))
	}
}

func TestPatchAuthFileStatus_PersistsAndReloadsDisabledState(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	fileName := "codex-toggle-user.json"
	filePath := filepath.Join(authDir, fileName)
	if err := os.WriteFile(filePath, []byte(`{"type":"codex","email":"toggle@example.com"}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = sdkAuth.NewFileTokenStore()

	authID := h.authIDForPath(filePath)
	if _, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:       authID,
		FileName: fileName,
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"path": filePath,
		},
		Metadata: map[string]any{
			"type":  "codex",
			"email": "toggle@example.com",
		},
	}); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	patchStatus := func(disabled bool) string {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(
			http.MethodPatch,
			"/v0/management/auth-files/status",
			strings.NewReader(`{"name":"`+fileName+`","disabled":`+map[bool]string{true: "true", false: "false"}[disabled]+`}`),
		)
		req.Header.Set("Content-Type", "application/json")
		ctx.Request = req
		h.PatchAuthFileStatus(ctx)
		if rec.Code != http.StatusOK {
			t.Fatalf("PatchAuthFileStatus(%t) status = %d, want %d, body=%s", disabled, rec.Code, http.StatusOK, rec.Body.String())
		}
		raw, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read auth file after patch: %v", err)
		}
		var metadata map[string]any
		if err := json.Unmarshal(raw, &metadata); err != nil {
			t.Fatalf("unmarshal auth file: %v", err)
		}
		gotDisabled, _ := metadata["disabled"].(bool)
		if gotDisabled != disabled {
			t.Fatalf("file disabled = %t, want %t", gotDisabled, disabled)
		}

		updated, ok := manager.GetByID(authID)
		if !ok || updated == nil {
			t.Fatalf("expected auth %q in manager", authID)
		}
		if updated.Disabled != disabled {
			t.Fatalf("manager disabled = %t, want %t", updated.Disabled, disabled)
		}
		if disabled {
			if updated.Status != coreauth.StatusDisabled {
				t.Fatalf("manager status = %q, want %q", updated.Status, coreauth.StatusDisabled)
			}
			if updated.StatusMessage != "disabled via management API" {
				t.Fatalf("manager status message = %q, want %q", updated.StatusMessage, "disabled via management API")
			}
		} else {
			if updated.Status == coreauth.StatusDisabled {
				t.Fatalf("manager status should not remain disabled after enable")
			}
			if updated.StatusMessage == "disabled via management API" {
				t.Fatalf("manager status message should be cleared after enable")
			}
		}

		listRec := httptest.NewRecorder()
		listCtx, _ := gin.CreateTestContext(listRec)
		listCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
		h.ListAuthFiles(listCtx)
		if listRec.Code != http.StatusOK {
			t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
		}
		var payload struct {
			Files []map[string]any `json:"files"`
		}
		if err := json.Unmarshal(listRec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal list payload: %v", err)
		}
		if len(payload.Files) != 1 {
			t.Fatalf("files len = %d, want 1", len(payload.Files))
		}
		entry := payload.Files[0]
		entryDisabled, _ := entry["disabled"].(bool)
		if entryDisabled != disabled {
			t.Fatalf("list disabled = %t, want %t", entryDisabled, disabled)
		}
		status, _ := entry["status"].(string)
		statusMsg, _ := entry["status_message"].(string)
		if disabled {
			if status != string(coreauth.StatusDisabled) {
				t.Fatalf("list status = %q, want %q", status, coreauth.StatusDisabled)
			}
			if statusMsg != "disabled via management API" {
				t.Fatalf("list status_message = %q, want %q", statusMsg, "disabled via management API")
			}
		} else {
			if status == string(coreauth.StatusDisabled) {
				t.Fatalf("list status should not remain disabled after enable")
			}
			if statusMsg == "disabled via management API" {
				t.Fatalf("list status_message should be cleared after enable")
			}
		}
		return string(raw)
	}

	disabledRaw := patchStatus(true)
	if !strings.Contains(disabledRaw, `"disabled":true`) {
		t.Fatalf("disabled patch should persist disabled=true, file=%s", disabledRaw)
	}

	enabledRaw := patchStatus(false)
	if strings.Contains(enabledRaw, `"disabled":true`) {
		t.Fatalf("enable patch should clear disabled=true, file=%s", enabledRaw)
	}
}
