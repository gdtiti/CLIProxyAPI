package executor

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestParseOpenAIUsageChatCompletions(t *testing.T) {
	data := []byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":5}}}`)
	detail := parseOpenAIUsage(data)
	if detail.InputTokens != 1 {
		t.Fatalf("input tokens = %d, want %d", detail.InputTokens, 1)
	}
	if detail.OutputTokens != 2 {
		t.Fatalf("output tokens = %d, want %d", detail.OutputTokens, 2)
	}
	if detail.TotalTokens != 3 {
		t.Fatalf("total tokens = %d, want %d", detail.TotalTokens, 3)
	}
	if detail.CachedTokens != 4 {
		t.Fatalf("cached tokens = %d, want %d", detail.CachedTokens, 4)
	}
	if detail.ReasoningTokens != 5 {
		t.Fatalf("reasoning tokens = %d, want %d", detail.ReasoningTokens, 5)
	}
}

func TestParseOpenAIUsageResponses(t *testing.T) {
	data := []byte(`{"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30,"input_tokens_details":{"cached_tokens":7},"output_tokens_details":{"reasoning_tokens":9}}}`)
	detail := parseOpenAIUsage(data)
	if detail.InputTokens != 10 {
		t.Fatalf("input tokens = %d, want %d", detail.InputTokens, 10)
	}
	if detail.OutputTokens != 20 {
		t.Fatalf("output tokens = %d, want %d", detail.OutputTokens, 20)
	}
	if detail.TotalTokens != 30 {
		t.Fatalf("total tokens = %d, want %d", detail.TotalTokens, 30)
	}
	if detail.CachedTokens != 7 {
		t.Fatalf("cached tokens = %d, want %d", detail.CachedTokens, 7)
	}
	if detail.ReasoningTokens != 9 {
		t.Fatalf("reasoning tokens = %d, want %d", detail.ReasoningTokens, 9)
	}
}

func TestParseCodexUsageLimitRetryAfter_FromResetSeconds(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)
	body := []byte(`{"error":{"type":"usage_limit_reached","resets_in_seconds":8593}}`)
	got := parseCodexUsageLimitRetryAfter(body, now)
	if got == nil {
		t.Fatalf("parseCodexUsageLimitRetryAfter() = nil")
	}
	if *got != 8593*time.Second {
		t.Fatalf("retryAfter = %v, want %v", *got, 8593*time.Second)
	}
}

func TestParseCodexUsageLimitRetryAfter_FromResetAt(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)
	body := []byte(`{"error":{"type":"usage_limit_reached","resets_at":1770007200}}`)
	got := parseCodexUsageLimitRetryAfter(body, now)
	if got == nil {
		t.Fatalf("parseCodexUsageLimitRetryAfter() = nil")
	}
	if *got != 2*time.Hour {
		t.Fatalf("retryAfter = %v, want %v", *got, 2*time.Hour)
	}
}

func TestParseCodexUsageLimitRetryAfter_FromNestedStatusMessage(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)
	body := []byte(`{"status_message":"{\"error\":{\"type\":\"usage_limit_reached\",\"resets_in_seconds\":300}}"}`)
	got := parseCodexUsageLimitRetryAfter(body, now)
	if got == nil {
		t.Fatalf("parseCodexUsageLimitRetryAfter() = nil")
	}
	if *got != 300*time.Second {
		t.Fatalf("retryAfter = %v, want %v", *got, 300*time.Second)
	}
}

func TestParseCodexUsageLimitRetryAfter_NonUsageLimit(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)
	body := []byte(`{"error":{"type":"rate_limit_exceeded","resets_in_seconds":120}}`)
	if got := parseCodexUsageLimitRetryAfter(body, now); got != nil {
		t.Fatalf("parseCodexUsageLimitRetryAfter() = %v, want nil", *got)
	}
}

func TestIsCodexUsageLimitReached(t *testing.T) {
	t.Run("direct payload", func(t *testing.T) {
		body := []byte(`{"error":{"type":"usage_limit_reached"}}`)
		if !isCodexUsageLimitReached(body) {
			t.Fatalf("isCodexUsageLimitReached() = false, want true")
		}
	})

	t.Run("nested status_message payload", func(t *testing.T) {
		body := []byte(`{"status_message":"{\"error\":{\"type\":\"usage_limit_reached\"}}"}`)
		if !isCodexUsageLimitReached(body) {
			t.Fatalf("isCodexUsageLimitReached() = false, want true")
		}
	})

	t.Run("non usage limit payload", func(t *testing.T) {
		body := []byte(`{"error":{"type":"invalid_request_error"}}`)
		if isCodexUsageLimitReached(body) {
			t.Fatalf("isCodexUsageLimitReached() = true, want false")
		}
	})
}

func TestParseCodexQuotaRetryAfter(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)

	t.Run("five-hour window reached", func(t *testing.T) {
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"primary_window": {"limit_window_seconds": 18000, "used_percent": 100, "reset_after_seconds": 1200},
				"secondary_window": {"limit_window_seconds": 604800, "used_percent": 30, "reset_after_seconds": 600000}
			}
		}`)
		got := parseCodexQuotaRetryAfter(body, now)
		if got == nil {
			t.Fatalf("parseCodexQuotaRetryAfter() = nil")
		}
		if *got != 1200*time.Second {
			t.Fatalf("retryAfter = %v, want %v", *got, 1200*time.Second)
		}
	})

	t.Run("weekly window reached", func(t *testing.T) {
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"primary_window": {"limit_window_seconds": 18000, "used_percent": 88, "reset_after_seconds": 1200},
				"secondary_window": {"limit_window_seconds": 604800, "used_percent": 100, "reset_after_seconds": 500000}
			}
		}`)
		got := parseCodexQuotaRetryAfter(body, now)
		if got == nil {
			t.Fatalf("parseCodexQuotaRetryAfter() = nil")
		}
		if *got != 500000*time.Second {
			t.Fatalf("retryAfter = %v, want %v", *got, 500000*time.Second)
		}
	})

	t.Run("both reached choose longer", func(t *testing.T) {
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"primary_window": {"limit_window_seconds": 18000, "used_percent": 100, "reset_after_seconds": 900},
				"secondary_window": {"limit_window_seconds": 604800, "used_percent": 100, "reset_after_seconds": 400000}
			}
		}`)
		got := parseCodexQuotaRetryAfter(body, now)
		if got == nil {
			t.Fatalf("parseCodexQuotaRetryAfter() = nil")
		}
		if *got != 400000*time.Second {
			t.Fatalf("retryAfter = %v, want %v", *got, 400000*time.Second)
		}
	})

	t.Run("limited but no 100-percent window falls back", func(t *testing.T) {
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"primary_window": {"limit_window_seconds": 18000, "used_percent": 80, "reset_after_seconds": 1000},
				"secondary_window": {"limit_window_seconds": 604800, "used_percent": 20, "reset_after_seconds": 100000}
			}
		}`)
		if got := parseCodexQuotaRetryAfter(body, now); got != nil {
			t.Fatalf("parseCodexQuotaRetryAfter() = %v, want nil", *got)
		}
	})
}

func TestParseCodexQuotaRetryDecision_WindowClassification(t *testing.T) {
	now := time.Unix(1_770_000_000, 0)

	t.Run("five-hour only", func(t *testing.T) {
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"primary_window": {"limit_window_seconds": 18000, "used_percent": 100, "reset_after_seconds": 1200}
			}
		}`)
		decision := parseCodexQuotaRetryDecision(body, now)
		if decision.RetryAfter == nil {
			t.Fatalf("RetryAfter = nil")
		}
		if decision.Window != "five_hour" {
			t.Fatalf("Window = %q, want %q", decision.Window, "five_hour")
		}
	})

	t.Run("weekly only", func(t *testing.T) {
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"secondary_window": {"limit_window_seconds": 604800, "used_percent": 100, "reset_after_seconds": 200000}
			}
		}`)
		decision := parseCodexQuotaRetryDecision(body, now)
		if decision.RetryAfter == nil {
			t.Fatalf("RetryAfter = nil")
		}
		if decision.Window != "weekly" {
			t.Fatalf("Window = %q, want %q", decision.Window, "weekly")
		}
	})

	t.Run("both 5h and weekly reached, identify as weekly", func(t *testing.T) {
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"primary_window": {"limit_window_seconds": 18000, "used_percent": 100, "reset_after_seconds": 900},
				"secondary_window": {"limit_window_seconds": 604800, "used_percent": 100, "reset_after_seconds": 400000}
			}
		}`)
		decision := parseCodexQuotaRetryDecision(body, now)
		if decision.RetryAfter == nil {
			t.Fatalf("RetryAfter = nil")
		}
		if decision.Window != "weekly" {
			t.Fatalf("Window = %q, want weekly (both reached, pick longer duration)", decision.Window)
		}
		if *decision.RetryAfter != 400000*time.Second {
			t.Fatalf("RetryAfter = %v, want 400000s (weekly reset)", *decision.RetryAfter)
		}
	})

	t.Run("actual Codex usage API response format (weekly only at 100%)", func(t *testing.T) {
		// Real response from chatgpt.com/backend-api/wham/usage for a rate-limited account
		body := []byte(`{
			"rate_limit": {
				"allowed": false,
				"limit_reached": true,
				"primary_window": {"used_percent": 0, "limit_window_seconds": 18000, "reset_after_seconds": 18000, "reset_at": 1772263559},
				"secondary_window": {"used_percent": 100, "limit_window_seconds": 604800, "reset_after_seconds": 475440, "reset_at": 1772720999}
			}
		}`)
		now := time.Unix(1772245000, 0) // ~5.5 days before reset_at
		decision := parseCodexQuotaRetryDecision(body, now)
		if decision.RetryAfter == nil {
			t.Fatalf("RetryAfter = nil")
		}
		if decision.Window != "weekly" {
			t.Fatalf("Window = %q, want weekly (secondary at 100%%, primary at 0%%)", decision.Window)
		}
		// reset_after_seconds=475440 should be used
		if *decision.RetryAfter != 475440*time.Second {
			t.Fatalf("RetryAfter = %v, want 475440s", *decision.RetryAfter)
		}
	})
}

func TestUsageReporterBuildRecordIncludesLatency(t *testing.T) {
	reporter := &usageReporter{
		provider:    "openai",
		model:       "gpt-5.4",
		requestedAt: time.Now().Add(-1500 * time.Millisecond),
	}

	record := reporter.buildRecord(usage.Detail{TotalTokens: 3}, false)
	if record.Latency < time.Second {
		t.Fatalf("latency = %v, want >= 1s", record.Latency)
	}
	if record.Latency > 3*time.Second {
		t.Fatalf("latency = %v, want <= 3s", record.Latency)
	}
}
