package auth

import (
	"context"
	"testing"
	"time"
)

func TestManager_ShouldRetryAfterError_RespectsAuthRequestRetryOverride(t *testing.T) {
	m := NewManager(nil, nil, nil)
	m.SetRetryConfig(3, 30*time.Second)

	model := "test-model"
	next := time.Now().Add(5 * time.Second)

	auth := &Auth{
		ID:       "auth-1",
		Provider: "claude",
		Metadata: map[string]any{
			"request_retry": float64(0),
		},
		ModelStates: map[string]*ModelState{
			model: {
				Unavailable:    true,
				Status:         StatusError,
				NextRetryAfter: next,
			},
		},
	}
	if _, errRegister := m.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("register auth: %v", errRegister)
	}

	_, maxWait := m.retrySettings()
	wait, shouldRetry := m.shouldRetryAfterError(&Error{HTTPStatus: 500, Message: "boom"}, 0, []string{"claude"}, model, maxWait)
	if shouldRetry {
		t.Fatalf("expected shouldRetry=false for request_retry=0, got true (wait=%v)", wait)
	}

	auth.Metadata["request_retry"] = float64(1)
	if _, errUpdate := m.Update(context.Background(), auth); errUpdate != nil {
		t.Fatalf("update auth: %v", errUpdate)
	}

	wait, shouldRetry = m.shouldRetryAfterError(&Error{HTTPStatus: 500, Message: "boom"}, 0, []string{"claude"}, model, maxWait)
	if !shouldRetry {
		t.Fatalf("expected shouldRetry=true for request_retry=1, got false")
	}
	if wait <= 0 {
		t.Fatalf("expected wait > 0, got %v", wait)
	}

	_, shouldRetry = m.shouldRetryAfterError(&Error{HTTPStatus: 500, Message: "boom"}, 1, []string{"claude"}, model, maxWait)
	if shouldRetry {
		t.Fatalf("expected shouldRetry=false on attempt=1 for request_retry=1, got true")
	}
}

func TestManager_MarkResult_RespectsAuthDisableCoolingOverride(t *testing.T) {
	prev := quotaCooldownDisabled.Load()
	quotaCooldownDisabled.Store(false)
	t.Cleanup(func() { quotaCooldownDisabled.Store(prev) })

	m := NewManager(nil, nil, nil)

	auth := &Auth{
		ID:       "auth-1",
		Provider: "claude",
		Metadata: map[string]any{
			"disable_cooling": true,
		},
	}
	if _, errRegister := m.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("register auth: %v", errRegister)
	}

	model := "test-model"
	m.MarkResult(context.Background(), Result{
		AuthID:   "auth-1",
		Provider: "claude",
		Model:    model,
		Success:  false,
		Error:    &Error{HTTPStatus: 500, Message: "boom"},
	})

	updated, ok := m.GetByID("auth-1")
	if !ok || updated == nil {
		t.Fatalf("expected auth to be present")
	}
	state := updated.ModelStates[model]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if !state.NextRetryAfter.IsZero() {
		t.Fatalf("expected NextRetryAfter to be zero when disable_cooling=true, got %v", state.NextRetryAfter)
	}
}

func TestManager_MarkResult_Codex429AppliesGlobalCooldown(t *testing.T) {
	m := NewManager(nil, nil, nil)

	auth := &Auth{ID: "auth-1", Provider: "codex"}
	if _, errRegister := m.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("register auth: %v", errRegister)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:   "auth-1",
		Provider: "codex",
		Model:    "gpt-5",
		Success:  false,
		Error:    &Error{HTTPStatus: 429, Message: "usage limit reached"},
		RetryAfter: func() *time.Duration {
			d := 10 * time.Minute
			return &d
		}(),
	})

	updated, ok := m.GetByID("auth-1")
	if !ok || updated == nil {
		t.Fatalf("expected auth to be present")
	}
	state := updated.ModelStates[globalModelStateKey]
	if state == nil {
		t.Fatalf("expected global model state to be present")
	}
	if !state.Unavailable {
		t.Fatalf("expected global model state to be unavailable")
	}
	if !state.Quota.Exceeded {
		t.Fatalf("expected global model state quota exceeded")
	}
	if state.NextRetryAfter.IsZero() {
		t.Fatalf("expected global model state NextRetryAfter to be set")
	}
}

func TestManager_MarkResult_CodexSuccessClearsGlobalCooldown(t *testing.T) {
	m := NewManager(nil, nil, nil)

	now := time.Now()
	auth := &Auth{
		ID:       "auth-1",
		Provider: "codex",
		ModelStates: map[string]*ModelState{
			globalModelStateKey: {
				Status:         StatusError,
				Unavailable:    true,
				NextRetryAfter: now.Add(20 * time.Minute),
				Quota:          QuotaState{Exceeded: true},
			},
			"gpt-5": {
				Status:         StatusError,
				Unavailable:    true,
				NextRetryAfter: now.Add(20 * time.Minute),
				Quota:          QuotaState{Exceeded: true},
			},
		},
	}
	if _, errRegister := m.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("register auth: %v", errRegister)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:   "auth-1",
		Provider: "codex",
		Model:    "gpt-5",
		Success:  true,
	})

	updated, ok := m.GetByID("auth-1")
	if !ok || updated == nil {
		t.Fatalf("expected auth to be present")
	}
	state := updated.ModelStates[globalModelStateKey]
	if state == nil {
		t.Fatalf("expected global model state to be present")
	}
	if state.Unavailable {
		t.Fatalf("expected global model state unavailable=false")
	}
	if state.Quota.Exceeded {
		t.Fatalf("expected global model state quota exceeded=false")
	}
	if !state.NextRetryAfter.IsZero() {
		t.Fatalf("expected global model state NextRetryAfter to be zero, got %v", state.NextRetryAfter)
	}
}

func TestManager_MarkResult_Codex429SetsWindowSpecificStatusAndReason(t *testing.T) {
	m := NewManager(nil, nil, nil)

	auth := &Auth{ID: "auth-1", Provider: "codex"}
	if _, errRegister := m.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("register auth: %v", errRegister)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:      "auth-1",
		Provider:    "codex",
		Model:       "gpt-5",
		Success:     false,
		QuotaWindow: "weekly",
		Error:       &Error{HTTPStatus: 429, Message: "usage limit reached"},
		RetryAfter: func() *time.Duration {
			d := 24 * time.Hour
			return &d
		}(),
	})

	updated, ok := m.GetByID("auth-1")
	if !ok || updated == nil {
		t.Fatalf("expected auth to be present")
	}
	state := updated.ModelStates["gpt-5"]
	if state == nil {
		t.Fatalf("expected model state to be present")
	}
	if state.StatusMessage != "quota exhausted (weekly window)" {
		t.Fatalf("model status message = %q, want %q", state.StatusMessage, "quota exhausted (weekly window)")
	}
	if state.Quota.Reason != "quota_weekly" {
		t.Fatalf("model quota reason = %q, want %q", state.Quota.Reason, "quota_weekly")
	}
	globalState := updated.ModelStates[globalModelStateKey]
	if globalState == nil {
		t.Fatalf("expected global model state to be present")
	}
	if globalState.Quota.Reason != "quota_weekly" {
		t.Fatalf("global quota reason = %q, want %q", globalState.Quota.Reason, "quota_weekly")
	}
	if updated.StatusMessage != "quota exhausted (weekly window)" {
		t.Fatalf("auth status message = %q, want %q", updated.StatusMessage, "quota exhausted (weekly window)")
	}
}

func TestManager_MarkResult_UpdatesRoundRobinSelectorHealth(t *testing.T) {
	selector := &RoundRobinSelector{}
	m := NewManager(nil, selector, nil)

	auth := &Auth{ID: "auth-1", Provider: "codex"}
	if _, errRegister := m.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("register auth: %v", errRegister)
	}

	m.MarkResult(context.Background(), Result{AuthID: "auth-1", Provider: "codex", Model: "gpt-5", Success: true})

	selector.mu.Lock()
	score, ok := selector.successScore["auth-1"]
	selector.mu.Unlock()
	if !ok {
		t.Fatalf("expected selector success score entry for auth-1")
	}
	if score <= selectorSuccessScoreDefault {
		t.Fatalf("expected score > default after success, got %v", score)
	}
}
