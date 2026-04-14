package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestGetUsageStatistics_IncludesSortedUsageSummaries(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	stats := usage.NewRequestStatistics()
	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	records := []coreusage.Record{
		{
			APIKey:      "api-b",
			Model:       "gpt-5.4",
			RequestedAt: base.Add(30 * time.Minute),
			Failed:      true,
			Detail: coreusage.Detail{
				InputTokens:  6,
				OutputTokens: 4,
				TotalTokens:  10,
			},
		},
		{
			APIKey:      "api-a",
			Model:       "gpt-5.4",
			RequestedAt: base.Add(10 * time.Minute),
			Detail: coreusage.Detail{
				InputTokens:  18,
				OutputTokens: 12,
				TotalTokens:  30,
			},
		},
		{
			APIKey:      "api-a",
			Model:       "claude-3-7-sonnet",
			RequestedAt: base.Add(20 * time.Minute),
			Detail: coreusage.Detail{
				InputTokens:  12,
				OutputTokens: 8,
				TotalTokens:  20,
			},
		},
	}
	for _, record := range records {
		stats.Record(context.Background(), record)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{}, coreauth.NewManager(nil, nil, nil))
	h.SetUsageStatistics(stats)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/usage?api_sort_by=total_tokens&api_sort_order=desc&model_sort_by=last_used_at&model_sort_order=asc",
		nil,
	)

	h.GetUsageStatistics(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetUsageStatistics status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		FailedRequests int64 `json:"failed_requests"`
		APIStats       []struct {
			API           string     `json:"api"`
			TotalRequests int64      `json:"total_requests"`
			TotalTokens   int64      `json:"total_tokens"`
			SuccessCount  int64      `json:"success_count"`
			FailureCount  int64      `json:"failure_count"`
			ModelCount    int        `json:"model_count"`
			LastUsedAt    *time.Time `json:"last_used_at"`
		} `json:"api_stats"`
		ModelStats []struct {
			Model         string     `json:"model"`
			TotalRequests int64      `json:"total_requests"`
			TotalTokens   int64      `json:"total_tokens"`
			SuccessCount  int64      `json:"success_count"`
			FailureCount  int64      `json:"failure_count"`
			APICount      int        `json:"api_count"`
			LastUsedAt    *time.Time `json:"last_used_at"`
		} `json:"model_stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.FailedRequests != 1 {
		t.Fatalf("failed_requests = %d, want %d", payload.FailedRequests, 1)
	}

	if len(payload.APIStats) != 2 {
		t.Fatalf("len(api_stats) = %d, want %d", len(payload.APIStats), 2)
	}
	if payload.APIStats[0].API != "api-a" || payload.APIStats[0].TotalTokens != 50 {
		t.Fatalf("api_stats[0] = %+v, want api-a with 50 tokens", payload.APIStats[0])
	}
	if payload.APIStats[0].SuccessCount != 2 || payload.APIStats[0].FailureCount != 0 || payload.APIStats[0].ModelCount != 2 {
		t.Fatalf("api_stats[0] counters = %+v, want success=2 failure=0 model_count=2", payload.APIStats[0])
	}
	if payload.APIStats[1].API != "api-b" || payload.APIStats[1].FailureCount != 1 {
		t.Fatalf("api_stats[1] = %+v, want api-b with failure_count=1", payload.APIStats[1])
	}

	if len(payload.ModelStats) != 2 {
		t.Fatalf("len(model_stats) = %d, want %d", len(payload.ModelStats), 2)
	}
	if payload.ModelStats[0].Model != "claude-3-7-sonnet" {
		t.Fatalf("model_stats[0].model = %q, want %q", payload.ModelStats[0].Model, "claude-3-7-sonnet")
	}
	if payload.ModelStats[1].Model != "gpt-5.4" || payload.ModelStats[1].APICount != 2 || payload.ModelStats[1].FailureCount != 1 {
		t.Fatalf("model_stats[1] = %+v, want gpt-5.4 with api_count=2 failure_count=1", payload.ModelStats[1])
	}
	if payload.ModelStats[0].LastUsedAt == nil || payload.ModelStats[1].LastUsedAt == nil {
		t.Fatalf("expected model_stats last_used_at values, got %+v", payload.ModelStats)
	}
	if !payload.ModelStats[0].LastUsedAt.Before(*payload.ModelStats[1].LastUsedAt) {
		t.Fatalf("model_stats last_used_at order invalid: first=%v second=%v", payload.ModelStats[0].LastUsedAt, payload.ModelStats[1].LastUsedAt)
	}
}

func TestGetUsageStatistics_AppliesSummaryLimitAndOffset(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	stats := usage.NewRequestStatistics()
	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	records := []coreusage.Record{
		{
			APIKey:      "api-a",
			Model:       "model-a",
			RequestedAt: base.Add(5 * time.Minute),
			Detail:      coreusage.Detail{TotalTokens: 10},
		},
		{
			APIKey:      "api-b",
			Model:       "model-b",
			RequestedAt: base.Add(10 * time.Minute),
			Detail:      coreusage.Detail{TotalTokens: 20},
		},
		{
			APIKey:      "api-c",
			Model:       "model-c",
			RequestedAt: base.Add(15 * time.Minute),
			Detail:      coreusage.Detail{TotalTokens: 30},
		},
	}
	for _, record := range records {
		stats.Record(context.Background(), record)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{}, coreauth.NewManager(nil, nil, nil))
	h.SetUsageStatistics(stats)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/usage?api_sort_by=total_tokens&api_sort_order=desc&api_limit=1&api_offset=1&model_sort_by=total_tokens&model_sort_order=desc&model_limit=2",
		nil,
	)

	h.GetUsageStatistics(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetUsageStatistics status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		APIStats []struct {
			API string `json:"api"`
		} `json:"api_stats"`
		APIStatsPagination struct {
			Total    int  `json:"total"`
			Offset   int  `json:"offset"`
			Limit    int  `json:"limit"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"api_stats_pagination"`
		ModelStats []struct {
			Model string `json:"model"`
		} `json:"model_stats"`
		ModelStatsPagination struct {
			Total    int  `json:"total"`
			Offset   int  `json:"offset"`
			Limit    int  `json:"limit"`
			Returned int  `json:"returned"`
			HasMore  bool `json:"has_more"`
		} `json:"model_stats_pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(payload.APIStats) != 1 || payload.APIStats[0].API != "api-b" {
		t.Fatalf("api_stats = %+v, want single api-b item", payload.APIStats)
	}
	if payload.APIStatsPagination.Total != 3 || payload.APIStatsPagination.Offset != 1 || payload.APIStatsPagination.Limit != 1 || payload.APIStatsPagination.Returned != 1 || !payload.APIStatsPagination.HasMore {
		t.Fatalf("api_stats_pagination = %+v, want total=3 offset=1 limit=1 returned=1 has_more=true", payload.APIStatsPagination)
	}

	if len(payload.ModelStats) != 2 || payload.ModelStats[0].Model != "model-c" || payload.ModelStats[1].Model != "model-b" {
		t.Fatalf("model_stats = %+v, want model-c then model-b", payload.ModelStats)
	}
	if payload.ModelStatsPagination.Total != 3 || payload.ModelStatsPagination.Offset != 0 || payload.ModelStatsPagination.Limit != 2 || payload.ModelStatsPagination.Returned != 2 || !payload.ModelStatsPagination.HasMore {
		t.Fatalf("model_stats_pagination = %+v, want total=3 offset=0 limit=2 returned=2 has_more=true", payload.ModelStatsPagination)
	}
}

func TestGetUsageStatistics_AppliesSummaryFilters(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	stats := usage.NewRequestStatistics()
	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	records := []coreusage.Record{
		{
			APIKey:      "team-alpha",
			Model:       "gpt-5.4",
			RequestedAt: base.Add(5 * time.Minute),
			Detail:      coreusage.Detail{TotalTokens: 11},
		},
		{
			APIKey:      "team-alpha",
			Model:       "claude-3-7-sonnet",
			RequestedAt: base.Add(10 * time.Minute),
			Detail:      coreusage.Detail{TotalTokens: 22},
		},
		{
			APIKey:      "team-beta",
			Model:       "gpt-5.4",
			RequestedAt: base.Add(15 * time.Minute),
			Failed:      true,
			Detail:      coreusage.Detail{TotalTokens: 33},
		},
	}
	for _, record := range records {
		stats.Record(context.Background(), record)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{}, coreauth.NewManager(nil, nil, nil))
	h.SetUsageStatistics(stats)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/usage?api_filter=alpha&model_filter=gpt-5.4",
		nil,
	)

	h.GetUsageStatistics(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("GetUsageStatistics status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		APIStats []struct {
			API           string `json:"api"`
			TotalRequests int64  `json:"total_requests"`
			TotalTokens   int64  `json:"total_tokens"`
			ModelCount    int    `json:"model_count"`
			FailureCount  int64  `json:"failure_count"`
		} `json:"api_stats"`
		APIStatsPagination struct {
			Total int `json:"total"`
		} `json:"api_stats_pagination"`
		ModelStats []struct {
			Model         string `json:"model"`
			TotalRequests int64  `json:"total_requests"`
			TotalTokens   int64  `json:"total_tokens"`
			APICount      int    `json:"api_count"`
			FailureCount  int64  `json:"failure_count"`
		} `json:"model_stats"`
		ModelStatsPagination struct {
			Total int `json:"total"`
		} `json:"model_stats_pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(payload.APIStats) != 1 || payload.APIStatsPagination.Total != 1 {
		t.Fatalf("api_stats = %+v, pagination=%+v, want one filtered API item", payload.APIStats, payload.APIStatsPagination)
	}
	if payload.APIStats[0].API != "team-alpha" || payload.APIStats[0].TotalRequests != 1 || payload.APIStats[0].TotalTokens != 11 || payload.APIStats[0].ModelCount != 1 || payload.APIStats[0].FailureCount != 0 {
		t.Fatalf("api_stats[0] = %+v, want only team-alpha/gpt-5.4 stats", payload.APIStats[0])
	}

	if len(payload.ModelStats) != 1 || payload.ModelStatsPagination.Total != 1 {
		t.Fatalf("model_stats = %+v, pagination=%+v, want one filtered model item", payload.ModelStats, payload.ModelStatsPagination)
	}
	if payload.ModelStats[0].Model != "gpt-5.4" || payload.ModelStats[0].TotalRequests != 1 || payload.ModelStats[0].TotalTokens != 11 || payload.ModelStats[0].APICount != 1 || payload.ModelStats[0].FailureCount != 0 {
		t.Fatalf("model_stats[0] = %+v, want only team-alpha/gpt-5.4 stats", payload.ModelStats[0])
	}
}
