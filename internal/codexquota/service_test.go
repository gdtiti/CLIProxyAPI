package codexquota

import (
	"context"
	"net/http"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestServicePersistsSnapshotsUsageAndEvents(t *testing.T) {
	t.Parallel()

	authDir := t.TempDir()
	service, err := NewService(authDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	manager := coreauth.NewManager(nil, nil, service.Hook())
	service.SetAuthManager(manager)

	auth := &coreauth.Auth{
		ID:       "codex-auth-1",
		Provider: "codex",
		FileName: "C:/Users/test/auths/alpha.json",
		Status:   coreauth.StatusActive,
		Metadata: map[string]any{
			"auth_method": "oauth",
			"email":       "alpha@example.com",
		},
	}
	if _, errRegister := manager.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("Register() error = %v", errRegister)
	}

	authIndex := auth.EnsureIndex()
	service.ApplyUsage(coreusage.Record{
		Provider:    "codex",
		AuthID:      auth.ID,
		AuthIndex:   authIndex,
		RequestedAt: time.Now().UTC(),
		Detail: coreusage.Detail{
			InputTokens:     120,
			OutputTokens:    30,
			CachedTokens:    25,
			ReasoningTokens: 10,
			TotalTokens:     160,
		},
	})

	retryAfter := 2 * time.Minute
	manager.MarkResult(context.Background(), coreauth.Result{
		AuthID:     auth.ID,
		Provider:   "codex",
		Model:      "gpt-5-codex",
		Success:    false,
		RetryAfter: &retryAfter,
		Error: &coreauth.Error{
			Message:    "quota exhausted",
			HTTPStatus: http.StatusTooManyRequests,
		},
	})

	snapshot, ok := service.GetSnapshot(authIndex)
	if !ok {
		t.Fatal("GetSnapshot() returned false")
	}
	if !snapshot.QuotaExceeded {
		t.Fatalf("snapshot.QuotaExceeded = false, want true")
	}
	if snapshot.FileName != "alpha.json" {
		t.Fatalf("snapshot.FileName = %q, want %q", snapshot.FileName, "alpha.json")
	}
	if snapshot.Usage.RequestCount != 1 {
		t.Fatalf("snapshot.Usage.RequestCount = %d, want 1", snapshot.Usage.RequestCount)
	}
	if snapshot.Usage.TotalTokens != 160 {
		t.Fatalf("snapshot.Usage.TotalTokens = %d, want 160", snapshot.Usage.TotalTokens)
	}
	if snapshot.Usage.AvgTotalTokens != 160 {
		t.Fatalf("snapshot.Usage.AvgTotalTokens = %v, want 160", snapshot.Usage.AvgTotalTokens)
	}

	events := service.ListEvents(authIndex, 10)
	if len(events) == 0 {
		t.Fatal("ListEvents() returned no events")
	}
	if events[0].EventType != eventTypeQuotaExceeded {
		t.Fatalf("events[0].EventType = %q, want %q", events[0].EventType, eventTypeQuotaExceeded)
	}
	if events[0].RequestCount != 1 {
		t.Fatalf("events[0].RequestCount = %d, want 1", events[0].RequestCount)
	}
	if events[0].RecoverAt == nil {
		t.Fatal("events[0].RecoverAt = nil, want non-nil")
	}

	manager.MarkResult(context.Background(), coreauth.Result{
		AuthID:   auth.ID,
		Provider: "codex",
		Model:    "gpt-5-codex",
		Success:  true,
	})

	reloaded, err := NewService(authDir)
	if err != nil {
		t.Fatalf("reloading service error = %v", err)
	}
	reloadedSnapshot, ok := reloaded.GetSnapshot(authIndex)
	if !ok {
		t.Fatal("reloaded GetSnapshot() returned false")
	}
	if reloadedSnapshot.QuotaExceeded {
		t.Fatalf("reloadedSnapshot.QuotaExceeded = true, want false")
	}
	foundRecovered := false
	for _, event := range reloaded.ListEvents(authIndex, 20) {
		if event.EventType == eventTypeQuotaRecovered {
			foundRecovered = true
			break
		}
	}
	if !foundRecovered {
		t.Fatal("quota_recovered event not found after reload")
	}
}
