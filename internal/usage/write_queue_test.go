package usage

import (
	"sync"
	"testing"
	"time"
)

// testFlusher 实现 pgFlusher 接口，用于测试。
type testFlusher struct {
	mu         sync.Mutex
	details    [][]DetailRow
	aggregates [][]AggregateRow
	instanceID string
}

func (f *testFlusher) BatchInsertDetails(rows []DetailRow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]DetailRow, len(rows))
	copy(cp, rows)
	f.details = append(f.details, cp)
	return nil
}

func (f *testFlusher) UpsertAggregates(aggs []AggregateRow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]AggregateRow, len(aggs))
	copy(cp, aggs)
	f.aggregates = append(f.aggregates, cp)
	return nil
}

func (f *testFlusher) getInstanceID() string { return f.instanceID }

func (f *testFlusher) totalDetails() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, b := range f.details {
		n += len(b)
	}
	return n
}

func (f *testFlusher) totalAggBatches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.aggregates)
}

func makeDetailAt(ts time.Time, failed bool, tokens int64) RequestDetail {
	return RequestDetail{
		Timestamp: ts,
		Source:    "test",
		Failed:    failed,
		Tokens:    TokenStats{InputTokens: tokens, TotalTokens: tokens},
	}
}

func newTestQueue(f *testFlusher) *WriteQueue {
	return &WriteQueue{
		ch:     make(chan DetailRow, 100),
		flush:  50 * time.Millisecond,
		batch:  10,
		chSize: 100,
		done:   make(chan struct{}),
		store:  f,
	}
}

func TestEnqueueNonBlocking(t *testing.T) {
	f := &testFlusher{instanceID: "test-inst"}
	// channel 大小为 2，入队 3 条，第 3 条应被丢弃
	q := &WriteQueue{
		ch:     make(chan DetailRow, 2),
		flush:  time.Second,
		batch:  100,
		chSize: 2,
		done:   make(chan struct{}),
		store:  f,
	}
	d := makeDetailAt(time.Now(), false, 10)
	q.Enqueue("key1", "gpt-4", d)
	q.Enqueue("key1", "gpt-4", d)
	q.Enqueue("key1", "gpt-4", d) // 应被丢弃

	if len(q.ch) != 2 {
		t.Errorf("expected channel len 2, got %d", len(q.ch))
	}
}

func TestStartAndStopFlushesAll(t *testing.T) {
	f := &testFlusher{instanceID: "inst-1"}
	q := newTestQueue(f)
	q.Start()

	now := time.Now()
	for i := 0; i < 5; i++ {
		q.Enqueue("key1", "gpt-4", makeDetailAt(now, false, 100))
	}
	q.Stop()

	got := f.totalDetails()
	if got != 5 {
		t.Errorf("expected 5 details flushed, got %d", got)
	}
	if f.totalAggBatches() == 0 {
		t.Error("expected at least 1 aggregate upsert batch")
	}
}

func TestBatchFlushOnThreshold(t *testing.T) {
	f := &testFlusher{instanceID: "inst-1"}
	q := newTestQueue(f) // batch=10
	q.Start()

	now := time.Now()
	for i := 0; i < 25; i++ {
		q.Enqueue("key1", "gpt-4", makeDetailAt(now, false, 10))
	}
	// 等待 flush 处理
	time.Sleep(200 * time.Millisecond)
	q.Stop()

	got := f.totalDetails()
	if got != 25 {
		t.Errorf("expected 25 details, got %d", got)
	}
}

func TestAggregateGroupsByBucketHour(t *testing.T) {
	f := &testFlusher{instanceID: "inst-1"}
	q := newTestQueue(f)

	h1 := time.Date(2026, 2, 12, 14, 15, 0, 0, time.UTC)
	h2 := time.Date(2026, 2, 12, 15, 30, 0, 0, time.UTC)

	rows := []DetailRow{
		{APIKey: "k1", Model: "m1", Detail: makeDetailAt(h1, false, 100)},
		{APIKey: "k1", Model: "m1", Detail: makeDetailAt(h1.Add(10 * time.Minute), true, 50)},
		{APIKey: "k1", Model: "m1", Detail: makeDetailAt(h2, false, 200)},
	}

	aggs := q.aggregate(rows)
	if len(aggs) != 2 {
		t.Fatalf("expected 2 aggregate groups, got %d", len(aggs))
	}

	// 找到 h1 桶的聚合
	bucket14 := h1.Truncate(time.Hour)
	var agg14 *AggregateRow
	for i := range aggs {
		if aggs[i].BucketHour.Equal(bucket14) {
			agg14 = &aggs[i]
			break
		}
	}
	if agg14 == nil {
		t.Fatal("missing aggregate for hour 14")
	}
	if agg14.TotalRequests != 2 {
		t.Errorf("expected 2 requests in hour 14, got %d", agg14.TotalRequests)
	}
	if agg14.SuccessCount != 1 || agg14.FailureCount != 1 {
		t.Errorf("expected 1 success + 1 failure, got %d/%d",
			agg14.SuccessCount, agg14.FailureCount)
	}
	if agg14.InputTokens != 150 {
		t.Errorf("expected 150 input tokens, got %d", agg14.InputTokens)
	}
}

func TestNewWriteQueueOptions(t *testing.T) {
	store := &PGStore{instanceID: "x"}
	q := NewWriteQueue(store,
		WithFlushInterval(10*time.Second),
		WithBatchSize(200),
		WithChannelSize(3000),
	)
	if q.flush != 10*time.Second {
		t.Errorf("flush interval: want 10s, got %v", q.flush)
	}
	if q.batch != 200 {
		t.Errorf("batch: want 200, got %d", q.batch)
	}
	if cap(q.ch) != 3000 {
		t.Errorf("channel cap: want 3000, got %d", cap(q.ch))
	}
}

func TestNewWriteQueueDefaults(t *testing.T) {
	store := &PGStore{instanceID: "x"}
	q := NewWriteQueue(store)
	if q.flush != 5*time.Second {
		t.Errorf("default flush: want 5s, got %v", q.flush)
	}
	if q.batch != 500 {
		t.Errorf("default batch: want 500, got %d", q.batch)
	}
	if cap(q.ch) != 5000 {
		t.Errorf("default channel cap: want 5000, got %d", cap(q.ch))
	}
}
