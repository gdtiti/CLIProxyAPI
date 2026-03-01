package auth

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type testQuotaStatusErr struct {
	code   int
	msg    string
	retry  time.Duration
	window string
}

func (e testQuotaStatusErr) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("status %d", e.code)
}

func (e testQuotaStatusErr) StatusCode() int            { return e.code }
func (e testQuotaStatusErr) RetryAfter() *time.Duration { d := e.retry; return &d }
func (e testQuotaStatusErr) CooldownWindow() string     { return e.window }

type testCodexQuotaExecutor struct {
	err error
}

func (e *testCodexQuotaExecutor) Identifier() string { return "codex" }

func (e *testCodexQuotaExecutor) Execute(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, e.err
}

func (e *testCodexQuotaExecutor) ExecuteStream(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, e.err
}

func (e *testCodexQuotaExecutor) Refresh(context.Context, *Auth) (*Auth, error) { return nil, nil }

func (e *testCodexQuotaExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, e.err
}

func (e *testCodexQuotaExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, e.err
}

func TestManagerExecute_CodexQuotaWindowPropagatesToAuthState(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, nil, nil)
	mgr.RegisterExecutor(&testCodexQuotaExecutor{err: testQuotaStatusErr{
		code:   http.StatusTooManyRequests,
		msg:    "usage limit reached",
		retry:  6 * time.Hour,
		window: "weekly",
	}})

	auth := &Auth{ID: "codex-auth-int", Provider: "codex"}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient(auth.ID, "codex", []*registry.ModelInfo{{ID: "gpt-5", Type: "codex"}})
	defer reg.UnregisterClient(auth.ID)

	_, err := mgr.Execute(context.Background(), []string{"codex"}, cliproxyexecutor.Request{Model: "gpt-5", Payload: []byte(`{}`)}, cliproxyexecutor.Options{})
	if err == nil {
		t.Fatalf("Execute() error = nil, want non-nil")
	}

	updated, ok := mgr.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("GetByID() missing auth")
	}
	if updated.StatusMessage != "quota exhausted (weekly window)" {
		t.Fatalf("auth status message = %q, want %q", updated.StatusMessage, "quota exhausted (weekly window)")
	}
	modelState := updated.ModelStates["gpt-5"]
	if modelState == nil {
		t.Fatalf("missing model state for gpt-5")
	}
	if modelState.Quota.Reason != "quota_weekly" {
		t.Fatalf("model quota reason = %q, want %q", modelState.Quota.Reason, "quota_weekly")
	}
	globalState := updated.ModelStates[globalModelStateKey]
	if globalState == nil {
		t.Fatalf("missing global model state")
	}
	if globalState.Quota.Reason != "quota_weekly" {
		t.Fatalf("global quota reason = %q, want %q", globalState.Quota.Reason, "quota_weekly")
	}
}
