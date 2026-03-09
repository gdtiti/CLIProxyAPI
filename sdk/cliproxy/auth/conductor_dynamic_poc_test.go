package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type blockingRefreshExecutor struct {
	entered chan *Auth
	release chan struct{}
}

func (e *blockingRefreshExecutor) Identifier() string { return "codex" }

func (e *blockingRefreshExecutor) Execute(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *blockingRefreshExecutor) ExecuteStream(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	chunks := make(chan cliproxyexecutor.StreamChunk)
	close(chunks)
	return &cliproxyexecutor.StreamResult{Chunks: chunks}, nil
}

func (e *blockingRefreshExecutor) Refresh(_ context.Context, auth *Auth) (*Auth, error) {
	e.entered <- auth.Clone()
	<-e.release
	return auth, nil
}

func (e *blockingRefreshExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *blockingRefreshExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestDynamicPOC_RefreshPreservesMarkResultState(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, nil, nil)
	exec := &blockingRefreshExecutor{
		entered: make(chan *Auth, 1),
		release: make(chan struct{}),
	}
	mgr.RegisterExecutor(exec)

	auth := &Auth{ID: "race-auth", Provider: "codex", Metadata: map[string]any{"account_id": "race"}}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	done := make(chan struct{})
	go func() {
		mgr.refreshAuth(context.Background(), auth.ID)
		close(done)
	}()

	stale := <-exec.entered
	if stale == nil {
		t.Fatal("refresh did not capture auth snapshot")
	}

	retry := 15 * time.Minute
	mgr.MarkResult(context.Background(), Result{
		AuthID:      auth.ID,
		Provider:    "codex",
		Model:       "gpt-5",
		Success:     false,
		QuotaWindow: "weekly",
		RetryAfter:  &retry,
		Error:       &Error{HTTPStatus: 429, Message: "quota"},
	})

	before, ok := mgr.GetByID(auth.ID)
	if !ok || before == nil {
		t.Fatalf("auth missing after MarkResult")
	}
	beforeModel := before.ModelStates["gpt-5"]
	beforeGlobal := before.ModelStates[globalModelStateKey]
	if beforeModel == nil || beforeGlobal == nil {
		t.Fatalf("expected model/global state after MarkResult, got model=%v global=%v", beforeModel, beforeGlobal)
	}
	t.Logf("before refresh update: unavailable=%v next_retry=%v quota_exceeded=%v reason=%s", beforeModel.Unavailable, beforeModel.NextRetryAfter, beforeModel.Quota.Exceeded, beforeModel.Quota.Reason)
	if !before.Quota.Exceeded {
		t.Fatalf("expected aggregated auth quota exceeded=true before refresh overwrite")
	}

	close(exec.release)
	<-done

	after, ok := mgr.GetByID(auth.ID)
	if !ok || after == nil {
		t.Fatalf("auth missing after refresh")
	}
	afterModel := after.ModelStates["gpt-5"]
	afterGlobal := after.ModelStates[globalModelStateKey]
	t.Logf("after refresh update: model=%v global=%v auth_quota_exceeded=%v auth_next_retry=%v", afterModel, afterGlobal, after.Quota.Exceeded, after.NextRetryAfter)

	if afterModel == nil || afterGlobal == nil {
		t.Fatalf("refresh should preserve model/global state, got model=%v global=%v", afterModel, afterGlobal)
	}
	if !after.Quota.Exceeded {
		t.Fatalf("refresh should preserve aggregated auth quota state")
	}
	if after.NextRetryAfter.IsZero() {
		t.Fatalf("refresh should preserve aggregated next retry state")
	}
}

func TestDynamicPOC_403StateIsStillSelectableWhenNextRetryZero(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, &FillFirstSelector{}, nil)
	auth := &Auth{ID: "forbidden-auth", Provider: "codex", Metadata: map[string]any{"account_id": "f"}}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	mgr.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: "codex",
		Model:    "gpt-5",
		Success:  false,
		Error:    &Error{HTTPStatus: 403, Message: "forbidden"},
	})

	updated, ok := mgr.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("auth missing after 403")
	}
	state := updated.ModelStates["gpt-5"]
	if state == nil {
		t.Fatalf("missing model state after 403")
	}
	t.Logf("403 state: unavailable=%v next_retry_is_zero=%v quota_exceeded=%v", state.Unavailable, state.NextRetryAfter.IsZero(), state.Quota.Exceeded)
	if !state.Unavailable || !state.NextRetryAfter.IsZero() {
		t.Fatalf("unexpected 403 model state: unavailable=%v next_retry=%v", state.Unavailable, state.NextRetryAfter)
	}

	selector := &FillFirstSelector{}
	picked1, err := selector.Pick(context.Background(), "codex", "gpt-5", cliproxyexecutor.Options{}, []*Auth{updated})
	if err != nil {
		t.Fatalf("first pick error: %v", err)
	}
	picked2, err := selector.Pick(context.Background(), "codex", "gpt-5", cliproxyexecutor.Options{}, []*Auth{updated})
	if err != nil {
		t.Fatalf("second pick error: %v", err)
	}
	if picked1 == nil || picked2 == nil {
		t.Fatalf("expected forbidden auth to be selected, got picked1=%v picked2=%v", picked1, picked2)
	}
	t.Logf("selector reused 403 auth: first=%s second=%s", picked1.ID, picked2.ID)
}
