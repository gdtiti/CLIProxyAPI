package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type backgroundRefreshTestRuntime struct {
	lead time.Duration
}

type backgroundRefreshNoopExecutor struct {
	provider string
}

func (e backgroundRefreshNoopExecutor) Identifier() string {
	return e.provider
}

func (backgroundRefreshNoopExecutor) Execute(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (backgroundRefreshNoopExecutor) ExecuteStream(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, nil
}

func (backgroundRefreshNoopExecutor) Refresh(context.Context, *Auth) (*Auth, error) {
	return nil, nil
}

func (backgroundRefreshNoopExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (backgroundRefreshNoopExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, nil
}

func (r backgroundRefreshTestRuntime) RefreshLead() *time.Duration {
	if r.lead <= 0 {
		return nil
	}
	lead := r.lead
	return &lead
}

func TestManager_ShouldRefresh_CodexColdAuthSkipsBackgroundRefresh(t *testing.T) {
	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	now := time.Now().UTC()
	auth := codexBackgroundRefreshTestAuth("codex-cold", now.Add(4*24*time.Hour))

	if manager.shouldRefresh(auth, now) {
		t.Fatal("shouldRefresh() = true, want false for cold codex auth")
	}
}

func TestManager_ShouldRefresh_CodexWarmAuthAllowsBackgroundRefresh(t *testing.T) {
	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	now := time.Now().UTC()
	auth := codexBackgroundRefreshTestAuth("codex-warm", now.Add(4*24*time.Hour))
	auth.WarmUntil = now.Add(10 * time.Minute)

	if !manager.shouldRefresh(auth, now) {
		t.Fatal("shouldRefresh() = false, want true for warm codex auth")
	}
}

func TestManager_ShouldRefresh_CodexResidentAuthAllowsBackgroundRefresh(t *testing.T) {
	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	now := time.Now().UTC()
	auth := codexBackgroundRefreshTestAuth("codex-resident", now.Add(4*24*time.Hour))
	auth.ResidentUntil = now.Add(30 * time.Minute)

	if !manager.shouldRefresh(auth, now) {
		t.Fatal("shouldRefresh() = false, want true for resident codex auth")
	}
}

func TestManager_ShouldRefresh_CodexDisabledAuthStillSkipsBackgroundRefresh(t *testing.T) {
	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	now := time.Now().UTC()
	auth := codexBackgroundRefreshTestAuth("codex-disabled", now.Add(4*24*time.Hour))
	auth.Disabled = true
	auth.WarmUntil = now.Add(10 * time.Minute)

	if manager.shouldRefresh(auth, now) {
		t.Fatal("shouldRefresh() = true, want false for disabled codex auth")
	}
}

func TestManager_MarkResult_SuccessWarmsCodexAuth(t *testing.T) {
	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	auth := codexBackgroundRefreshTestAuth("codex-success", time.Now().UTC().Add(4*24*time.Hour))
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: "codex",
		Model:    "gpt-5",
		Success:  true,
	})

	updated, ok := manager.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("GetByID(%q) missing updated auth", auth.ID)
	}
	if updated.LastUsedAt.IsZero() {
		t.Fatal("LastUsedAt is zero, want non-zero after successful request")
	}
	if !updated.WarmUntil.After(updated.LastUsedAt) {
		t.Fatalf("WarmUntil = %v, want > LastUsedAt %v", updated.WarmUntil, updated.LastUsedAt)
	}
	if updated.WarmUntil.Sub(updated.LastUsedAt) < codexBackgroundRefreshWarmWindow {
		t.Fatalf("WarmUntil-LastUsedAt = %v, want >= %v", updated.WarmUntil.Sub(updated.LastUsedAt), codexBackgroundRefreshWarmWindow)
	}
}

func TestManager_Update_PreservesBackgroundRefreshHeat(t *testing.T) {
	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	now := time.Now().UTC()
	auth := codexBackgroundRefreshTestAuth("codex-preserve", now.Add(4*24*time.Hour))
	auth.LastUsedAt = now
	auth.WarmUntil = now.Add(20 * time.Minute)
	auth.ResidentUntil = now.Add(40 * time.Minute)
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	reloaded := codexBackgroundRefreshTestAuth(auth.ID, now.Add(5*24*time.Hour))
	if _, err := manager.Update(context.Background(), reloaded); err != nil {
		t.Fatalf("update auth: %v", err)
	}

	updated, ok := manager.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("GetByID(%q) missing updated auth", auth.ID)
	}
	if updated.LastUsedAt.IsZero() {
		t.Fatal("LastUsedAt lost after update")
	}
	if !updated.WarmUntil.Equal(auth.WarmUntil) {
		t.Fatalf("WarmUntil = %v, want %v", updated.WarmUntil, auth.WarmUntil)
	}
	if !updated.ResidentUntil.Equal(auth.ResidentUntil) {
		t.Fatalf("ResidentUntil = %v, want %v", updated.ResidentUntil, auth.ResidentUntil)
	}
}

func TestManager_PickNextLegacy_SimHashSelectionMarksResident(t *testing.T) {
	manager := NewManager(nil, NewSimHashSelector(internalconfig.RoutingSimHashConfig{PoolSize: 2}), nil)
	manager.RegisterExecutor(backgroundRefreshNoopExecutor{provider: "codex"})

	first := codexBackgroundRefreshTestAuth("codex-a", time.Now().UTC().Add(4*24*time.Hour))
	second := codexBackgroundRefreshTestAuth("codex-b", time.Now().UTC().Add(4*24*time.Hour))
	if _, err := manager.Register(context.Background(), first); err != nil {
		t.Fatalf("register first auth: %v", err)
	}
	if _, err := manager.Register(context.Background(), second); err != nil {
		t.Fatalf("register second auth: %v", err)
	}

	selected, _, err := manager.pickNextLegacy(context.Background(), "codex", "", cliproxyexecutor.Options{}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("pickNextLegacy error: %v", err)
	}
	if selected == nil {
		t.Fatal("selected auth is nil")
	}

	updated, ok := manager.GetByID(selected.ID)
	if !ok || updated == nil {
		t.Fatalf("GetByID(%q) missing updated auth", selected.ID)
	}
	if updated.ResidentUntil.IsZero() {
		t.Fatal("ResidentUntil is zero, want non-zero after simhash selection")
	}
	if !updated.ResidentUntil.After(time.Now()) {
		t.Fatalf("ResidentUntil = %v, want > now", updated.ResidentUntil)
	}
}

func TestManager_PickNextLegacy_SimHashPoolMembersAllBecomeResident(t *testing.T) {
	manager := NewManager(nil, NewSimHashSelector(internalconfig.RoutingSimHashConfig{PoolSize: 2}), nil)
	manager.RegisterExecutor(backgroundRefreshNoopExecutor{provider: "codex"})

	authA := codexBackgroundRefreshTestAuth("codex-a", time.Now().UTC().Add(4*24*time.Hour))
	authB := codexBackgroundRefreshTestAuth("codex-b", time.Now().UTC().Add(4*24*time.Hour))
	if _, err := manager.Register(context.Background(), authA); err != nil {
		t.Fatalf("register authA: %v", err)
	}
	if _, err := manager.Register(context.Background(), authB); err != nil {
		t.Fatalf("register authB: %v", err)
	}

	if _, _, err := manager.pickNextLegacy(context.Background(), "codex", "", cliproxyexecutor.Options{}, map[string]struct{}{}); err != nil {
		t.Fatalf("first pickNextLegacy error: %v", err)
	}
	if _, _, err := manager.pickNextLegacy(context.Background(), "codex", "", cliproxyexecutor.Options{}, map[string]struct{}{}); err != nil {
		t.Fatalf("second pickNextLegacy error: %v", err)
	}

	updatedA, _ := manager.GetByID(authA.ID)
	updatedB, _ := manager.GetByID(authB.ID)
	if updatedA == nil || updatedB == nil {
		t.Fatal("expected both auths to remain registered")
	}
	if updatedA.ResidentUntil.IsZero() || updatedB.ResidentUntil.IsZero() {
		t.Fatalf("resident bridge incomplete: a=%v b=%v", updatedA.ResidentUntil, updatedB.ResidentUntil)
	}
}

func TestApplyBackgroundRefreshHintsLocked_OnlyMarksMatchingProvider(t *testing.T) {
	now := time.Now().UTC()
	auths := map[string]*Auth{
		"codex-a":  codexBackgroundRefreshTestAuth("codex-a", now.Add(4*24*time.Hour)),
		"claude-a": {ID: "claude-a", Provider: "claude"},
	}

	applyBackgroundRefreshHintsLocked(auths, "codex", now, BackgroundRefreshHints{
		ResidentAuthIDs: []string{"codex-a", "claude-a"},
	})

	if auths["codex-a"].ResidentUntil.IsZero() {
		t.Fatal("codex auth was not marked resident")
	}
	if !auths["claude-a"].ResidentUntil.IsZero() {
		t.Fatalf("claude auth ResidentUntil = %v, want zero", auths["claude-a"].ResidentUntil)
	}
}

func codexBackgroundRefreshTestAuth(id string, expiry time.Time) *Auth {
	return &Auth{
		ID:       id,
		Provider: "codex",
		Runtime:  backgroundRefreshTestRuntime{lead: 5 * 24 * time.Hour},
		Metadata: map[string]any{
			"expired": expiry.UTC().Format(time.RFC3339Nano),
		},
	}
}
