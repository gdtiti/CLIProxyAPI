package usage

import (
	"sync"
)

// HotBuffer 是一个线程安全的环形缓冲，用于保留最近 N 条请求明细。
// 写满后自动覆盖最旧记录，内存占用恒定。
// 同时维护独立计数器，无需遍历 Details 即可获取总量统计。
type HotBuffer struct {
	mu      sync.RWMutex
	details []RequestDetail // 环形缓冲，最大 maxSize 条
	head    int             // 下一个写入位置
	count   int             // 当前有效数量（最大为 maxSize）
	maxSize int             // 容量上限

	// 实时计数器
	totalRequests int64
	successCount  int64
	failureCount  int64
	totalTokens   int64
}

// NewHotBuffer 创建指定容量的环形缓冲。
// 若 maxSize <= 0，默认使用 10000。
func NewHotBuffer(maxSize int) *HotBuffer {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &HotBuffer{
		details: make([]RequestDetail, maxSize),
		maxSize: maxSize,
	}
}

// Record 写入一条请求明细到环形缓冲，并更新计数器。
func (b *HotBuffer) Record(detail RequestDetail) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.details[b.head] = detail
	b.head = (b.head + 1) % b.maxSize
	if b.count < b.maxSize {
		b.count++
	}

	b.totalRequests++
	if detail.Failed {
		b.failureCount++
	} else {
		b.successCount++
	}
	b.totalTokens += detail.Tokens.TotalTokens
}

// RecentDetails 返回最近 n 条记录，按时间倒序（最新在前）。
func (b *HotBuffer) RecentDetails(n int) []RequestDetail {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n <= 0 || b.count == 0 {
		return nil
	}
	if n > b.count {
		n = b.count
	}

	result := make([]RequestDetail, n)
	// 从 head-1 开始倒序读取
	idx := (b.head - 1 + b.maxSize) % b.maxSize
	for i := 0; i < n; i++ {
		result[i] = b.details[idx]
		idx = (idx - 1 + b.maxSize) % b.maxSize
	}
	return result
}

// Counters 返回计数器值：总请求数、成功数、失败数、总 token 数。
func (b *HotBuffer) Counters() (total, success, failure, tokens int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.totalRequests, b.successCount, b.failureCount, b.totalTokens
}

// Reset 清空缓冲和所有计数器。
func (b *HotBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.details = make([]RequestDetail, b.maxSize)
	b.head = 0
	b.count = 0
	b.totalRequests = 0
	b.successCount = 0
	b.failureCount = 0
	b.totalTokens = 0
}
