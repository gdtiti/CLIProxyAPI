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
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestPatchAuthFilesProxyURLBatchUpdatesAndClearsProxyURL(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	authDir := t.TempDir()

	writeAuthFile := func(name, proxyURL string) string {
		path := filepath.Join(authDir, name)
		data := map[string]any{
			"type":  "codex",
			"email": name + "@example.com",
		}
		if proxyURL != "" {
			data["proxy_url"] = proxyURL
		}
		raw, errMarshal := json.Marshal(data)
		if errMarshal != nil {
			t.Fatalf("Marshal auth file %s: %v", name, errMarshal)
		}
		if errWrite := os.WriteFile(path, raw, 0o600); errWrite != nil {
			t.Fatalf("WriteFile(%s): %v", name, errWrite)
		}
		record := &coreauth.Auth{
			ID:       name,
			FileName: name,
			Provider: "codex",
			ProxyURL: proxyURL,
			Attributes: map[string]string{
				"path": path,
			},
			Metadata: map[string]any{
				"type": "codex",
			},
		}
		if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
			t.Fatalf("Register(%s): %v", name, errRegister)
		}
		return path
	}

	pathA := writeAuthFile("a.json", "http://old-a.example.com:8080")
	pathB := writeAuthFile("b.json", "")

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	body := `{"names":["a.json","b.json"],"proxy_url":"http://new-proxy.example.com:8080"}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/proxy-url/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesProxyURLBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	updatedA, ok := manager.GetByID("a.json")
	if !ok || updatedA == nil {
		t.Fatal("expected a.json to exist in manager")
	}
	if updatedA.ProxyURL != "http://new-proxy.example.com:8080" {
		t.Fatalf("a.json proxy_url = %q, want %q", updatedA.ProxyURL, "http://new-proxy.example.com:8080")
	}

	updatedB, ok := manager.GetByID("b.json")
	if !ok || updatedB == nil {
		t.Fatal("expected b.json to exist in manager")
	}
	if updatedB.ProxyURL != "http://new-proxy.example.com:8080" {
		t.Fatalf("b.json proxy_url = %q, want %q", updatedB.ProxyURL, "http://new-proxy.example.com:8080")
	}

	rawA, errRead := os.ReadFile(pathA)
	if errRead != nil {
		t.Fatalf("ReadFile(a.json): %v", errRead)
	}
	if !strings.Contains(string(rawA), `"proxy_url": "http://new-proxy.example.com:8080"`) {
		t.Fatalf("a.json file missing new proxy_url: %s", string(rawA))
	}

	clearBody := `{"names":["a.json","b.json"],"proxy_url":""}`
	rec = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(rec)
	req = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/proxy-url/batch", strings.NewReader(clearBody))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesProxyURLBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("clear status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	updatedA, _ = manager.GetByID("a.json")
	if updatedA.ProxyURL != "" {
		t.Fatalf("a.json proxy_url after clear = %q, want empty", updatedA.ProxyURL)
	}
	updatedB, _ = manager.GetByID("b.json")
	if updatedB.ProxyURL != "" {
		t.Fatalf("b.json proxy_url after clear = %q, want empty", updatedB.ProxyURL)
	}

	rawB, errRead := os.ReadFile(pathB)
	if errRead != nil {
		t.Fatalf("ReadFile(b.json): %v", errRead)
	}
	if strings.Contains(string(rawB), `"proxy_url"`) {
		t.Fatalf("b.json file should not contain proxy_url after clear: %s", string(rawB))
	}
}

func TestPatchAuthFilesProxyURLBatchDryRunDoesNotPersist(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	authDir := t.TempDir()
	path := filepath.Join(authDir, "dry.json")
	raw := []byte(`{"type":"codex","proxy_url":"http://old.example.com:8080"}`)
	if errWrite := os.WriteFile(path, raw, 0o600); errWrite != nil {
		t.Fatalf("WriteFile(dry.json): %v", errWrite)
	}
	record := &coreauth.Auth{
		ID:       "dry.json",
		FileName: "dry.json",
		Provider: "codex",
		ProxyURL: "http://old.example.com:8080",
		Attributes: map[string]string{
			"path": path,
		},
		Metadata: map[string]any{
			"type":      "codex",
			"proxy_url": "http://old.example.com:8080",
		},
	}
	if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
		t.Fatalf("Register(dry.json): %v", errRegister)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	body := `{"names":["dry.json"],"proxy_url":"http://new.example.com:8080","dry_run":true}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/proxy-url/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesProxyURLBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	updated, _ := manager.GetByID("dry.json")
	if updated.ProxyURL != "http://old.example.com:8080" {
		t.Fatalf("manager proxy_url = %q, want unchanged", updated.ProxyURL)
	}
	current, errRead := os.ReadFile(path)
	if errRead != nil {
		t.Fatalf("ReadFile(dry.json): %v", errRead)
	}
	if string(current) != string(raw) {
		t.Fatalf("file content changed during dry run: %s", string(current))
	}
}
