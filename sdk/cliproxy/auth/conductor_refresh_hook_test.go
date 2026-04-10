package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type refreshErrorStore struct {
	saved []*Auth
}

func (s *refreshErrorStore) List(context.Context) ([]*Auth, error) {
	return nil, nil
}

func (s *refreshErrorStore) Save(_ context.Context, auth *Auth) (string, error) {
	s.saved = append(s.saved, auth.Clone())
	return auth.ID, nil
}

func (s *refreshErrorStore) Delete(context.Context, string) error {
	return nil
}

type refreshErrorHook struct {
	NoopHook
	updated []*Auth
}

func (h *refreshErrorHook) OnAuthUpdated(_ context.Context, auth *Auth) {
	h.updated = append(h.updated, auth.Clone())
}

type failingRefreshExecutor struct{}

func (failingRefreshExecutor) Identifier() string { return "codex" }
func (failingRefreshExecutor) Execute(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, errors.New("not implemented")
}
func (failingRefreshExecutor) ExecuteStream(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, errors.New("not implemented")
}
func (failingRefreshExecutor) Refresh(context.Context, *Auth) (*Auth, error) {
	return nil, errors.New("refresh failed")
}
func (failingRefreshExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, errors.New("not implemented")
}
func (failingRefreshExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, errors.New("not implemented")
}

func TestManagerRefreshAuthFailurePersistsAndCallsHook(t *testing.T) {
	t.Parallel()

	store := &refreshErrorStore{}
	hook := &refreshErrorHook{}
	manager := NewManager(store, nil, hook)
	manager.RegisterExecutor(failingRefreshExecutor{})

	auth := &Auth{
		ID:       "codex-auth-1",
		Provider: "codex",
		Status:   StatusActive,
		Metadata: map[string]any{
			"access_token": "test-token",
		},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	manager.refreshAuth(context.Background(), auth.ID)

	updated, ok := manager.GetByID(auth.ID)
	if !ok {
		t.Fatal("GetByID() returned false")
	}
	if updated.LastError == nil || updated.LastError.Message != "refresh failed" {
		t.Fatalf("updated.LastError = %#v, want refresh failed", updated.LastError)
	}
	if updated.NextRefreshAfter.IsZero() {
		t.Fatal("updated.NextRefreshAfter = zero, want non-zero")
	}
	if len(hook.updated) == 0 {
		t.Fatal("OnAuthUpdated was not called")
	}
	if hook.updated[0].LastError == nil || hook.updated[0].LastError.Message != "refresh failed" {
		t.Fatalf("hook.updated[0].LastError = %#v, want refresh failed", hook.updated[0].LastError)
	}
	if len(store.saved) == 0 {
		t.Fatal("store.Save was not called")
	}
	lastSaved := store.saved[len(store.saved)-1]
	if lastSaved.LastError == nil || !strings.Contains(lastSaved.LastError.Message, "refresh failed") {
		t.Fatalf("lastSaved.LastError = %#v, want refresh failed", lastSaved.LastError)
	}
}
