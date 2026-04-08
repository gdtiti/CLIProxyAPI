package usage

import (
	"testing"
	"time"
)

func TestBuildSnapshotFromRows_IncludesDetails(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	aggs := []AggregateRow{
		{
			InstanceID:    "instance-a",
			APIKey:        "api-key-1",
			Model:         "gpt-4.1",
			BucketHour:    base,
			TotalRequests: 3,
			SuccessCount:  2,
			FailureCount:  1,
			TotalTokens:   30,
		},
	}
	details := []DetailRow{
		{
			APIKey: "api-key-1",
			Model:  "gpt-4.1",
			Detail: RequestDetail{
				Timestamp: base.Add(15 * time.Minute),
				Source:    "openai",
				AuthIndex: "auth-1",
				Failed:    false,
				Tokens: TokenStats{
					InputTokens:  10,
					OutputTokens: 20,
					TotalTokens:  30,
				},
			},
		},
	}

	snapshot := buildSnapshotFromRows(aggs, details)

	model, ok := snapshot.APIs["api-key-1"].Models["gpt-4.1"]
	if !ok {
		t.Fatalf("expected model snapshot for api-key-1/gpt-4.1")
	}
	if len(model.Details) != 1 {
		t.Fatalf("details length = %d, want 1", len(model.Details))
	}
	if got := model.Details[0].AuthIndex; got != "auth-1" {
		t.Fatalf("details[0].AuthIndex = %q, want %q", got, "auth-1")
	}
	if snapshot.TotalRequests != 3 {
		t.Fatalf("TotalRequests = %d, want 3", snapshot.TotalRequests)
	}
	if snapshot.TotalTokens != 30 {
		t.Fatalf("TotalTokens = %d, want 30", snapshot.TotalTokens)
	}
}
