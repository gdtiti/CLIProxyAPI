package management

import (
	"testing"
	"time"
)

func TestParseTimeRange_All(t *testing.T) {
	t.Parallel()

	before := time.Now()
	from, to := parseTimeRange("all", "", "")
	after := time.Now()

	expectedFrom := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if !from.Equal(expectedFrom) {
		t.Fatalf("from = %s, want %s", from.Format(time.RFC3339), expectedFrom.Format(time.RFC3339))
	}
	if to.Before(before) || to.After(after.Add(2*time.Second)) {
		t.Fatalf("to = %s, expected between %s and %s", to.Format(time.RFC3339), before.Format(time.RFC3339), after.Add(2*time.Second).Format(time.RFC3339))
	}
}
