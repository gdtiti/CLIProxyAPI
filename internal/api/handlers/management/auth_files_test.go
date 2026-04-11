package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type managementQuotaStatusErr struct {
	code   int
	msg    string
	retry  time.Duration
	window string
}

func (e managementQuotaStatusErr) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("status %d", e.code)
}

func (e managementQuotaStatusErr) StatusCode() int            { return e.code }
func (e managementQuotaStatusErr) RetryAfter() *time.Duration { d := e.retry; return &d }
func (e managementQuotaStatusErr) CooldownWindow() string     { return e.window }

type managementQuotaExecutor struct {
	err error
}

func (e *managementQuotaExecutor) Identifier() string { return "codex" }

func (e *managementQuotaExecutor) Execute(context.Context, *coreauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, e.err
}

func (e *managementQuotaExecutor) ExecuteStream(context.Context, *coreauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, e.err
}

func (e *managementQuotaExecutor) Refresh(context.Context, *coreauth.Auth) (*coreauth.Auth, error) {
	return nil, nil
}

func (e *managementQuotaExecutor) CountTokens(context.Context, *coreauth.Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, e.err
}

func (e *managementQuotaExecutor) HttpRequest(context.Context, *coreauth.Auth, *http.Request) (*http.Response, error) {
	return nil, e.err
}

func TestResolveAuthQuotaDisplay_UsesGlobalQuotaReason(t *testing.T) {
	auth := &coreauth.Auth{
		Provider: "codex",
		ModelStates: map[string]*coreauth.ModelState{
			"_global": {
				Quota: coreauth.QuotaState{Reason: "quota_weekly"},
			},
		},
	}

	reason, window, display := resolveAuthQuotaDisplay(auth)
	if reason != "quota_weekly" {
		t.Fatalf("reason = %q, want %q", reason, "quota_weekly")
	}
	if window != "weekly" {
		t.Fatalf("window = %q, want %q", window, "weekly")
	}
	if display != "quota exhausted (weekly window)" {
		t.Fatalf("display = %q, want %q", display, "quota exhausted (weekly window)")
	}
}

func TestResolveAuthQuotaDisplay_PrefersAuthStatusMessage(t *testing.T) {
	auth := &coreauth.Auth{
		StatusMessage: "quota exhausted (5h window)",
		Quota:         coreauth.QuotaState{Reason: "quota_5h"},
	}

	reason, window, display := resolveAuthQuotaDisplay(auth)
	if reason != "quota_5h" {
		t.Fatalf("reason = %q, want %q", reason, "quota_5h")
	}
	if window != "five_hour" {
		t.Fatalf("window = %q, want %q", window, "five_hour")
	}
	if display != "quota exhausted (5h window)" {
		t.Fatalf("display = %q, want %q", display, "quota exhausted (5h window)")
	}
}

func TestBuildAuthFileEntry_ExposesQuotaStatusFields(t *testing.T) {
	h := &Handler{}
	now := time.Now()
	auth := &coreauth.Auth{
		ID:       "a-1",
		Provider: "codex",
		Status:   coreauth.StatusError,
		Attributes: map[string]string{
			"runtime_only": "true",
		},
		StatusMessage: "quota exhausted (weekly window)",
		ModelStates: map[string]*coreauth.ModelState{
			"_global": {
				Status:         coreauth.StatusError,
				Unavailable:    true,
				NextRetryAfter: now.Add(time.Hour),
				Quota: coreauth.QuotaState{
					Exceeded:      true,
					Reason:        "quota_weekly",
					NextRecoverAt: now.Add(time.Hour),
				},
			},
		},
	}

	entry := h.buildAuthFileEntry(auth)
	if entry == nil {
		t.Fatalf("buildAuthFileEntry() returned nil")
	}
	if got := entry["status_reason"]; got != "quota_weekly" {
		t.Fatalf("status_reason = %v, want %q", got, "quota_weekly")
	}
	if got := entry["quota_window"]; got != "weekly" {
		t.Fatalf("quota_window = %v, want %q", got, "weekly")
	}
	if got := entry["status_display"]; got != "quota exhausted (weekly window)" {
		t.Fatalf("status_display = %v, want %q", got, "quota exhausted (weekly window)")
	}
}

func TestListAuthFiles_ExposesQuotaWindowFieldsAfterExecutionFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := coreauth.NewManager(nil, nil, nil)
	mgr.RegisterExecutor(&managementQuotaExecutor{err: managementQuotaStatusErr{
		code:   http.StatusTooManyRequests,
		msg:    "usage limit reached",
		retry:  2 * time.Hour,
		window: "five_hour",
	}})

	auth := &coreauth.Auth{
		ID:       "codex-management-list",
		Provider: "codex",
		Attributes: map[string]string{
			"runtime_only": "true",
		},
	}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient(auth.ID, "codex", []*registry.ModelInfo{{ID: "gpt-5", Type: "codex"}})
	defer reg.UnregisterClient(auth.ID)

	_, err := mgr.Execute(
		context.Background(),
		[]string{"codex"},
		cliproxyexecutor.Request{Model: "gpt-5", Payload: []byte(`{}`)},
		cliproxyexecutor.Options{},
	)
	if err == nil {
		t.Fatalf("Execute() error = nil, want non-nil")
	}

	h := &Handler{authManager: mgr}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)

	h.ListAuthFiles(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.Files) == 0 {
		t.Fatalf("expected non-empty files list")
	}
	entry := payload.Files[0]
	if got := entry["status_reason"]; got != "quota_5h" {
		t.Fatalf("status_reason = %v, want %q", got, "quota_5h")
	}
	if got := entry["quota_window"]; got != "five_hour" {
		t.Fatalf("quota_window = %v, want %q", got, "five_hour")
	}
	if got := entry["status_display"]; got != "quota exhausted (5h window)" {
		t.Fatalf("status_display = %v, want %q", got, "quota exhausted (5h window)")
	}
}

func TestListAuthFiles_SupportsSortAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := coreauth.NewManager(nil, nil, nil)
	registerAuth := func(id, name, email string, priority int) {
		t.Helper()
		auth := &coreauth.Auth{
			ID:       id,
			Provider: "codex",
			FileName: name,
			Attributes: map[string]string{
				"runtime_only": "true",
				"priority":     fmt.Sprintf("%d", priority),
			},
			Metadata: map[string]any{
				"email": email,
			},
		}
		if _, err := mgr.Register(context.Background(), auth); err != nil {
			t.Fatalf("register auth %s: %v", id, err)
		}
	}

	registerAuth("auth-a", "a.json", "a@example.com", 1)
	registerAuth("auth-b", "b.json", "b@example.com", 3)
	registerAuth("auth-c", "c.json", "c@example.com", 2)

	h := &Handler{authManager: mgr}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?sort_by=priority&sort_order=desc&page=2&page_size=1", nil)

	h.ListAuthFiles(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var payload struct {
		Files     []map[string]any `json:"files"`
		Total     int              `json:"total"`
		Page      int              `json:"page"`
		PageSize  int              `json:"page_size"`
		SortBy    string           `json:"sort_by"`
		SortOrder string           `json:"sort_order"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Total != 3 {
		t.Fatalf("total = %d, want 3", payload.Total)
	}
	if payload.Page != 2 || payload.PageSize != 1 {
		t.Fatalf("page info = (%d,%d), want (2,1)", payload.Page, payload.PageSize)
	}
	if payload.SortBy != "priority" || payload.SortOrder != "desc" {
		t.Fatalf("sort = (%q,%q), want (%q,%q)", payload.SortBy, payload.SortOrder, "priority", "desc")
	}
	if len(payload.Files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(payload.Files))
	}
	if got := payload.Files[0]["name"]; got != "c.json" {
		t.Fatalf("files[0].name = %v, want %q", got, "c.json")
	}
}

func TestListAuthFiles_RejectsInvalidSortBy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mgr := coreauth.NewManager(nil, nil, nil)
	auth := &coreauth.Auth{
		ID:       "auth-a",
		Provider: "codex",
		FileName: "a.json",
		Attributes: map[string]string{
			"runtime_only": "true",
		},
	}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := &Handler{authManager: mgr}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?sort_by=unknown", nil)

	h.ListAuthFiles(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
}
