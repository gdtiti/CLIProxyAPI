package management

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

type usageExportPayload struct {
	Version    int                      `json:"version"`
	ExportedAt time.Time                `json:"exported_at"`
	Usage      usage.StatisticsSnapshot `json:"usage"`
}

type usageImportPayload struct {
	Version int                      `json:"version"`
	Usage   usage.StatisticsSnapshot `json:"usage"`
}

// parseTimeRange 解析 range/from/to 查询参数为时间范围。
// 优先使用 rangeStr（如 "24h", "7d"），其次使用 from/to 字符串。
func parseTimeRange(rangeStr, fromStr, toStr string) (time.Time, time.Time) {
	now := time.Now()
	to := now
	from := now.Add(-24 * time.Hour) // 默认 24h

	if rangeStr != "" {
		d := parseDuration(rangeStr)
		from = now.Add(-d)
		return from, to
	}
	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24*time.Hour - time.Nanosecond)
		}
	}
	return from, to
}

// parseDuration 解析 "24h", "7d", "30d" 等格式为 time.Duration。
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 24 * time.Hour
	}
	// 尝试标准 Go duration 解析 (如 "24h", "1h30m")
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	// 解析 "7d", "30d" 等天数格式
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		if days, err := strconv.Atoi(numStr); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	return 24 * time.Hour
}

// GetUsageStatistics returns usage statistics with optional time range and instance filtering.
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	rangeStr := c.Query("range")
	fromStr := c.Query("from")
	toStr := c.Query("to")
	instance := c.Query("instance")

	// PG 持久化模式
	if h.usagePlugin != nil && h.usagePlugin.IsPersistent() {
		store := h.usagePlugin.GetPGStore()
		if store != nil {
			from, to := parseTimeRange(rangeStr, fromStr, toStr)
			snapshot, err := store.QuerySnapshot(from, to, instance)
			if err != nil {
				log.Printf("[GetUsageStatistics] PG query error: %v, fallback to memory", err)
			} else {
				c.JSON(http.StatusOK, gin.H{
					"usage":           snapshot,
					"failed_requests": snapshot.FailureCount,
				})
				return
			}
		}
	}

	// 退化为内存模式
	var snapshot usage.StatisticsSnapshot
	if h.usagePlugin != nil {
		snapshot = h.usagePlugin.Snapshot()
	} else if h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, gin.H{
		"usage":           snapshot,
		"failed_requests": snapshot.FailureCount,
	})
}

// DeleteUsageStatistics removes usage data from PG and resets HotBuffer.
func (h *Handler) DeleteUsageStatistics(c *gin.Context) {
	if h.usagePlugin == nil || !h.usagePlugin.IsPersistent() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PG mode not enabled"})
		return
	}
	store := h.usagePlugin.GetPGStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PG store unavailable"})
		return
	}

	beforeStr := c.Query("before")
	var err error
	if beforeStr != "" {
		var before time.Time
		before, err = time.Parse("2006-01-02", beforeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, use YYYY-MM-DD"})
			return
		}
		err = store.DeleteBefore(before)
	} else {
		err = store.DeleteAll()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同时重置 HotBuffer
	if buf := h.usagePlugin.GetHotBuffer(); buf != nil {
		buf.Reset()
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ExportUsageStatistics returns a complete usage snapshot for backup/migration.
func (h *Handler) ExportUsageStatistics(c *gin.Context) {
	var snapshot usage.StatisticsSnapshot

	// PG 模式：从 PG 查询完整数据
	if h.usagePlugin != nil && h.usagePlugin.IsPersistent() {
		store := h.usagePlugin.GetPGStore()
		if store != nil {
			// 导出全量：从很早到现在
			from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Now().Add(time.Hour)
			snap, err := store.QuerySnapshot(from, to, "")
			if err != nil {
				log.Printf("[ExportUsageStatistics] PG query error: %v, fallback", err)
			} else {
				snapshot = snap
			}
		}
	}

	// 退化为内存模式
	if snapshot.APIs == nil {
		if h.usagePlugin != nil {
			snapshot = h.usagePlugin.Snapshot()
		} else if h.usageStats != nil {
			snapshot = h.usageStats.Snapshot()
		}
	}

	c.JSON(http.StatusOK, usageExportPayload{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Usage:      snapshot,
	})
}

// ImportUsageStatistics merges a previously exported usage snapshot.
func (h *Handler) ImportUsageStatistics(c *gin.Context) {
	if h == nil || (h.usageStats == nil && h.usagePlugin == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usage statistics unavailable"})
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var payload usageImportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if payload.Version != 0 && payload.Version != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported version"})
		return
	}

	// 内存 merge（保留现有行为）
	var result usage.MergeResult
	if h.usageStats != nil {
		result = h.usageStats.MergeSnapshot(payload.Usage)
	}

	// PG 模式：同时写入 PG
	if h.usagePlugin != nil && h.usagePlugin.IsPersistent() {
		h.importToPG(payload.Usage)
	}

	var snapshot usage.StatisticsSnapshot
	if h.usagePlugin != nil {
		snapshot = h.usagePlugin.Snapshot()
	} else if h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, gin.H{
		"added":           result.Added,
		"skipped":         result.Skipped,
		"total_requests":  snapshot.TotalRequests,
		"failed_requests": snapshot.FailureCount,
	})
}

// importToPG 将导入的快照数据写入 PG 聚合表。
func (h *Handler) importToPG(snap usage.StatisticsSnapshot) {
	store := h.usagePlugin.GetPGStore()
	if store == nil {
		return
	}
	var aggs []usage.AggregateRow
	for apiKey, apiSnap := range snap.APIs {
		for model, modelSnap := range apiSnap.Models {
			// 按天聚合导入数据
			dayBuckets := make(map[time.Time]*usage.AggregateRow)
			for _, d := range modelSnap.Details {
				bucket := d.Timestamp.Truncate(time.Hour)
				agg, ok := dayBuckets[bucket]
				if !ok {
					agg = &usage.AggregateRow{
						APIKey:     apiKey,
						Model:      model,
						BucketHour: bucket,
					}
					dayBuckets[bucket] = agg
				}
				agg.TotalRequests++
				if d.Failed {
					agg.FailureCount++
				} else {
					agg.SuccessCount++
				}
				agg.InputTokens += d.Tokens.InputTokens
				agg.OutputTokens += d.Tokens.OutputTokens
				agg.ReasoningTokens += d.Tokens.ReasoningTokens
				agg.CachedTokens += d.Tokens.CachedTokens
				agg.TotalTokens += d.Tokens.TotalTokens
			}
			for _, agg := range dayBuckets {
				aggs = append(aggs, *agg)
			}
		}
	}
	if len(aggs) > 0 {
		if err := store.UpsertAggregates(aggs); err != nil {
			log.Printf("[ImportUsageStatistics] PG upsert error: %v", err)
		}
	}
}
