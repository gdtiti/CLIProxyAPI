package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestListCodexAuthRefreshHistory_DefaultSortDesc(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	manager := coreauth.NewManager(nil, nil, nil)
	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	authA := &coreauth.Auth{
		ID:       "codex-a",
		Provider: "codex",
		FileName: "codex-a.json",
		Metadata: map[string]any{"email": "alpha@example.com"},
	}
	coreauth.RecordRefreshHistory(authA, base.Add(1*time.Minute), "auto_refresh", "error", "expired", time.Time{})
	coreauth.RecordRefreshHistory(authA, base.Add(3*time.Minute), "auto_refresh", "success", "", base.Add(25*time.Hour))
	if _, err := manager.Register(context.Background(), authA); err != nil {
		t.Fatalf("register authA: %v", err)
	}

	authB := &coreauth.Auth{
		ID:       "codex-b",
		Provider: "codex",
		FileName: "codex-b.json",
		Metadata: map[string]any{"email": "beta@example.com"},
	}
	coreauth.RecordRefreshHistory(authB, base.Add(2*time.Minute), "auto_refresh", "success", "", base.Add(26*time.Hour))
	if _, err := manager.Register(context.Background(), authB); err != nil {
		t.Fatalf("register authB: %v", err)
	}

	authIgnored := &coreauth.Auth{
		ID:       "claude-a",
		Provider: "claude",
		FileName: "claude-a.json",
		Metadata: map[string]any{"email": "claude@example.com"},
	}
	coreauth.RecordRefreshHistory(authIgnored, base.Add(4*time.Minute), "auto_refresh", "success", "", base.Add(24*time.Hour))
	if _, err := manager.Register(context.Background(), authIgnored); err != nil {
		t.Fatalf("register ignored auth: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-refresh-history", nil)

	h.ListCodexAuthRefreshHistory(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Items []struct {
			AuthID string    `json:"auth_id"`
			Result string    `json:"result"`
			At     time.Time `json:"at"`
		} `json:"items"`
		Pagination struct {
			Total    int  `json:"total"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.Items) != 3 {
		t.Fatalf("items length = %d, want 3", len(payload.Items))
	}
	if payload.Items[0].AuthID != "codex-a" || payload.Items[0].Result != "success" {
		t.Fatalf("first item = %+v, want codex-a success", payload.Items[0])
	}
	if payload.Items[1].AuthID != "codex-b" || payload.Items[2].AuthID != "codex-a" {
		t.Fatalf("items order = %+v, want codex-a/codex-b/codex-a", payload.Items)
	}
	if payload.Pagination.Total != 3 || payload.Pagination.Returned != 3 || payload.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want total=3 returned=3 has_more=false", payload.Pagination)
	}
}

func TestListCodexAuthRefreshHistory_SupportsFiltersSortingAndPagination(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	manager := coreauth.NewManager(nil, nil, nil)
	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	authA := &coreauth.Auth{
		ID:       "codex-a",
		Provider: "codex",
		FileName: "codex-a.json",
		Metadata: map[string]any{"email": "alpha@example.com"},
	}
	coreauth.RecordRefreshHistory(authA, base.Add(1*time.Minute), "auto_refresh", "error", "expired token", time.Time{})
	if _, err := manager.Register(context.Background(), authA); err != nil {
		t.Fatalf("register authA: %v", err)
	}

	authB := &coreauth.Auth{
		ID:       "codex-b",
		Provider: "codex",
		FileName: "codex-b.json",
		Metadata: map[string]any{"email": "beta@example.com"},
	}
	coreauth.RecordRefreshHistory(authB, base.Add(2*time.Minute), "auto_refresh", "error", "expired session", time.Time{})
	coreauth.RecordRefreshHistory(authB, base.Add(3*time.Minute), "auto_refresh", "success", "", base.Add(24*time.Hour))
	if _, err := manager.Register(context.Background(), authB); err != nil {
		t.Fatalf("register authB: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/codex-auth-refresh-history?q=expired&result=error&sort_by=email&sort_order=asc&offset=1&limit=1",
		nil,
	)

	h.ListCodexAuthRefreshHistory(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Items []struct {
			AuthID string `json:"auth_id"`
			Email  string `json:"email"`
			Result string `json:"result"`
		} `json:"items"`
		Pagination struct {
			Total    int  `json:"total"`
			Offset   int  `json:"offset"`
			Limit    int  `json:"limit"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(payload.Items))
	}
	if payload.Items[0].AuthID != "codex-b" || payload.Items[0].Email != "beta@example.com" || payload.Items[0].Result != "error" {
		t.Fatalf("item = %+v, want codex-b beta@example.com error", payload.Items[0])
	}
	if payload.Pagination.Total != 2 || payload.Pagination.Offset != 1 || payload.Pagination.Limit != 1 || payload.Pagination.Returned != 1 || payload.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want total=2 offset=1 limit=1 returned=1 has_more=false", payload.Pagination)
	}
}
