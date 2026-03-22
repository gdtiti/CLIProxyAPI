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
	if updated.Disabled {
		t.Fatalf("auth.Disabled = true, want false")
	}
	if updated.Status != StatusDisabled {
		t.Fatalf("auth.Status = %q, want %q", updated.Status, StatusDisabled)
	}
	if updated.StatusMessage != "quota exhausted (weekly window)" {
		t.Fatalf("auth status message = %q, want %q", updated.StatusMessage, "quota exhausted (weekly window)")
	}
	if updated.NextRetryAfter.IsZero() {
		t.Fatalf("auth.NextRetryAfter = zero, want non-zero")
	}
	if diff := updated.NextRetryAfter.Sub(time.Now().Add(codex429TemporaryDisableDuration)); diff < -5*time.Second || diff > 5*time.Second {
		t.Fatalf("auth.NextRetryAfter = %v, want about now+%v", updated.NextRetryAfter, codex429TemporaryDisableDuration)
	}
	until := temporaryDisableUntil(updated)
	if until.IsZero() {
		t.Fatalf("temporary disable until = zero, want non-zero")
	}
	if diff := until.Sub(time.Now().Add(codex429TemporaryDisableDuration)); diff < -5*time.Second || diff > 5*time.Second {
		t.Fatalf("temporary disable until = %v, want about now+%v", until, codex429TemporaryDisableDuration)
	}
	if modelState.NextRetryAfter.IsZero() {
		t.Fatalf("model state next retry = zero, want non-zero")
	}
	if globalState.NextRetryAfter.IsZero() {
		t.Fatalf("global state next retry = zero, want non-zero")
	}
}

func TestManagerExecute_CodexUnauthorizedDoesNotImmediatelyDisableAuth(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, nil, nil)
	mgr.RegisterExecutor(&testCodexQuotaExecutor{err: testQuotaStatusErr{
		code: http.StatusUnauthorized,
		msg:  "unauthorized",
	}})

	auth := &Auth{ID: "codex-auth-unauthorized", Provider: "codex"}
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
	if updated.Disabled {
		t.Fatalf("auth.Disabled = true, want false")
	}
	if updated.Status != StatusError {
		t.Fatalf("auth.Status = %q, want %q", updated.Status, StatusError)
	}
	modelState := updated.ModelStates["gpt-5"]
	if modelState == nil {
		t.Fatalf("missing model state for gpt-5")
	}
	if modelState.Status != StatusError {
		t.Fatalf("model state status = %q, want %q", modelState.Status, StatusError)
	}
	if modelState.StatusMessage != "unauthorized" {
		t.Fatalf("model state status message = %q, want %q", modelState.StatusMessage, "unauthorized")
	}
}

func TestManagerGetByID_ClearsExpiredTemporaryDisableFromSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Now()
	expiredAt := now.Add(-1 * time.Minute)
	mgr := NewManager(nil, nil, nil)
	auth := &Auth{
		ID:             "codex-auth-expired-temp-disable",
		Provider:       "codex",
		Status:         StatusDisabled,
		StatusMessage:  "quota exhausted (5h window)",
		Unavailable:    true,
		NextRetryAfter: expiredAt,
		Metadata: map[string]any{
			temporaryDisableUntilMetadataKey:  expiredAt.UTC().Format(time.RFC3339Nano),
			temporaryDisableReasonMetadataKey: temporaryDisableReasonCodex429,
		},
	}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	updated, ok := mgr.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("GetByID() missing auth")
	}
	if updated.Status != StatusActive {
		t.Fatalf("auth.Status = %q, want %q", updated.Status, StatusActive)
	}
	if updated.StatusMessage != "" {
		t.Fatalf("auth.StatusMessage = %q, want empty", updated.StatusMessage)
	}
	if updated.Unavailable {
		t.Fatalf("auth.Unavailable = true, want false")
	}
	if _, exists := updated.Metadata[temporaryDisableUntilMetadataKey]; exists {
		t.Fatalf("temporary disable metadata still present after snapshot normalization")
	}
}
