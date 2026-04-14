package auth

import (
	"testing"
	"time"
)

func TestRecordRefreshHistory_TrimsOldEntries(t *testing.T) {
	t.Parallel()

	auth := &Auth{
		ID:       "auth-1",
		Provider: "codex",
		Metadata: map[string]any{},
	}
	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)

	for i := 0; i < persistedRefreshHistoryLimit+5; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		RecordRefreshHistory(auth, at, "auto_refresh", "success", "", at.Add(24*time.Hour))
	}

	history := ListRefreshHistory(auth)
	if len(history) != persistedRefreshHistoryLimit {
		t.Fatalf("history length = %d, want %d", len(history), persistedRefreshHistoryLimit)
	}
	if !history[0].At.Equal(base.Add(5 * time.Minute)) {
		t.Fatalf("first history at = %v, want %v", history[0].At, base.Add(5*time.Minute))
	}
	if !history[len(history)-1].At.Equal(base.Add(time.Duration(persistedRefreshHistoryLimit+4) * time.Minute)) {
		t.Fatalf("last history at = %v, want %v", history[len(history)-1].At, base.Add(time.Duration(persistedRefreshHistoryLimit+4)*time.Minute))
	}
}

func TestSyncRuntimePersistence_PreservesRefreshHistory(t *testing.T) {
	t.Parallel()

	auth := &Auth{
		ID:       "auth-2",
		Provider: "codex",
		Metadata: map[string]any{},
		Status:   StatusError,
		Quota: QuotaState{
			Exceeded:      true,
			Reason:        "quota",
			NextRecoverAt: time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		},
	}
	now := time.Date(2026, 4, 12, 11, 0, 0, 0, time.UTC)
	expiry := now.Add(2 * time.Hour)

	RecordRefreshHistory(auth, now.Add(-time.Hour), "auto_refresh", "error", "expired", time.Time{})
	RecordRefreshHistory(auth, now, "auto_refresh", "success", "", expiry)

	if !SyncRuntimePersistence(auth, now) {
		t.Fatal("SyncRuntimePersistence returned false, want true")
	}

	history := ListRefreshHistory(auth)
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
	if history[0].Result != "error" || history[1].Result != "success" {
		t.Fatalf("history results = %#v, want [error success]", history)
	}
	if !history[1].ExpiresAt.Equal(expiry) {
		t.Fatalf("history expires_at = %v, want %v", history[1].ExpiresAt, expiry)
	}
}
