package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	if got := entry["quota_checked"]; got != true {
		t.Fatalf("quota_checked = %v, want true", got)
	}
	if got := entry["quota_level"]; got != "low" {
		t.Fatalf("quota_level = %v, want %q", got, "low")
	}
	if got := entry["quota_exceeded"]; got != true {
		t.Fatalf("quota_exceeded = %v, want true", got)
	}
	if got := entry["quota_reason"]; got != "quota_weekly" {
		t.Fatalf("quota_reason = %v, want %q", got, "quota_weekly")
	}
	if got := entry["quota_backoff_level"]; got != nil {
		t.Fatalf("quota_backoff_level should be omitted when zero, got %v", got)
	}
	nextRecoverAt, ok := entry["next_recover_at"].(time.Time)
	if !ok || !nextRecoverAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("next_recover_at = %v, want %v", entry["next_recover_at"], now.Add(time.Hour))
	}
	if got := entry["state"]; got != "normal" {
		t.Fatalf("state = %v, want %q", got, "normal")
	}
}

func TestBuildAuthFileEntry_ExposesExpirationAndQuotaSummaryFields(t *testing.T) {
	h := &Handler{}
	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(4 * time.Hour)
	path := t.TempDir() + "/auth.json"
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}
	auth := &coreauth.Auth{
		ID:       "a-2",
		Provider: "codex",
		Disabled: true,
		Attributes: map[string]string{
			"path": path,
		},
		Metadata: map[string]any{
			"expires_at": expiresAt.Format(time.RFC3339),
			coreauth.PersistedRuntimeStateMetadataKey: map[string]any{
				"auths": map[string]any{
					"a-2": map[string]any{
						"request_count": 3,
					},
				},
			},
		},
		Quota: coreauth.QuotaState{
			Exceeded:      true,
			Reason:        "quota_5h",
			NextRecoverAt: now.Add(2 * time.Hour),
			BackoffLevel:  2,
		},
	}

	entry := h.buildAuthFileEntry(auth)
	if entry == nil {
		t.Fatalf("buildAuthFileEntry() returned nil")
	}
	if got := entry["state"]; got != "disabled" {
		t.Fatalf("state = %v, want %q", got, "disabled")
	}
	if got := entry["quota_checked"]; got != true {
		t.Fatalf("quota_checked = %v, want true", got)
	}
	if got := entry["quota_level"]; got != "medium" {
		t.Fatalf("quota_level = %v, want %q", got, "medium")
	}
	if got := entry["quota_backoff_level"]; got != 2 {
		t.Fatalf("quota_backoff_level = %v, want 2", got)
	}
	gotExpiresAt, ok := entry["expires_at"].(time.Time)
	if !ok || !gotExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v, want %v", entry["expires_at"], expiresAt)
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
