package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

// TestCodexExecutor_429FetchesUsageAPI verifies that when Codex returns 429 with
// "Rate limit exceeded" (no usage_limit_reached in body), the executor still
// fetches the usage API to determine five_hour/weekly and set precise recovery time.
func TestCodexExecutor_429FetchesUsageAPI(t *testing.T) {
	t.Parallel()

	// Usage API response: five_hour window reached, reset in 20 minutes
	usageBody := []byte(`{
		"rate_limit": {
			"allowed": false,
			"limit_reached": true,
			"primary_window": {"limit_window_seconds": 18000, "used_percent": 100, "reset_after_seconds": 1200},
			"secondary_window": {"limit_window_seconds": 604800, "used_percent": 30, "reset_after_seconds": 600000}
		}
	}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/backend-api/wham/usage":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(usageBody)
		case r.Method == http.MethodPost && (r.URL.Path == "/backend-api/codex/responses" || r.URL.Path == "/backend-api/codex/responses/compact"):
			// Simulate Codex 429 "Rate limit exceeded" - no usage_limit_reached in body
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"detail":"Rate limit exceeded"}`))
		default:
			t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	auth := &cliproxyauth.Auth{
		ID:       "codex-429-test",
		Provider: "codex",
		Attributes: map[string]string{
			"api_key":  "test-key",
			"base_url": server.URL + "/backend-api/codex",
		},
		Metadata: map[string]any{
			"account_id": "test-account",
		},
	}

	exec := NewCodexExecutor(&config.Config{})
	ctx := context.Background()
	req := cliproxyexecutor.Request{
		Model:   "gpt-5",
		Payload: []byte(`{"input":[{"type":"message","content":[{"type":"text","text":"hi"}]}]}`),
	}
	opts := cliproxyexecutor.Options{SourceFormat: "codex"}

	_, err := exec.Execute(ctx, auth, req, opts)
	if err == nil {
		t.Fatalf("Execute() err = nil, want non-nil")
	}

	// Verify statusErr has RetryAfter and CooldownWindow from usage API
	type statusErr interface {
		StatusCode() int
		RetryAfter() *time.Duration
		CooldownWindow() string
	}
	se, ok := err.(statusErr)
	if !ok {
		t.Fatalf("error %T does not implement statusErr", err)
	}
	if se.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("StatusCode() = %d, want %d", se.StatusCode(), http.StatusTooManyRequests)
	}
	if se.RetryAfter() == nil {
		t.Fatalf("RetryAfter() = nil, want non-nil (from usage API)")
	}
	if got := *se.RetryAfter(); got != 1200*time.Second {
		t.Fatalf("RetryAfter() = %v, want 20m", got)
	}
	if got := se.CooldownWindow(); got != "five_hour" {
		t.Fatalf("CooldownWindow() = %q, want %q", got, "five_hour")
	}
}

// TestCodexExecutor_429WeeklyWindow verifies weekly window classification.
func TestCodexExecutor_429WeeklyWindow(t *testing.T) {
	t.Parallel()

	usageBody := []byte(`{
		"rate_limit": {
			"allowed": false,
			"limit_reached": true,
			"primary_window": {"limit_window_seconds": 18000, "used_percent": 80, "reset_after_seconds": 1000},
			"secondary_window": {"limit_window_seconds": 604800, "used_percent": 100, "reset_after_seconds": 86400}
		}
	}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/backend-api/wham/usage":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(usageBody)
		case r.Method == http.MethodPost && (r.URL.Path == "/backend-api/codex/responses" || r.URL.Path == "/backend-api/codex/responses/compact"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"detail":"Rate limit exceeded"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	auth := &cliproxyauth.Auth{
		ID:       "codex-429-weekly",
		Provider: "codex",
		Attributes: map[string]string{
			"api_key":  "test-key",
			"base_url": server.URL + "/backend-api/codex",
		},
		Metadata: map[string]any{"account_id": "test-account"},
	}

	exec := NewCodexExecutor(&config.Config{})
	_, err := exec.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "gpt-5",
		Payload: []byte(`{"input":[{"type":"message","content":[{"type":"text","text":"hi"}]}]}`),
	}, cliproxyexecutor.Options{SourceFormat: "codex"})
	if err == nil {
		t.Fatalf("Execute() err = nil, want non-nil")
	}
	type statusErr interface {
		StatusCode() int
		RetryAfter() *time.Duration
		CooldownWindow() string
	}
	se, ok := err.(statusErr)
	if !ok {
		t.Fatalf("error %T does not implement statusErr", err)
	}
	if se.CooldownWindow() != "weekly" {
		t.Fatalf("CooldownWindow() = %q, want weekly", se.CooldownWindow())
	}
	if se.RetryAfter() == nil || *se.RetryAfter() != 86400*time.Second {
		t.Fatalf("RetryAfter() = %v, want 24h", se.RetryAfter())
	}
}
