package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractAccessToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata map[string]any
		expected string
	}{
		{
			"antigravity top-level access_token",
			map[string]any{"access_token": "tok-abc"},
			"tok-abc",
		},
		{
			"gemini nested token.access_token",
			map[string]any{
				"token": map[string]any{"access_token": "tok-nested"},
			},
			"tok-nested",
		},
		{
			"top-level takes precedence over nested",
			map[string]any{
				"access_token": "tok-top",
				"token":        map[string]any{"access_token": "tok-nested"},
			},
			"tok-top",
		},
		{
			"empty metadata",
			map[string]any{},
			"",
		},
		{
			"whitespace-only access_token",
			map[string]any{"access_token": "   "},
			"",
		},
		{
			"wrong type access_token",
			map[string]any{"access_token": 12345},
			"",
		},
		{
			"token is not a map",
			map[string]any{"token": "not-a-map"},
			"",
		},
		{
			"nested whitespace-only",
			map[string]any{
				"token": map[string]any{"access_token": "  "},
			},
			"",
		},
		{
			"fallback to nested when top-level empty",
			map[string]any{
				"access_token": "",
				"token":        map[string]any{"access_token": "tok-fallback"},
			},
			"tok-fallback",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractAccessToken(tt.metadata)
			if got != tt.expected {
				t.Errorf("extractAccessToken() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFileTokenStoreReadAuthFile_AppliesPersistedRuntimeState(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "codex-auth.json")
	nextRetry := time.Now().Add(2 * time.Hour).UTC().Round(0)
	payload := map[string]any{
		"type": "codex",
		"cliproxy_runtime_state": map[string]any{
			"auths": map[string]any{
				"codex-auth.json": map[string]any{
					"status":           "error",
					"status_message":   "quota exhausted (weekly window)",
					"next_retry_after": nextRetry.Format(time.RFC3339Nano),
					"quota": map[string]any{
						"exceeded":        true,
						"reason":          "quota_weekly",
						"next_recover_at": nextRetry.Format(time.RFC3339Nano),
					},
				},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	store := NewFileTokenStore()
	auth, err := store.readAuthFile(path, tempDir)
	if err != nil {
		t.Fatalf("readAuthFile() error = %v", err)
	}
	if auth == nil {
		t.Fatal("readAuthFile() returned nil auth")
	}
	if !auth.NextRetryAfter.Equal(nextRetry) {
		t.Fatalf("NextRetryAfter = %v, want %v", auth.NextRetryAfter, nextRetry)
	}
	if !auth.Quota.Exceeded {
		t.Fatalf("Quota.Exceeded = false, want true")
	}
	if auth.Quota.Reason != "quota_weekly" {
		t.Fatalf("Quota.Reason = %q, want %q", auth.Quota.Reason, "quota_weekly")
	}
}
