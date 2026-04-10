package usage

import (
	"testing"
	"time"
)

func makeDetail(i int, failed bool, tokens int64) RequestDetail {
	return RequestDetail{
		Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		Source:    "test",
		Failed:    failed,
		Tokens:    TokenStats{TotalTokens: tokens},
	}
}

func TestNewHotBuffer_DefaultSize(t *testing.T) {
	b := NewHotBuffer(0)
	if b.maxSize != 10000 {
		t.Errorf("expected default maxSize 10000, got %d", b.maxSize)
	}
}

func TestRecord_And_Counters(t *testing.T) {
	b := NewHotBuffer(5)
	b.Record(makeDetail(1, false, 100))
	b.Record(makeDetail(2, true, 50))
	b.Record(makeDetail(3, false, 200))

	total, success, failure, tokens := b.Counters()
	if total != 3 {
		t.Errorf("total: want 3, got %d", total)
	}
	if success != 2 {
		t.Errorf("success: want 2, got %d", success)
	}
	if failure != 1 {
		t.Errorf("failure: want 1, got %d", failure)
	}
	if tokens != 350 {
		t.Errorf("tokens: want 350, got %d", tokens)
	}
}

func TestRecentDetails_Order(t *testing.T) {
	b := NewHotBuffer(5)
	for i := 0; i < 3; i++ {
		b.Record(makeDetail(i, false, int64(i+1)*10))
	}
	recent := b.RecentDetails(3)
	if len(recent) != 3 {
		t.Fatalf("want 3 details, got %d", len(recent))
	}
	// 最新在前：tokens 应为 30, 20, 10
	if recent[0].Tokens.TotalTokens != 30 {
		t.Errorf("recent[0] tokens: want 30, got %d", recent[0].Tokens.TotalTokens)
	}
	if recent[2].Tokens.TotalTokens != 10 {
		t.Errorf("recent[2] tokens: want 10, got %d", recent[2].Tokens.TotalTokens)
	}
}

func TestRecentDetails_MoreThanCount(t *testing.T) {
	b := NewHotBuffer(10)
	b.Record(makeDetail(1, false, 100))
	recent := b.RecentDetails(5)
	if len(recent) != 1 {
		t.Errorf("want 1 detail, got %d", len(recent))
	}
}

func TestRecentDetails_ZeroOrNegative(t *testing.T) {
	b := NewHotBuffer(5)
	b.Record(makeDetail(1, false, 100))
	if r := b.RecentDetails(0); r != nil {
		t.Errorf("want nil for n=0, got %v", r)
	}
	if r := b.RecentDetails(-1); r != nil {
		t.Errorf("want nil for n=-1, got %v", r)
	}
}

func TestRingBuffer_Overwrite(t *testing.T) {
	b := NewHotBuffer(3)
	// 写入 5 条，容量 3，应覆盖前 2 条
	for i := 0; i < 5; i++ {
		b.Record(makeDetail(i, false, int64(i+1)*10))
	}
	total, _, _, _ := b.Counters()
	if total != 5 {
		t.Errorf("total: want 5, got %d", total)
	}
	recent := b.RecentDetails(3)
	if len(recent) != 3 {
		t.Fatalf("want 3 details, got %d", len(recent))
	}
	// 最新 3 条 tokens: 50, 40, 30
	expected := []int64{50, 40, 30}
	for i, exp := range expected {
		if recent[i].Tokens.TotalTokens != exp {
			t.Errorf("recent[%d] tokens: want %d, got %d",
				i, exp, recent[i].Tokens.TotalTokens)
		}
	}
}

func TestReset(t *testing.T) {
	b := NewHotBuffer(5)
	b.Record(makeDetail(1, false, 100))
	b.Record(makeDetail(2, true, 50))
	b.Reset()

	total, success, failure, tokens := b.Counters()
	if total != 0 || success != 0 || failure != 0 || tokens != 0 {
		t.Errorf("counters not zero after reset: %d %d %d %d",
			total, success, failure, tokens)
	}
	if r := b.RecentDetails(10); len(r) != 0 {
		t.Errorf("want 0 details after reset, got %d", len(r))
	}
}
