package usage

import (
	"log"
	"sync"
	"time"
)

// WriteQueueOption 用于配置 WriteQueue 的函数选项。
type WriteQueueOption func(*WriteQueue)

// WithFlushInterval 设置刷新间隔。
func WithFlushInterval(d time.Duration) WriteQueueOption {
	return func(q *WriteQueue) { q.flush = d }
}

// WithBatchSize 设置单次批量写入上限。
func WithBatchSize(n int) WriteQueueOption {
	return func(q *WriteQueue) { q.batch = n }
}

// WithChannelSize 设置 channel 缓冲大小。
func WithChannelSize(n int) WriteQueueOption {
	return func(q *WriteQueue) { q.chSize = n }
}

// pgFlusher 定义 WriteQueue 所需的 PG 写入接口。
type pgFlusher interface {
	BatchInsertDetails(rows []DetailRow) error
	UpsertAggregates(aggs []AggregateRow) error
	getInstanceID() string
}

// WriteQueue 通过 channel + ticker 实现异步批量写入 PG。
type WriteQueue struct {
	ch     chan DetailRow
	flush  time.Duration
	batch  int
	chSize int
	done   chan struct{}
	wg     sync.WaitGroup
	store  pgFlusher
}

// NewWriteQueue 创建 WriteQueue 实例。
// 默认: channel 5000, flush 间隔 5s, batch 500。
func NewWriteQueue(store *PGStore, opts ...WriteQueueOption) *WriteQueue {
	q := &WriteQueue{
		flush:  5 * time.Second,
		batch:  500,
		chSize: 5000,
		done:   make(chan struct{}),
		store:  store,
	}
	for _, o := range opts {
		o(q)
	}
	q.ch = make(chan DetailRow, q.chSize)
	return q
}

// Enqueue 非阻塞地将一条 DetailRow 放入写入队列。
// channel 满时直接丢弃，不阻塞调用方。
func (q *WriteQueue) Enqueue(apiKey, model string, detail RequestDetail) {
	row := DetailRow{
		APIKey: apiKey,
		Model:  model,
		Detail: detail,
	}
	select {
	case q.ch <- row:
	default:
		log.Printf("[WriteQueue] channel full, dropping record for model=%s", model)
	}
}

// Start 启动后台 flush 协程。
// 使用 ticker + channel 驱动：积累 batch 条或 ticker 触发时执行 flush。
func (q *WriteQueue) Start() {
	q.wg.Add(1)
	go q.loop()
}

func (q *WriteQueue) loop() {
	defer q.wg.Done()
	ticker := time.NewTicker(q.flush)
	defer ticker.Stop()

	buf := make([]DetailRow, 0, q.batch)

	for {
		select {
		case row, ok := <-q.ch:
			if !ok {
				// channel 已关闭，flush 剩余并退出
				if len(buf) > 0 {
					q.doFlush(buf)
				}
				return
			}
			buf = append(buf, row)
			if len(buf) >= q.batch {
				q.doFlush(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				q.doFlush(buf)
				buf = buf[:0]
			}
		}
	}
}

// Stop 优雅关闭写入队列。
// 关闭 channel 后，loop 协程会 drain 剩余数据并 flush，然后退出。
func (q *WriteQueue) Stop() {
	close(q.ch)
	q.wg.Wait()
}

// doFlush 将一批 DetailRow 写入 PG：
// 1. 批量插入 usage_details
// 2. 按 (instanceID, apiKey, model, bucketHour) 聚合后 upsert usage_aggregates
func (q *WriteQueue) doFlush(rows []DetailRow) {
	// 1. 批量插入明细
	if err := q.store.BatchInsertDetails(rows); err != nil {
		log.Printf("[WriteQueue] batch insert error: %v", err)
	}

	// 2. 聚合并 upsert
	aggs := q.aggregate(rows)
	if err := q.store.UpsertAggregates(aggs); err != nil {
		log.Printf("[WriteQueue] upsert aggregates error: %v", err)
	}
}

// aggKey 是聚合的复合键。
type aggKey struct {
	apiKey     string
	model      string
	bucketHour time.Time
}

// aggregate 将 DetailRow 按 (apiKey, model, bucketHour) 聚合为 AggregateRow。
func (q *WriteQueue) aggregate(rows []DetailRow) []AggregateRow {
	m := make(map[aggKey]*AggregateRow, len(rows)/2+1)
	for _, r := range rows {
		bh := r.Detail.Timestamp.Truncate(time.Hour)
		k := aggKey{apiKey: r.APIKey, model: r.Model, bucketHour: bh}
		agg, ok := m[k]
		if !ok {
			agg = &AggregateRow{
				InstanceID: q.store.getInstanceID(),
				APIKey:     r.APIKey,
				Model:      r.Model,
				BucketHour: bh,
			}
			m[k] = agg
		}
		agg.TotalRequests++
		if r.Detail.Failed {
			agg.FailureCount++
		} else {
			agg.SuccessCount++
		}
		t := r.Detail.Tokens
		agg.InputTokens += t.InputTokens
		agg.OutputTokens += t.OutputTokens
		agg.ReasoningTokens += t.ReasoningTokens
		agg.CachedTokens += t.CachedTokens
		agg.TotalTokens += t.TotalTokens
	}
	result := make([]AggregateRow, 0, len(m))
	for _, agg := range m {
		result = append(result, *agg)
	}
	return result
}
