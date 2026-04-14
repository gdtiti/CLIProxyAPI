package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func registerEditableAuthFileForTest(t *testing.T, h *Handler, manager *coreauth.Manager, authDir, name string, metadata map[string]any) string {
	t.Helper()

	if metadata == nil {
		metadata = make(map[string]any)
	}
	provider, _ := metadata["type"].(string)
	if provider == "" {
		provider = "codex"
		metadata["type"] = provider
	}
	if _, ok := metadata["email"]; !ok {
		metadata["email"] = name + "@example.com"
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal auth metadata for %s: %v", name, err)
	}

	path := filepath.Join(authDir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file %s: %v", name, err)
	}

	authID := h.authIDForPath(path)
	record := &coreauth.Auth{
		ID:       authID,
		FileName: name,
		Provider: provider,
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"path": path,
		},
		Metadata: metadata,
	}
	if prefix, _ := metadata["prefix"].(string); prefix != "" {
		record.Prefix = prefix
	}
	if proxyURL, _ := metadata["proxy_url"].(string); proxyURL != "" {
		record.ProxyURL = proxyURL
	}
	if note, _ := metadata["note"].(string); note != "" {
		record.Attributes["note"] = note
	}
	switch value := metadata["priority"].(type) {
	case int:
		record.Attributes["priority"] = strconv.Itoa(value)
	case float64:
		record.Attributes["priority"] = strconv.Itoa(int(value))
	}
	if headers, ok := metadata["headers"].(map[string]any); ok {
		for key, value := range headers {
			headerValue, _ := value.(string)
			if headerValue != "" {
				record.Attributes["header:"+key] = headerValue
			}
		}
	}

	if _, err := manager.Register(context.Background(), record); err != nil {
		t.Fatalf("register auth %s: %v", name, err)
	}
	return authID
}

func readAuthFileMetadataForTest(t *testing.T, authDir, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(authDir, name))
	if err != nil {
		t.Fatalf("read auth file %s: %v", name, err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal auth file %s: %v", name, err)
	}
	return metadata
}

func TestPatchAuthFileFields_MergeHeadersAndDeleteEmptyValues(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	authDir := t.TempDir()
	authPath := filepath.Join(authDir, "test.json")
	record := &coreauth.Auth{
		ID:       "test.json",
		FileName: "test.json",
		Provider: "claude",
		Attributes: map[string]string{
			"path":            authPath,
			"header:X-Old":    "old",
			"header:X-Remove": "gone",
		},
		Metadata: map[string]any{
			"type": "claude",
			"headers": map[string]any{
				"X-Old":    "old",
				"X-Remove": "gone",
			},
		},
	}
	if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
		t.Fatalf("failed to register auth record: %v", errRegister)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	body := `{"name":"test.json","prefix":"p1","proxy_url":"http://proxy.local","headers":{"X-Old":"new","X-New":"v","X-Remove":"  ","X-Nope":""}}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFileFields(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	updated, ok := manager.GetByID("test.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth record to exist after patch")
	}

	if updated.Prefix != "p1" {
		t.Fatalf("prefix = %q, want %q", updated.Prefix, "p1")
	}
	if updated.ProxyURL != "http://proxy.local" {
		t.Fatalf("proxy_url = %q, want %q", updated.ProxyURL, "http://proxy.local")
	}

	if updated.Metadata == nil {
		t.Fatalf("expected metadata to be non-nil")
	}
	if got, _ := updated.Metadata["prefix"].(string); got != "p1" {
		t.Fatalf("metadata.prefix = %q, want %q", got, "p1")
	}
	if got, _ := updated.Metadata["proxy_url"].(string); got != "http://proxy.local" {
		t.Fatalf("metadata.proxy_url = %q, want %q", got, "http://proxy.local")
	}

	headersMeta, ok := updated.Metadata["headers"].(map[string]any)
	if !ok {
		raw, _ := json.Marshal(updated.Metadata["headers"])
		t.Fatalf("metadata.headers = %T (%s), want map[string]any", updated.Metadata["headers"], string(raw))
	}
	if got := headersMeta["X-Old"]; got != "new" {
		t.Fatalf("metadata.headers.X-Old = %#v, want %q", got, "new")
	}
	if got := headersMeta["X-New"]; got != "v" {
		t.Fatalf("metadata.headers.X-New = %#v, want %q", got, "v")
	}
	if _, ok := headersMeta["X-Remove"]; ok {
		t.Fatalf("expected metadata.headers.X-Remove to be deleted")
	}
	if _, ok := headersMeta["X-Nope"]; ok {
		t.Fatalf("expected metadata.headers.X-Nope to be absent")
	}

	if got := updated.Attributes["header:X-Old"]; got != "new" {
		t.Fatalf("attrs header:X-Old = %q, want %q", got, "new")
	}
	if got := updated.Attributes["header:X-New"]; got != "v" {
		t.Fatalf("attrs header:X-New = %q, want %q", got, "v")
	}
	if _, ok := updated.Attributes["header:X-Remove"]; ok {
		t.Fatalf("expected attrs header:X-Remove to be deleted")
	}
	if _, ok := updated.Attributes["header:X-Nope"]; ok {
		t.Fatalf("expected attrs header:X-Nope to be absent")
	}
}

func TestPatchAuthFileFields_HeadersEmptyMapIsNoop(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	authDir := t.TempDir()
	authPath := filepath.Join(authDir, "noop.json")
	record := &coreauth.Auth{
		ID:       "noop.json",
		FileName: "noop.json",
		Provider: "claude",
		Attributes: map[string]string{
			"path":         authPath,
			"header:X-Kee": "1",
		},
		Metadata: map[string]any{
			"type": "claude",
			"headers": map[string]any{
				"X-Kee": "1",
			},
		},
	}
	if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
		t.Fatalf("failed to register auth record: %v", errRegister)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	body := `{"name":"noop.json","note":"hello","headers":{}}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFileFields(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	updated, ok := manager.GetByID("noop.json")
	if !ok || updated == nil {
		t.Fatalf("expected auth record to exist after patch")
	}
	if got := updated.Attributes["header:X-Kee"]; got != "1" {
		t.Fatalf("attrs header:X-Kee = %q, want %q", got, "1")
	}
	headersMeta, ok := updated.Metadata["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata.headers to remain a map, got %T", updated.Metadata["headers"])
	}
	if got := headersMeta["X-Kee"]; got != "1" {
		t.Fatalf("metadata.headers.X-Kee = %#v, want %q", got, "1")
	}
}

func TestPatchAuthFilesFieldsBatch_SetsAndClearsProxyURL(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = sdkAuth.NewFileTokenStore()

	register := func(name, provider string) string {
		path := filepath.Join(authDir, name)
		if err := os.WriteFile(path, []byte(`{"type":"`+provider+`","email":"`+name+`@example.com"}`), 0o600); err != nil {
			t.Fatalf("write auth file %s: %v", name, err)
		}
		authID := h.authIDForPath(path)
		if _, err := manager.Register(context.Background(), &coreauth.Auth{
			ID:       authID,
			FileName: name,
			Provider: provider,
			Status:   coreauth.StatusActive,
			Attributes: map[string]string{
				"path": path,
			},
			Metadata: map[string]any{
				"type":  provider,
				"email": name + "@example.com",
			},
		}); err != nil {
			t.Fatalf("register auth %s: %v", name, err)
		}
		return authID
	}

	firstID := register("alpha.json", "codex")
	secondID := register("beta.json", "claude")

	callBatch := func(body string) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx.Request = req
		h.PatchAuthFilesFieldsBatch(ctx)
		if rec.Code != http.StatusOK {
			t.Fatalf("PatchAuthFilesFieldsBatch status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	callBatch(`{"names":["alpha.json","beta.json"],"proxy_url":"https://proxy.example.com"}`)

	for _, item := range []struct {
		id   string
		name string
	}{
		{id: firstID, name: "alpha.json"},
		{id: secondID, name: "beta.json"},
	} {
		updated, ok := manager.GetByID(item.id)
		if !ok || updated == nil {
			t.Fatalf("expected auth %s in manager", item.id)
		}
		if updated.ProxyURL != "https://proxy.example.com" {
			t.Fatalf("%s proxy_url = %q, want %q", item.name, updated.ProxyURL, "https://proxy.example.com")
		}

		raw, err := os.ReadFile(filepath.Join(authDir, item.name))
		if err != nil {
			t.Fatalf("read auth file %s: %v", item.name, err)
		}
		var metadata map[string]any
		if err := json.Unmarshal(raw, &metadata); err != nil {
			t.Fatalf("unmarshal auth file %s: %v", item.name, err)
		}
		if got := metadata["proxy_url"]; got != "https://proxy.example.com" {
			t.Fatalf("%s file proxy_url = %v, want %q", item.name, got, "https://proxy.example.com")
		}
	}

	callBatch(`{"names":["alpha.json","beta.json"],"proxy_url":""}`)

	for _, item := range []struct {
		id   string
		name string
	}{
		{id: firstID, name: "alpha.json"},
		{id: secondID, name: "beta.json"},
	} {
		updated, ok := manager.GetByID(item.id)
		if !ok || updated == nil {
			t.Fatalf("expected auth %s in manager after clear", item.id)
		}
		if updated.ProxyURL != "" {
			t.Fatalf("%s proxy_url after clear = %q, want empty", item.name, updated.ProxyURL)
		}

		raw, err := os.ReadFile(filepath.Join(authDir, item.name))
		if err != nil {
			t.Fatalf("read auth file %s after clear: %v", item.name, err)
		}
		var metadata map[string]any
		if err := json.Unmarshal(raw, &metadata); err != nil {
			t.Fatalf("unmarshal auth file %s after clear: %v", item.name, err)
		}
		if _, ok := metadata["proxy_url"]; ok {
			t.Fatalf("%s file proxy_url should be removed after clear", item.name)
		}
	}
}

func TestListAuthFiles_SupportsSortByPriorityDesc(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(&memoryAuthStore{}, nil, nil)
	now := time.Now().UTC()
	records := []*coreauth.Auth{
		{
			ID:        "gamma.json",
			FileName:  "gamma.json",
			Provider:  "codex",
			UpdatedAt: now.Add(-time.Hour),
			Attributes: map[string]string{
				"path":     "/tmp/gamma.json",
				"priority": "3",
			},
			Metadata: map[string]any{"type": "codex", "priority": 3},
		},
		{
			ID:        "alpha.json",
			FileName:  "alpha.json",
			Provider:  "claude",
			UpdatedAt: now,
			Attributes: map[string]string{
				"path":     "/tmp/alpha.json",
				"priority": "9",
			},
			Metadata: map[string]any{"type": "claude", "priority": 9},
		},
		{
			ID:        "beta.json",
			FileName:  "beta.json",
			Provider:  "gemini",
			UpdatedAt: now.Add(-2 * time.Hour),
			Attributes: map[string]string{
				"path":     "/tmp/beta.json",
				"priority": "5",
			},
			Metadata: map[string]any{"type": "gemini", "priority": 5},
		},
	}
	for _, record := range records {
		if _, err := manager.Register(context.Background(), record); err != nil {
			t.Fatalf("register auth %s: %v", record.ID, err)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?sort_by=priority&sort_order=desc", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 3 {
		t.Fatalf("files len = %d, want 3", len(payload.Files))
	}

	gotOrder := []string{
		payload.Files[0]["name"].(string),
		payload.Files[1]["name"].(string),
		payload.Files[2]["name"].(string),
	}
	wantOrder := []string{"alpha.json", "beta.json", "gamma.json"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("sorted order[%d] = %q, want %q (full=%v)", i, gotOrder[i], wantOrder[i], gotOrder)
		}
	}
}

func TestListAuthFiles_SupportsPaginationFromManager(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(&memoryAuthStore{}, nil, nil)
	now := time.Now().UTC()
	records := []*coreauth.Auth{
		{
			ID:        "alpha.json",
			FileName:  "alpha.json",
			Provider:  "claude",
			UpdatedAt: now,
			Attributes: map[string]string{
				"path": "/tmp/alpha.json",
			},
			Metadata: map[string]any{"type": "claude"},
		},
		{
			ID:        "beta.json",
			FileName:  "beta.json",
			Provider:  "codex",
			UpdatedAt: now,
			Attributes: map[string]string{
				"path": "/tmp/beta.json",
			},
			Metadata: map[string]any{"type": "codex"},
		},
		{
			ID:        "gamma.json",
			FileName:  "gamma.json",
			Provider:  "gemini",
			UpdatedAt: now,
			Attributes: map[string]string{
				"path": "/tmp/gamma.json",
			},
			Metadata: map[string]any{"type": "gemini"},
		},
	}
	for _, record := range records {
		if _, err := manager.Register(context.Background(), record); err != nil {
			t.Fatalf("register auth %s: %v", record.ID, err)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?sort_by=name&sort_order=asc&offset=1&limit=1", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Files      []map[string]any `json:"files"`
		Pagination struct {
			Total    int  `json:"total"`
			Offset   int  `json:"offset"`
			Limit    int  `json:"limit"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	if payload.Files[0]["name"] != "beta.json" {
		t.Fatalf("files[0].name = %v, want %q", payload.Files[0]["name"], "beta.json")
	}
	if payload.Pagination.Total != 3 || payload.Pagination.Offset != 1 || payload.Pagination.Limit != 1 || payload.Pagination.Returned != 1 || !payload.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want total=3 offset=1 limit=1 returned=1 has_more=true", payload.Pagination)
	}
}

func TestListAuthFiles_SupportsPaginationFromDisk(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	files := []string{"alpha.json", "beta.json", "gamma.json"}
	for _, name := range files {
		content := map[string]any{
			"type":  "codex",
			"email": name + "@example.com",
		}
		raw, err := json.Marshal(content)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(authDir, name), raw, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	h := &Handler{cfg: &config.Config{AuthDir: authDir}}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?sort_by=name&sort_order=asc&offset=2&limit=5", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Files      []map[string]any `json:"files"`
		Pagination struct {
			Total    int  `json:"total"`
			Offset   int  `json:"offset"`
			Limit    int  `json:"limit"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	if payload.Files[0]["name"] != "gamma.json" {
		t.Fatalf("files[0].name = %v, want %q", payload.Files[0]["name"], "gamma.json")
	}
	if payload.Pagination.Total != 3 || payload.Pagination.Offset != 2 || payload.Pagination.Limit != 5 || payload.Pagination.Returned != 1 || payload.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want total=3 offset=2 limit=5 returned=1 has_more=false", payload.Pagination)
	}
}

func TestListAuthFiles_SupportsFiltersFromManager(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(&memoryAuthStore{}, nil, nil)
	authDir := t.TempDir()
	records := []*coreauth.Auth{
		{
			ID:       "alpha-id",
			FileName: "alpha.json",
			Provider: "codex",
			Status:   coreauth.StatusActive,
			Disabled: false,
			Prefix:   "team-a",
			ProxyURL: "http://proxy-a.local",
			Attributes: map[string]string{
				"path": filepath.Join(authDir, "alpha.json"),
			},
			Metadata: map[string]any{
				"type":      "codex",
				"email":     "alpha@example.com",
				"prefix":    "team-a",
				"proxy_url": "http://proxy-a.local",
			},
		},
		{
			ID:       "beta-id",
			FileName: "beta.json",
			Provider: "claude",
			Status:   coreauth.StatusDisabled,
			Disabled: true,
			Prefix:   "team-b",
			ProxyURL: "http://proxy-b.local",
			Attributes: map[string]string{
				"path": filepath.Join(authDir, "beta.json"),
			},
			Metadata: map[string]any{
				"type":      "claude",
				"email":     "beta@example.com",
				"prefix":    "team-b",
				"proxy_url": "http://proxy-b.local",
			},
		},
		{
			ID:       "gamma-id",
			FileName: "gamma.json",
			Provider: "codex",
			Status:   coreauth.StatusActive,
			Disabled: false,
			Prefix:   "team-a",
			ProxyURL: "http://proxy-c.local",
			Attributes: map[string]string{
				"path": filepath.Join(authDir, "gamma.json"),
			},
			Metadata: map[string]any{
				"type":      "codex",
				"email":     "ops@example.com",
				"prefix":    "team-a",
				"proxy_url": "http://proxy-c.local",
			},
		},
	}
	for _, record := range records {
		if _, err := manager.Register(context.Background(), record); err != nil {
			t.Fatalf("register auth %s: %v", record.FileName, err)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?provider=codex&prefix=team-a&status=active&email=alpha&proxy_url=proxy-a&disabled=false", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Files      []map[string]any `json:"files"`
		Pagination struct {
			Total    int  `json:"total"`
			Offset   int  `json:"offset"`
			Limit    int  `json:"limit"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	if payload.Files[0]["name"] != "alpha.json" {
		t.Fatalf("files[0].name = %v, want %q", payload.Files[0]["name"], "alpha.json")
	}
	if payload.Pagination.Total != 1 || payload.Pagination.Returned != 1 || payload.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want total=1 returned=1 has_more=false", payload.Pagination)
	}
}

func TestListAuthFiles_SupportsFiltersFromDisk(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	fixtures := map[string]map[string]any{
		"alpha.json": {
			"type":      "codex",
			"email":     "alpha@example.com",
			"prefix":    "team-a",
			"proxy_url": "http://proxy-a.local",
			"disabled":  false,
		},
		"beta.json": {
			"type":      "claude",
			"email":     "beta@example.com",
			"prefix":    "team-b",
			"proxy_url": "http://proxy-b.local",
			"disabled":  true,
		},
		"gamma.json": {
			"type":      "codex",
			"email":     "ops@example.com",
			"prefix":    "team-a",
			"proxy_url": "http://proxy-c.local",
			"disabled":  false,
		},
	}
	for name, content := range fixtures {
		raw, err := json.Marshal(content)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(authDir, name), raw, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	h := &Handler{cfg: &config.Config{AuthDir: authDir}}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?provider=codex&prefix=team-a&proxy_url=proxy-c&disabled=false&offset=0&limit=1", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Files      []map[string]any `json:"files"`
		Pagination struct {
			Total    int  `json:"total"`
			Offset   int  `json:"offset"`
			Limit    int  `json:"limit"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	if payload.Files[0]["name"] != "gamma.json" {
		t.Fatalf("files[0].name = %v, want %q", payload.Files[0]["name"], "gamma.json")
	}
	if payload.Pagination.Total != 1 || payload.Pagination.Limit != 1 || payload.Pagination.Returned != 1 || payload.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want total=1 limit=1 returned=1 has_more=false", payload.Pagination)
	}
}

func TestListAuthFiles_SupportsQuotaAndExpiryFiltersFromManager(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(&memoryAuthStore{}, nil, nil)
	authDir := t.TempDir()
	base := time.Now().UTC().Truncate(time.Second)

	records := []*coreauth.Auth{
		{
			ID:       "alpha-id",
			FileName: "alpha.json",
			Provider: "codex",
			Status:   coreauth.StatusActive,
			Attributes: map[string]string{
				"path": filepath.Join(authDir, "alpha.json"),
			},
			Metadata: map[string]any{
				"type":  "codex",
				"email": "alpha@example.com",
			},
		},
		{
			ID:       "beta-id",
			FileName: "beta.json",
			Provider: "codex",
			Status:   coreauth.StatusDisabled,
			Disabled: true,
			Attributes: map[string]string{
				"path": filepath.Join(authDir, "beta.json"),
			},
			Metadata: map[string]any{
				"type":       "codex",
				"email":      "beta@example.com",
				"expires_at": base.Add(12 * time.Hour).Format(time.RFC3339),
				coreauth.PersistedRuntimeStateMetadataKey: map[string]any{
					"auths": map[string]any{
						"beta-id": map[string]any{
							"request_count": 7,
						},
					},
				},
			},
			Quota: coreauth.QuotaState{
				Exceeded:      true,
				Reason:        "quota_weekly",
				NextRecoverAt: base.Add(48 * time.Hour),
				BackoffLevel:  3,
			},
		},
		{
			ID:       "gamma-id",
			FileName: "gamma.json",
			Provider: "codex",
			Status:   coreauth.StatusActive,
			Attributes: map[string]string{
				"path": filepath.Join(authDir, "gamma.json"),
			},
			Metadata: map[string]any{
				"type":       "codex",
				"email":      "gamma@example.com",
				"expires_at": base.Add(24 * time.Hour).Format(time.RFC3339),
			},
			Quota: coreauth.QuotaState{
				Exceeded:      true,
				Reason:        "quota_5h",
				NextRecoverAt: base.Add(2 * time.Hour),
				BackoffLevel:  1,
			},
		},
	}
	for _, record := range records {
		if err := os.WriteFile(filepath.Join(authDir, record.FileName), []byte(`{"type":"codex"}`), 0o600); err != nil {
			t.Fatalf("write %s: %v", record.FileName, err)
		}
		if _, err := manager.Register(context.Background(), record); err != nil {
			t.Fatalf("register auth %s: %v", record.FileName, err)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/auth-files?state=disabled&quota_checked=true&quota_level=low&has_expiry=true&expired=false&sort_by=expires_at&sort_order=asc",
		nil,
	)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Files      []map[string]any `json:"files"`
		Pagination struct {
			Total    int `json:"total"`
			Returned int `json:"returned"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	entry := payload.Files[0]
	if entry["name"] != "beta.json" {
		t.Fatalf("name = %v, want %q", entry["name"], "beta.json")
	}
	if entry["quota_level"] != "low" {
		t.Fatalf("quota_level = %v, want %q", entry["quota_level"], "low")
	}
	if entry["quota_checked"] != true {
		t.Fatalf("quota_checked = %v, want true", entry["quota_checked"])
	}
	if entry["state"] != "disabled" {
		t.Fatalf("state = %v, want %q", entry["state"], "disabled")
	}
	if payload.Pagination.Total != 1 || payload.Pagination.Returned != 1 {
		t.Fatalf("pagination = %+v, want total=1 returned=1", payload.Pagination)
	}
}

func TestListAuthFiles_SupportsQuotaAndExpiryFiltersFromDisk(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	base := time.Now().UTC().Truncate(time.Second)
	fixtures := map[string]map[string]any{
		"alpha.json": {
			"type":       "codex",
			"email":      "alpha@example.com",
			"expires_at": base.Add(24 * time.Hour).Format(time.RFC3339),
			"disabled":   false,
		},
		"beta.json": {
			"type":       "codex",
			"email":      "beta@example.com",
			"expires_at": base.Add(-2 * time.Hour).Format(time.RFC3339),
			"disabled":   false,
		},
		"gamma.json": {
			"type":     "codex",
			"email":    "gamma@example.com",
			"disabled": true,
		},
	}
	for name, content := range fixtures {
		raw, err := json.Marshal(content)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(authDir, name), raw, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	h := &Handler{cfg: &config.Config{AuthDir: authDir}}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/auth-files?state=normal&quota_checked=false&quota_level=unchecked&has_expiry=true&expired=false&sort_by=expires_at&sort_order=asc",
		nil,
	)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Files      []map[string]any `json:"files"`
		Pagination struct {
			Total    int `json:"total"`
			Returned int `json:"returned"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	entry := payload.Files[0]
	if entry["name"] != "alpha.json" {
		t.Fatalf("name = %v, want %q", entry["name"], "alpha.json")
	}
	if entry["quota_level"] != "unchecked" {
		t.Fatalf("quota_level = %v, want %q", entry["quota_level"], "unchecked")
	}
	if entry["quota_checked"] != false {
		t.Fatalf("quota_checked = %v, want false", entry["quota_checked"])
	}
	if entry["state"] != "normal" {
		t.Fatalf("state = %v, want %q", entry["state"], "normal")
	}
	if payload.Pagination.Total != 1 || payload.Pagination.Returned != 1 {
		t.Fatalf("pagination = %+v, want total=1 returned=1", payload.Pagination)
	}
}

func TestPatchAuthFilesFieldsBatch_DryRunDoesNotPersistChanges(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = sdkAuth.NewFileTokenStore()

	authID := registerEditableAuthFileForTest(t, h, manager, authDir, "alpha.json", map[string]any{
		"type":     "codex",
		"email":    "alpha@example.com",
		"prefix":   "old-prefix",
		"priority": 1,
		"note":     "old-note",
		"headers": map[string]any{
			"X-Test": "old",
		},
	})

	body := `{"names":["alpha.json"],"prefix":"new-prefix","priority":7,"note":"new-note","headers":{"X-Test":"new","X-Extra":"v"},"dry_run":true}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("PatchAuthFilesFieldsBatch status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Status  string `json:"status"`
		DryRun  bool   `json:"dry_run"`
		Summary struct {
			Updated int `json:"updated"`
			Failed  int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Changed bool   `json:"changed"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal batch response: %v", err)
	}
	if payload.Status != "dry_run" || !payload.DryRun {
		t.Fatalf("response status = %q dry_run=%v, want dry_run/true", payload.Status, payload.DryRun)
	}
	if payload.Summary.Updated != 1 || payload.Summary.Failed != 0 {
		t.Fatalf("summary = %+v, want updated=1 failed=0", payload.Summary)
	}
	if len(payload.Results) != 1 || payload.Results[0].Status != "would_update" || !payload.Results[0].Changed {
		t.Fatalf("results = %+v, want single would_update changed=true", payload.Results)
	}

	updated, ok := manager.GetByID(authID)
	if !ok || updated == nil {
		t.Fatalf("expected auth %s in manager", authID)
	}
	if updated.Prefix != "old-prefix" {
		t.Fatalf("manager prefix = %q, want %q", updated.Prefix, "old-prefix")
	}
	if got, _ := updated.Metadata["priority"].(int); got != 1 {
		t.Fatalf("manager metadata.priority = %d, want %d", got, 1)
	}
	if got, _ := updated.Metadata["note"].(string); got != "old-note" {
		t.Fatalf("manager metadata.note = %q, want %q", got, "old-note")
	}

	metadata := readAuthFileMetadataForTest(t, authDir, "alpha.json")
	if got, _ := metadata["prefix"].(string); got != "old-prefix" {
		t.Fatalf("file prefix = %q, want %q", got, "old-prefix")
	}
	if got := int(metadata["priority"].(float64)); got != 1 {
		t.Fatalf("file priority = %d, want %d", got, 1)
	}
	if got, _ := metadata["note"].(string); got != "old-note" {
		t.Fatalf("file note = %q, want %q", got, "old-note")
	}
}

func TestPatchAuthFilesFieldsBatch_StopOnErrorSkipsRemainingItems(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = sdkAuth.NewFileTokenStore()

	alphaID := registerEditableAuthFileForTest(t, h, manager, authDir, "alpha.json", map[string]any{
		"type":  "codex",
		"email": "alpha@example.com",
		"note":  "keep-alpha",
	})
	betaID := registerEditableAuthFileForTest(t, h, manager, authDir, "beta.json", map[string]any{
		"type":  "claude",
		"email": "beta@example.com",
		"note":  "keep-beta",
	})

	body := `{"names":["missing.json","alpha.json","beta.json"],"note":"patched","stop_on_error":true}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("PatchAuthFilesFieldsBatch status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Status  string `json:"status"`
		Summary struct {
			Failed  int `json:"failed"`
			Skipped int `json:"skipped"`
			Updated int `json:"updated"`
		} `json:"summary"`
		Results []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal batch response: %v", err)
	}
	if payload.Status != "partial" {
		t.Fatalf("response status = %q, want %q", payload.Status, "partial")
	}
	if payload.Summary.Failed != 1 || payload.Summary.Skipped != 2 || payload.Summary.Updated != 0 {
		t.Fatalf("summary = %+v, want failed=1 skipped=2 updated=0", payload.Summary)
	}
	if len(payload.Results) != 3 {
		t.Fatalf("len(results) = %d, want %d", len(payload.Results), 3)
	}
	if payload.Results[0].Name != "missing.json" || payload.Results[0].Status != "failed" || payload.Results[0].Error == "" {
		t.Fatalf("results[0] = %+v, want missing.json failed with error", payload.Results[0])
	}
	if payload.Results[1].Status != "skipped" || payload.Results[2].Status != "skipped" {
		t.Fatalf("results skip states invalid: %+v", payload.Results)
	}

	alpha, ok := manager.GetByID(alphaID)
	if !ok || alpha == nil {
		t.Fatalf("expected alpha auth in manager")
	}
	beta, ok := manager.GetByID(betaID)
	if !ok || beta == nil {
		t.Fatalf("expected beta auth in manager")
	}
	if got, _ := alpha.Metadata["note"].(string); got != "keep-alpha" {
		t.Fatalf("alpha note = %q, want %q", got, "keep-alpha")
	}
	if got, _ := beta.Metadata["note"].(string); got != "keep-beta" {
		t.Fatalf("beta note = %q, want %q", got, "keep-beta")
	}
}

func TestPatchAuthFilesFieldsBatch_UpdatesMultipleEditableFields(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = sdkAuth.NewFileTokenStore()

	authID := registerEditableAuthFileForTest(t, h, manager, authDir, "alpha.json", map[string]any{
		"type":     "codex",
		"email":    "alpha@example.com",
		"prefix":   "old-prefix",
		"priority": 1,
		"note":     "old-note",
		"headers": map[string]any{
			"X-Test":   "old",
			"X-Remove": "gone",
		},
	})

	body := `{"names":["alpha.json"],"prefix":"new-prefix","priority":9,"note":"new-note","headers":{"X-Test":"new","X-Remove":"","X-Added":"v"}}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("PatchAuthFilesFieldsBatch status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Status  string `json:"status"`
		Summary struct {
			Updated int `json:"updated"`
			Failed  int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal batch response: %v", err)
	}
	if payload.Status != "ok" || payload.Summary.Updated != 1 || payload.Summary.Failed != 0 || len(payload.Results) != 1 || payload.Results[0].Status != "updated" {
		t.Fatalf("response payload unexpected: %+v", payload)
	}

	updated, ok := manager.GetByID(authID)
	if !ok || updated == nil {
		t.Fatalf("expected auth %s in manager", authID)
	}
	if updated.Prefix != "new-prefix" {
		t.Fatalf("manager prefix = %q, want %q", updated.Prefix, "new-prefix")
	}
	if got := updated.Attributes["priority"]; got != "9" {
		t.Fatalf("manager attrs priority = %q, want %q", got, "9")
	}
	if got := updated.Attributes["note"]; got != "new-note" {
		t.Fatalf("manager attrs note = %q, want %q", got, "new-note")
	}
	if got := updated.Attributes["header:X-Test"]; got != "new" {
		t.Fatalf("manager attrs header:X-Test = %q, want %q", got, "new")
	}
	if _, ok := updated.Attributes["header:X-Remove"]; ok {
		t.Fatalf("manager attrs header:X-Remove should be deleted")
	}
	if got := updated.Attributes["header:X-Added"]; got != "v" {
		t.Fatalf("manager attrs header:X-Added = %q, want %q", got, "v")
	}

	metadata := readAuthFileMetadataForTest(t, authDir, "alpha.json")
	if got, _ := metadata["prefix"].(string); got != "new-prefix" {
		t.Fatalf("file prefix = %q, want %q", got, "new-prefix")
	}
	if got := int(metadata["priority"].(float64)); got != 9 {
		t.Fatalf("file priority = %d, want %d", got, 9)
	}
	if got, _ := metadata["note"].(string); got != "new-note" {
		t.Fatalf("file note = %q, want %q", got, "new-note")
	}
	headers, ok := metadata["headers"].(map[string]any)
	if !ok {
		t.Fatalf("file headers type = %T, want map[string]any", metadata["headers"])
	}
	if got := headers["X-Test"]; got != "new" {
		t.Fatalf("file headers[X-Test] = %#v, want %q", got, "new")
	}
	if _, ok := headers["X-Remove"]; ok {
		t.Fatalf("file headers[X-Remove] should be deleted")
	}
	if got := headers["X-Added"]; got != "v" {
		t.Fatalf("file headers[X-Added] = %#v, want %q", got, "v")
	}
}

func TestPatchAuthFilesFieldsBatch_UnchangedWhenValueMatchesExistingState(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = sdkAuth.NewFileTokenStore()

	registerEditableAuthFileForTest(t, h, manager, authDir, "alpha.json", map[string]any{
		"type":      "codex",
		"email":     "alpha@example.com",
		"prefix":    "same-prefix",
		"proxy_url": "https://proxy.example.com",
		"priority":  5,
		"note":      "same-note",
	})

	body := `{"names":["alpha.json"],"prefix":"same-prefix","proxy_url":"https://proxy.example.com","priority":5,"note":"same-note"}`
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/fields/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	h.PatchAuthFilesFieldsBatch(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("PatchAuthFilesFieldsBatch status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Status  string `json:"status"`
		Summary struct {
			Updated   int `json:"updated"`
			Unchanged int `json:"unchanged"`
			Failed    int `json:"failed"`
		} `json:"summary"`
		Results []struct {
			Status  string `json:"status"`
			Changed bool   `json:"changed"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal batch response: %v", err)
	}
	if payload.Status != "ok" || payload.Summary.Updated != 0 || payload.Summary.Unchanged != 1 || payload.Summary.Failed != 0 {
		t.Fatalf("summary = %+v, want updated=0 unchanged=1 failed=0", payload.Summary)
	}
	if len(payload.Results) != 1 || payload.Results[0].Status != "unchanged" || payload.Results[0].Changed {
		t.Fatalf("results = %+v, want unchanged changed=false", payload.Results)
	}
}
