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

func TestPatchAuthFilesFieldsBatchUpdatesAndClearsFields(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	authDir := t.TempDir()

	writeAuthFile := func(name string, data map[string]any, record *coreauth.Auth) string {
		path := filepath.Join(authDir, name)
		raw, errMarshal := json.Marshal(data)
		if errMarshal != nil {
			t.Fatalf("Marshal(%s): %v", name, errMarshal)
		}
		if errWrite := os.WriteFile(path, raw, 0o600); errWrite != nil {
			t.Fatalf("WriteFile(%s): %v", name, errWrite)
		}
		if record.Attributes == nil {
			record.Attributes = make(map[string]string)
		}
		record.Attributes["path"] = path
		record.Metadata = data
		if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
			t.Fatalf("Register(%s): %v", name, errRegister)
		}
		return path
	}

	pathA := writeAuthFile("a.json", map[string]any{
		"type":     "codex",
		"email":    "a@example.com",
		"prefix":   "team-a",
		"priority": 2,
		"note":     "legacy",
		"headers": map[string]any{
			"X-Old":    "old",
			"X-Remove": "remove-me",
		},
	}, &coreauth.Auth{
		ID:       "a.json",
		FileName: "a.json",
		Provider: "codex",
		Prefix:   "team-a",
		Attributes: map[string]string{
			"header:X-Old":    "old",
			"header:X-Remove": "remove-me",
			"priority":        "2",
			"note":            "legacy",
		},
	})
	pathB := writeAuthFile("b.json", map[string]any{
		"type":  "codex",
		"email": "b@example.com",
	}, &coreauth.Auth{
		ID:       "b.json",
		FileName: "b.json",
		Provider: "codex",
	})

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	body := `{"names":["a.json","b.json"],"prefix":"team-b","headers":{"X-Old":"new","X-New":"v","X-Remove":"","X-Blank":"   "},"priority":5,"note":"hello"}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	for _, id := range []string{"a.json", "b.json"} {
		updated, ok := manager.GetByID(id)
		if !ok || updated == nil {
			t.Fatalf("expected %s to exist in manager", id)
		}
		if updated.Prefix != "team-b" {
			t.Fatalf("%s prefix = %q, want %q", id, updated.Prefix, "team-b")
		}
		if got, _ := updated.Metadata["priority"].(float64); got != 5 {
			if alt, okPriority := parsePriorityValue(updated.Metadata["priority"]); !okPriority || alt != 5 {
				t.Fatalf("%s metadata.priority = %#v, want 5", id, updated.Metadata["priority"])
			}
		}
		if got, _ := updated.Metadata["note"].(string); got != "hello" {
			t.Fatalf("%s metadata.note = %#v, want %q", id, updated.Metadata["note"], "hello")
		}
		headersMeta, ok := updated.Metadata["headers"].(map[string]any)
		if !ok {
			t.Fatalf("%s metadata.headers = %T, want map[string]any", id, updated.Metadata["headers"])
		}
		if got := headersMeta["X-Old"]; got != "new" {
			t.Fatalf("%s metadata.headers.X-Old = %#v, want %q", id, got, "new")
		}
		if got := headersMeta["X-New"]; got != "v" {
			t.Fatalf("%s metadata.headers.X-New = %#v, want %q", id, got, "v")
		}
		if _, ok := headersMeta["X-Remove"]; ok {
			t.Fatalf("%s metadata.headers.X-Remove should be deleted", id)
		}
	}

	rawA, errRead := os.ReadFile(pathA)
	if errRead != nil {
		t.Fatalf("ReadFile(a.json): %v", errRead)
	}
	if !strings.Contains(string(rawA), `"prefix": "team-b"`) || !strings.Contains(string(rawA), `"note": "hello"`) {
		t.Fatalf("a.json missing updated fields: %s", string(rawA))
	}

	clearBody := `{"names":["a.json","b.json"],"prefix":"","priority":0,"note":""}`
	rec = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(rec)
	req = httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(clearBody))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("clear status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	for _, id := range []string{"a.json", "b.json"} {
		updated, _ := manager.GetByID(id)
		if updated.Prefix != "" {
			t.Fatalf("%s prefix after clear = %q, want empty", id, updated.Prefix)
		}
		if _, ok := updated.Metadata["priority"]; ok {
			t.Fatalf("%s metadata.priority should be deleted", id)
		}
		if _, ok := updated.Metadata["note"]; ok {
			t.Fatalf("%s metadata.note should be deleted", id)
		}
	}

	rawB, errRead := os.ReadFile(pathB)
	if errRead != nil {
		t.Fatalf("ReadFile(b.json): %v", errRead)
	}
	if strings.Contains(string(rawB), `"prefix"`) || strings.Contains(string(rawB), `"priority"`) || strings.Contains(string(rawB), `"note"`) {
		t.Fatalf("b.json should not contain cleared fields: %s", string(rawB))
	}
}

func TestPatchAuthFilesFieldsBatchDryRunDoesNotPersist(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	authDir := t.TempDir()
	path := filepath.Join(authDir, "dry.json")
	raw := []byte(`{"type":"codex","email":"dry@example.com","prefix":"old","priority":3,"note":"keep"}`)
	if errWrite := os.WriteFile(path, raw, 0o600); errWrite != nil {
		t.Fatalf("WriteFile(dry.json): %v", errWrite)
	}
	record := &coreauth.Auth{
		ID:       "dry.json",
		FileName: "dry.json",
		Provider: "codex",
		Prefix:   "old",
		Attributes: map[string]string{
			"path":     path,
			"priority": "3",
			"note":     "keep",
		},
		Metadata: map[string]any{
			"type":     "codex",
			"email":    "dry@example.com",
			"prefix":   "old",
			"priority": 3,
			"note":     "keep",
		},
	}
	if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
		t.Fatalf("Register(dry.json): %v", errRegister)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	body := `{"names":["dry.json"],"prefix":"new","priority":9,"note":"dry run","dry_run":true}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	updated, _ := manager.GetByID("dry.json")
	if updated.Prefix != "old" {
		t.Fatalf("manager prefix = %q, want unchanged", updated.Prefix)
	}
	if got, _ := updated.Metadata["note"].(string); got != "keep" {
		t.Fatalf("manager note = %#v, want %q", updated.Metadata["note"], "keep")
	}
	current, errRead := os.ReadFile(path)
	if errRead != nil {
		t.Fatalf("ReadFile(dry.json): %v", errRead)
	}
	if string(current) != string(raw) {
		t.Fatalf("file content changed during dry run: %s", string(current))
	}
}

func TestPatchAuthFilesFieldsBatchRejectsEmptyUpdate(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	authDir := t.TempDir()
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(`{"names":["a.json"]}`))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no fields to update") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
