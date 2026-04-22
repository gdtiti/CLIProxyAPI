package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type refreshHistoryStore struct {
	mu     sync.Mutex
	items  map[string]*Auth
	saves  int
	lastID string
}

func (s *refreshHistoryStore) List(context.Context) ([]*Auth, error) { return nil, nil }

func (s *refreshHistoryStore) Save(_ context.Context, auth *Auth) (string, error) {
	if auth == nil {
		return "", nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = make(map[string]*Auth)
	}
	s.items[auth.ID] = auth.Clone()
	s.saves++
	s.lastID = auth.ID
	return auth.ID, nil
}

func (s *refreshHistoryStore) Delete(context.Context, string) error { return nil }

func (s *refreshHistoryStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves = 0
	s.lastID = ""
}

func (s *refreshHistoryStore) saveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saves
}

type refreshHistoryExecutor struct {
	refresh func(context.Context, *Auth) (*Auth, error)
}

func (e *refreshHistoryExecutor) Identifier() string { return "codex" }

func (e *refreshHistoryExecutor) Execute(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *refreshHistoryExecutor) ExecuteStream(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	chunks := make(chan cliproxyexecutor.StreamChunk)
	close(chunks)
	return &cliproxyexecutor.StreamResult{Chunks: chunks}, nil
}

func (e *refreshHistoryExecutor) Refresh(ctx context.Context, auth *Auth) (*Auth, error) {
	if e.refresh != nil {
		return e.refresh(ctx, auth)
	}
	return auth, nil
}

func (e *refreshHistoryExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *refreshHistoryExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestRefreshAuth_RecordsSuccessHistory(t *testing.T) {
	t.Parallel()

	store := &refreshHistoryStore{}
	mgr := NewManager(store, nil, nil)
	mgr.RegisterExecutor(&refreshHistoryExecutor{
		refresh: func(_ context.Context, auth *Auth) (*Auth, error) {
			updated := auth.Clone()
			if updated.Metadata == nil {
				updated.Metadata = make(map[string]any)
			}
			updated.Metadata["expires_at"] = time.Date(2026, 4, 13, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
			return updated, nil
		},
	})

	auth := &Auth{
		ID:       "codex-success",
		Provider: "codex",
		Metadata: map[string]any{"email": "success@example.com"},
	}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}
	store.reset()

	mgr.refreshAuth(context.Background(), auth.ID)

	current, ok := mgr.GetByID(auth.ID)
	if !ok || current == nil {
		t.Fatalf("auth missing after refresh")
	}
	history := ListRefreshHistory(current)
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].Trigger != "auto_refresh" || history[0].Result != "success" {
		t.Fatalf("history entry = %+v, want auto_refresh success", history[0])
	}
	if history[0].ExpiresAt.IsZero() {
		t.Fatalf("history expires_at is zero, want populated")
	}
	if store.saveCount() != 1 {
		t.Fatalf("save count = %d, want 1", store.saveCount())
	}
}

func TestRefreshAuth_RecordsFailureHistoryAndPersists(t *testing.T) {
	t.Parallel()

	store := &refreshHistoryStore{}
	mgr := NewManager(store, nil, nil)
	mgr.RegisterExecutor(&refreshHistoryExecutor{
		refresh: func(context.Context, *Auth) (*Auth, error) {
			return nil, errors.New("refresh failed")
		},
	})

	auth := &Auth{
		ID:       "codex-failure",
		Provider: "codex",
		Metadata: map[string]any{"email": "failure@example.com"},
	}
	if _, err := mgr.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}
	store.reset()

	mgr.refreshAuth(context.Background(), auth.ID)

	current, ok := mgr.GetByID(auth.ID)
	if !ok || current == nil {
		t.Fatalf("auth missing after failed refresh")
	}
	history := ListRefreshHistory(current)
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].Trigger != "auto_refresh" || history[0].Result != "error" {
		t.Fatalf("history entry = %+v, want auto_refresh error", history[0])
	}
	if history[0].Message != "refresh failed" {
		t.Fatalf("history message = %q, want %q", history[0].Message, "refresh failed")
	}
	if current.LastError == nil || current.LastError.Message != "refresh failed" {
		t.Fatalf("last error = %+v, want refresh failed", current.LastError)
	}
	if current.NextRefreshAfter.IsZero() {
		t.Fatalf("next_refresh_after is zero, want backoff")
	}
	if store.saveCount() != 1 {
		t.Fatalf("save count = %d, want 1", store.saveCount())
	}
}
