package management

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
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

type usageResponsePayload struct {
	Usage                usage.StatisticsSnapshot `json:"usage"`
	FailedRequests       int64                    `json:"failed_requests"`
	APIStats             []usageAPISummary        `json:"api_stats,omitempty"`
	APIStatsPagination   *usageSummaryPagination  `json:"api_stats_pagination,omitempty"`
	ModelStats           []usageModelSummary      `json:"model_stats,omitempty"`
	ModelStatsPagination *usageSummaryPagination  `json:"model_stats_pagination,omitempty"`
}

type usageAPISummary struct {
	API           string     `json:"api"`
	TotalRequests int64      `json:"total_requests"`
	TotalTokens   int64      `json:"total_tokens"`
	SuccessCount  int64      `json:"success_count"`
	FailureCount  int64      `json:"failure_count"`
	ModelCount    int        `json:"model_count"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
}

type usageModelSummary struct {
	Model         string     `json:"model"`
	TotalRequests int64      `json:"total_requests"`
	TotalTokens   int64      `json:"total_tokens"`
	SuccessCount  int64      `json:"success_count"`
	FailureCount  int64      `json:"failure_count"`
	APICount      int        `json:"api_count"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
}

type usageModelSummaryAccumulator struct {
	summary usageModelSummary
	apiKeys map[string]struct{}
}

type usageSummaryFilters struct {
	API   string
	Model string
}

type usageSummaryPagination struct {
	Total    int  `json:"total"`
	Offset   int  `json:"offset"`
	Limit    int  `json:"limit"`
	Returned int  `json:"returned"`
	HasMore  bool `json:"has_more"`
}

// parseTimeRange parses range/from/to query parameters into a time window.
// rangeStr takes priority (for example: "24h", "7d"), then from/to are used.
func parseTimeRange(rangeStr, fromStr, toStr string) (time.Time, time.Time) {
	now := time.Now()
	to := now
	from := now.Add(-24 * time.Hour)

	if rangeStr != "" {
		if strings.EqualFold(strings.TrimSpace(rangeStr), "all") {
			return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), to
		}
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

// parseDuration parses values like "24h", "7d", and "30d".
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 24 * time.Hour
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		if days, err := strconv.Atoi(numStr); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	return 24 * time.Hour
}

func buildUsageResponsePayload(c *gin.Context, snapshot usage.StatisticsSnapshot) usageResponsePayload {
	filters := usageSummaryFilters{
		API:   normalizeUsageSummaryFilter(c.Query("api_filter")),
		Model: normalizeUsageSummaryFilter(c.Query("model_filter")),
	}
	apiStats, apiPagination := buildUsageAPISummaries(
		snapshot,
		filters,
		c.Query("api_sort_by"),
		c.Query("api_sort_order"),
		parseUsageSummaryOffset(c.Query("api_offset")),
		parseUsageSummaryLimit(c.Query("api_limit")),
	)
	modelStats, modelPagination := buildUsageModelSummaries(
		snapshot,
		filters,
		c.Query("model_sort_by"),
		c.Query("model_sort_order"),
		parseUsageSummaryOffset(c.Query("model_offset")),
		parseUsageSummaryLimit(c.Query("model_limit")),
	)
	return usageResponsePayload{
		Usage:                snapshot,
		FailedRequests:       snapshot.FailureCount,
		APIStats:             apiStats,
		APIStatsPagination:   apiPagination,
		ModelStats:           modelStats,
		ModelStatsPagination: modelPagination,
	}
}

func buildUsageAPISummaries(snapshot usage.StatisticsSnapshot, filters usageSummaryFilters, sortBy, sortOrder string, offset, limit int) ([]usageAPISummary, *usageSummaryPagination) {
	if len(snapshot.APIs) == 0 {
		return nil, buildUsageSummaryPagination(0, offset, limit, 0)
	}

	summaries := make([]usageAPISummary, 0, len(snapshot.APIs))
	for apiKey, apiSnapshot := range snapshot.APIs {
		if !matchesUsageSummaryFilter(apiKey, filters.API) {
			continue
		}
		summary := usageAPISummary{
			API: apiKey,
		}
		for modelName, modelSnapshot := range apiSnapshot.Models {
			if !matchesUsageSummaryFilter(modelName, filters.Model) {
				continue
			}
			summary.TotalRequests += modelSnapshot.TotalRequests
			summary.TotalTokens += modelSnapshot.TotalTokens
			summary.ModelCount++
			accumulateUsageDetailStats(&summary.SuccessCount, &summary.FailureCount, &summary.LastUsedAt, modelSnapshot.Details)
		}
		if summary.ModelCount == 0 {
			continue
		}
		summaries = append(summaries, summary)
	}

	sortUsageAPISummaries(summaries, sortBy, sortOrder)
	pagedItems, pagination := paginateUsageSummaries(summaries, offset, limit)
	return pagedItems, pagination
}

func buildUsageModelSummaries(snapshot usage.StatisticsSnapshot, filters usageSummaryFilters, sortBy, sortOrder string, offset, limit int) ([]usageModelSummary, *usageSummaryPagination) {
	if len(snapshot.APIs) == 0 {
		return nil, buildUsageSummaryPagination(0, offset, limit, 0)
	}

	accumulators := make(map[string]*usageModelSummaryAccumulator)
	for apiKey, apiSnapshot := range snapshot.APIs {
		if !matchesUsageSummaryFilter(apiKey, filters.API) {
			continue
		}
		for modelName, modelSnapshot := range apiSnapshot.Models {
			if !matchesUsageSummaryFilter(modelName, filters.Model) {
				continue
			}
			accumulator, ok := accumulators[modelName]
			if !ok {
				accumulator = &usageModelSummaryAccumulator{
					summary: usageModelSummary{
						Model: modelName,
					},
					apiKeys: make(map[string]struct{}),
				}
				accumulators[modelName] = accumulator
			}
			accumulator.summary.TotalRequests += modelSnapshot.TotalRequests
			accumulator.summary.TotalTokens += modelSnapshot.TotalTokens
			accumulator.apiKeys[apiKey] = struct{}{}
			accumulateUsageDetailStats(&accumulator.summary.SuccessCount, &accumulator.summary.FailureCount, &accumulator.summary.LastUsedAt, modelSnapshot.Details)
		}
	}

	summaries := make([]usageModelSummary, 0, len(accumulators))
	for _, accumulator := range accumulators {
		accumulator.summary.APICount = len(accumulator.apiKeys)
		summaries = append(summaries, accumulator.summary)
	}

	sortUsageModelSummaries(summaries, sortBy, sortOrder)
	pagedItems, pagination := paginateUsageSummaries(summaries, offset, limit)
	return pagedItems, pagination
}

func normalizeUsageSummaryFilter(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func matchesUsageSummaryFilter(value, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), filter)
}

func parseUsageSummaryOffset(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func parseUsageSummaryLimit(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func paginateUsageSummaries[T any](items []T, offset, limit int) ([]T, *usageSummaryPagination) {
	total := len(items)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	pagination := buildUsageSummaryPagination(total, offset, limit, end-offset)
	if offset == 0 && end == total {
		return items, pagination
	}
	out := make([]T, end-offset)
	copy(out, items[offset:end])
	return out, pagination
}

func buildUsageSummaryPagination(total, offset, limit, returned int) *usageSummaryPagination {
	if offset < 0 {
		offset = 0
	}
	if total < 0 {
		total = 0
	}
	if returned < 0 {
		returned = 0
	}
	return &usageSummaryPagination{
		Total:    total,
		Offset:   offset,
		Limit:    limit,
		Returned: returned,
		HasMore:  offset+returned < total,
	}
}

func accumulateUsageDetailStats(successCount, failureCount *int64, lastUsedAt **time.Time, details []usage.RequestDetail) {
	for _, detail := range details {
		if detail.Failed {
			*failureCount = *failureCount + 1
		} else {
			*successCount = *successCount + 1
		}
		if detail.Timestamp.IsZero() {
			continue
		}
		if *lastUsedAt == nil || detail.Timestamp.After(**lastUsedAt) {
			timestamp := detail.Timestamp.UTC()
			*lastUsedAt = &timestamp
		}
	}
}

func sortUsageAPISummaries(items []usageAPISummary, sortBy, sortOrder string) {
	normalizedSortBy := normalizeUsageAPISortBy(sortBy)
	descending := normalizeUsageSortOrder(sortOrder) == "desc"
	sort.SliceStable(items, func(leftIndex, rightIndex int) bool {
		comparison := compareUsageAPISummaries(items[leftIndex], items[rightIndex], normalizedSortBy)
		if comparison == 0 {
			comparison = compareStrings(items[leftIndex].API, items[rightIndex].API)
		}
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
}

func sortUsageModelSummaries(items []usageModelSummary, sortBy, sortOrder string) {
	normalizedSortBy := normalizeUsageModelSortBy(sortBy)
	descending := normalizeUsageSortOrder(sortOrder) == "desc"
	sort.SliceStable(items, func(leftIndex, rightIndex int) bool {
		comparison := compareUsageModelSummaries(items[leftIndex], items[rightIndex], normalizedSortBy)
		if comparison == 0 {
			comparison = compareStrings(items[leftIndex].Model, items[rightIndex].Model)
		}
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
}

func normalizeUsageAPISortBy(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "api", "name":
		return "api"
	case "total_requests", "requests":
		return "total_requests"
	case "total_tokens", "tokens":
		return "total_tokens"
	case "success_count", "success":
		return "success_count"
	case "failure_count", "failed", "failure":
		return "failure_count"
	case "model_count", "models":
		return "model_count"
	case "last_used_at", "last_used":
		return "last_used_at"
	default:
		return "total_tokens"
	}
}

func normalizeUsageModelSortBy(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "model", "name":
		return "model"
	case "total_requests", "requests":
		return "total_requests"
	case "total_tokens", "tokens":
		return "total_tokens"
	case "success_count", "success":
		return "success_count"
	case "failure_count", "failed", "failure":
		return "failure_count"
	case "api_count", "apis":
		return "api_count"
	case "last_used_at", "last_used":
		return "last_used_at"
	default:
		return "total_tokens"
	}
}

func normalizeUsageSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}

func compareUsageAPISummaries(left, right usageAPISummary, sortBy string) int {
	switch sortBy {
	case "api":
		return compareStrings(left.API, right.API)
	case "total_requests":
		return compareInt64(left.TotalRequests, right.TotalRequests)
	case "total_tokens":
		return compareInt64(left.TotalTokens, right.TotalTokens)
	case "success_count":
		return compareInt64(left.SuccessCount, right.SuccessCount)
	case "failure_count":
		return compareInt64(left.FailureCount, right.FailureCount)
	case "model_count":
		return compareInt64(int64(left.ModelCount), int64(right.ModelCount))
	case "last_used_at":
		return compareOptionalTime(left.LastUsedAt, right.LastUsedAt)
	default:
		return 0
	}
}

func compareUsageModelSummaries(left, right usageModelSummary, sortBy string) int {
	switch sortBy {
	case "model":
		return compareStrings(left.Model, right.Model)
	case "total_requests":
		return compareInt64(left.TotalRequests, right.TotalRequests)
	case "total_tokens":
		return compareInt64(left.TotalTokens, right.TotalTokens)
	case "success_count":
		return compareInt64(left.SuccessCount, right.SuccessCount)
	case "failure_count":
		return compareInt64(left.FailureCount, right.FailureCount)
	case "api_count":
		return compareInt64(int64(left.APICount), int64(right.APICount))
	case "last_used_at":
		return compareOptionalTime(left.LastUsedAt, right.LastUsedAt)
	default:
		return 0
	}
}

func compareOptionalTime(left, right *time.Time) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return -1
	case right == nil:
		return 1
	default:
		return compareTime(*left, *right)
	}
}

// GetUsageStatistics returns usage statistics with optional time range and instance filtering.
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	rangeStr := c.Query("range")
	fromStr := c.Query("from")
	toStr := c.Query("to")
	instance := c.Query("instance")

	if h.usagePlugin != nil && h.usagePlugin.IsPersistent() {
		store := h.usagePlugin.GetPGStore()
		if store != nil {
			from, to := parseTimeRange(rangeStr, fromStr, toStr)
			snapshot, err := store.QuerySnapshot(from, to, instance)
			if err != nil {
				log.Printf("[GetUsageStatistics] PG query error: %v, fallback to memory", err)
			} else {
				c.JSON(http.StatusOK, buildUsageResponsePayload(c, snapshot))
				return
			}
		}
	}

	var snapshot usage.StatisticsSnapshot
	if h.usagePlugin != nil {
		snapshot = h.usagePlugin.Snapshot()
	} else if h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, buildUsageResponsePayload(c, snapshot))
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

	if buf := h.usagePlugin.GetHotBuffer(); buf != nil {
		buf.Reset()
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ExportUsageStatistics returns a complete usage snapshot for backup or migration.
func (h *Handler) ExportUsageStatistics(c *gin.Context) {
	var snapshot usage.StatisticsSnapshot

	if h.usagePlugin != nil && h.usagePlugin.IsPersistent() {
		store := h.usagePlugin.GetPGStore()
		if store != nil {
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

	var result usage.MergeResult
	if h.usageStats != nil {
		result = h.usageStats.MergeSnapshot(payload.Usage)
	}

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

// importToPG writes imported snapshot data into the PG aggregate tables.
func (h *Handler) importToPG(snap usage.StatisticsSnapshot) {
	store := h.usagePlugin.GetPGStore()
	if store == nil {
		return
	}
	var aggs []usage.AggregateRow
	for apiKey, apiSnap := range snap.APIs {
		for model, modelSnap := range apiSnap.Models {
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
