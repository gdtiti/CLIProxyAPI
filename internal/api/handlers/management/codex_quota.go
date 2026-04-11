package management

import (
	"encoding/json"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/codexquota"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

type codexPayloadConfigResponse struct {
	Default     []config.PayloadRule       `json:"default"`
	DefaultRaw  []config.PayloadRule       `json:"default_raw"`
	Override    []config.PayloadRule       `json:"override"`
	OverrideRaw []config.PayloadRule       `json:"override_raw"`
	Filter      []config.PayloadFilterRule `json:"filter"`
}

type codexHeaderDefaultsResponse struct {
	UserAgent    string `json:"user_agent"`
	BetaFeatures string `json:"beta_features"`
}

type codexAuthConfigResponse struct {
	CodexHeaderDefaults codexHeaderDefaultsResponse `json:"codex_header_defaults"`
	Payload             codexPayloadConfigResponse  `json:"payload"`
	Guide               codexConfigGuideResponse    `json:"guide"`
	Notes               map[string]any              `json:"notes"`
}

type codexAuthConfigRequest struct {
	CodexHeaderDefaults codexHeaderDefaultsResponse `json:"codex_header_defaults"`
	Payload             codexPayloadConfigResponse  `json:"payload"`
}

type codexConfigGuideResponse struct {
	ContextWindows codexContextWindowsGuideResponse `json:"context_windows"`
	HeaderFields   []codexHeaderFieldHintResponse   `json:"header_fields"`
	RuleTargets    []codexRuleTargetGuideResponse   `json:"rule_targets"`
	FieldGroups    []codexFieldGroupResponse        `json:"field_groups"`
	FieldHints     []codexPayloadFieldHintResponse  `json:"field_hints"`
	FilterPaths    []codexFilterPathHintResponse    `json:"filter_paths"`
	Presets        []codexPayloadPresetResponse     `json:"presets"`
	OfficialDocs   map[string]string                `json:"official_docs"`
}

type codexContextWindowsGuideResponse struct {
	GPT5MaxContextTokens                int    `json:"gpt5_max_context_tokens"`
	GPT41MaxContextTokens               int    `json:"gpt41_max_context_tokens"`
	GPT5SupportsOfficialOneMillion      bool   `json:"gpt5_supports_official_one_million"`
	OfficialOneMillionRecommendedFamily string `json:"official_one_million_recommended_family"`
}

type codexPayloadFieldHintResponse struct {
	Path        string   `json:"path"`
	Label       string   `json:"label"`
	ValueType   string   `json:"value_type"`
	RuleTargets []string `json:"rule_targets"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Example     any      `json:"example,omitempty"`
	Official    bool     `json:"official"`
}

type codexHeaderFieldHintResponse struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	ValueType   string `json:"value_type"`
	Description string `json:"description"`
	Example     any    `json:"example,omitempty"`
}

type codexRuleTargetGuideResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Raw         bool   `json:"raw"`
	Description string `json:"description"`
}

type codexFieldGroupResponse struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	RuleTargets []string `json:"rule_targets,omitempty"`
	Paths       []string `json:"paths"`
}

type codexFilterPathHintResponse struct {
	Path        string `json:"path"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type codexPayloadPresetResponse struct {
	ID          string                    `json:"id"`
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	RuleTarget  string                    `json:"rule_target"`
	Raw         bool                      `json:"raw"`
	Official    bool                      `json:"official"`
	Models      []config.PayloadModelRule `json:"models"`
	Params      map[string]any            `json:"params,omitempty"`
	Paths       []string                  `json:"paths,omitempty"`
}

func (h *Handler) GetCodexAuthQuota(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	items := service.ListSnapshots()
	sortBy, sortOrder, err := parseListSort(c, "auth_index", "asc", allowedCodexQuotaSortFields)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sort.Slice(items, func(i, j int) bool {
		diff := compareCodexSnapshotViews(items[i], items[j], sortBy)
		if diff == 0 {
			diff = compareCodexSnapshotViews(items[i], items[j], "auth_index")
		}
		if diff == 0 {
			diff = compareStringsFold(items[i].AuthID, items[j].AuthID)
		}
		if sortOrder == "desc" {
			return diff > 0
		}
		return diff < 0
	})
	paged, pageInfo, err := parseListPageInfo(c, items, sortBy, sortOrder)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"accounts":   paged,
		"total":      pageInfo.Total,
		"page":       pageInfo.Page,
		"page_size":  pageInfo.PageSize,
		"sort_by":    pageInfo.SortBy,
		"sort_order": pageInfo.SortOrder,
	})
}

func (h *Handler) GetCodexAuthQuotaByIndex(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	authIndex := strings.TrimSpace(c.Param("auth_index"))
	if authIndex == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing auth_index"})
		return
	}
	snapshot, ok := service.GetSnapshot(authIndex)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "codex auth not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"snapshot": snapshot,
		"events":   service.ListEvents(authIndex, 50),
	})
}

func (h *Handler) GetCodexAuthEvents(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = value
	}
	authIndex := strings.TrimSpace(c.Query("auth_index"))
	items := service.ListEvents(authIndex, 0)
	sortBy, sortOrder, err := parseListSort(c, "created_at", "desc", allowedCodexEventSortFields)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sort.Slice(items, func(i, j int) bool {
		diff := compareCodexEvents(items[i], items[j], sortBy)
		if diff == 0 {
			diff = compareTimes(items[i].CreatedAt, items[j].CreatedAt)
		}
		if diff == 0 {
			diff = compareStringsFold(items[i].ID, items[j].ID)
		}
		if sortOrder == "desc" {
			return diff > 0
		}
		return diff < 0
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	paged, pageInfo, err := parseListPageInfo(c, items, sortBy, sortOrder)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"events":     paged,
		"total":      pageInfo.Total,
		"page":       pageInfo.Page,
		"page_size":  pageInfo.PageSize,
		"sort_by":    pageInfo.SortBy,
		"sort_order": pageInfo.SortOrder,
		"limit":      limit,
	})
}

func (h *Handler) GetCodexAuthUsage(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	items := service.ListRollups()
	sortBy, sortOrder, err := parseListSort(c, "auth_index", "asc", allowedCodexUsageSortFields)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sort.Slice(items, func(i, j int) bool {
		diff := compareCodexUsageRollups(items[i], items[j], sortBy)
		if diff == 0 {
			diff = compareCodexUsageRollups(items[i], items[j], "auth_index")
		}
		if diff == 0 {
			diff = compareStringsFold(items[i].AuthID, items[j].AuthID)
		}
		if sortOrder == "desc" {
			return diff > 0
		}
		return diff < 0
	})
	paged, pageInfo, err := parseListPageInfo(c, items, sortBy, sortOrder)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"usage":      paged,
		"total":      pageInfo.Total,
		"page":       pageInfo.Page,
		"page_size":  pageInfo.PageSize,
		"sort_by":    pageInfo.SortBy,
		"sort_order": pageInfo.SortOrder,
	})
}

func (h *Handler) GetCodexAuthConfig(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	c.JSON(http.StatusOK, codexAuthConfigResponse{
		CodexHeaderDefaults: codexHeaderDefaultsResponse{
			UserAgent:    h.cfg.CodexHeaderDefaults.UserAgent,
			BetaFeatures: h.cfg.CodexHeaderDefaults.BetaFeatures,
		},
		Payload: codexPayloadConfigResponse{
			Default:     filterCodexPayloadRules(h.cfg.Payload.Default),
			DefaultRaw:  filterCodexPayloadRules(h.cfg.Payload.DefaultRaw),
			Override:    filterCodexPayloadRules(h.cfg.Payload.Override),
			OverrideRaw: filterCodexPayloadRules(h.cfg.Payload.OverrideRaw),
			Filter:      filterCodexPayloadFilterRules(h.cfg.Payload.Filter),
		},
		Guide: codexConfigGuideResponse{
			ContextWindows: codexContextWindowsGuideResponse{
				GPT5MaxContextTokens:                1050000,
				GPT41MaxContextTokens:               1047576,
				GPT5SupportsOfficialOneMillion:      true,
				OfficialOneMillionRecommendedFamily: "gpt-5.4",
			},
			HeaderFields: []codexHeaderFieldHintResponse{
				{
					ID:          "user_agent",
					Label:       "User-Agent",
					ValueType:   "string",
					Description: "Overrides the Codex/OpenAI User-Agent header sent upstream.",
					Example:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Codex/1.0",
				},
				{
					ID:          "beta_features",
					Label:       "Beta features",
					ValueType:   "string",
					Description: "Comma-separated beta feature header value for Codex/OpenAI upstreams that require it.",
					Example:     "responses-v1,reasoning_summaries",
				},
			},
			RuleTargets: []codexRuleTargetGuideResponse{
				{
					ID:          "default",
					Title:       "payload.default",
					Raw:         false,
					Description: "Set a field only when the incoming payload does not already contain it.",
				},
				{
					ID:          "default_raw",
					Title:       "payload.default_raw",
					Raw:         true,
					Description: "Set a missing field using a raw JSON fragment string.",
				},
				{
					ID:          "override",
					Title:       "payload.override",
					Raw:         false,
					Description: "Always overwrite the target field with the configured value.",
				},
				{
					ID:          "override_raw",
					Title:       "payload.override_raw",
					Raw:         true,
					Description: "Always overwrite the target field using a raw JSON fragment string.",
				},
				{
					ID:          "filter",
					Title:       "payload.filter",
					Raw:         false,
					Description: "Remove fields from the outgoing payload by JSON path.",
				},
			},
			FieldGroups: []codexFieldGroupResponse{
				{
					ID:          "request_basics",
					Title:       "Request basics",
					Description: "Common request controls that are safe to expose as standard form fields.",
					RuleTargets: []string{"default", "override"},
					Paths:       []string{"instructions", "max_output_tokens", "store", "background", "truncation", "safety_identifier", "service_tier"},
				},
				{
					ID:          "gpt5_controls",
					Title:       "GPT-5 controls",
					Description: "GPT-5 family options documented in the Responses API.",
					RuleTargets: []string{"default", "override"},
					Paths:       []string{"reasoning.effort", "reasoning.summary", "text.verbosity"},
				},
				{
					ID:          "structured_output",
					Title:       "Structured output",
					Description: "Raw JSON settings that should use a structured editor instead of a plain text box.",
					RuleTargets: []string{"default_raw", "override_raw"},
					Paths:       []string{"response_format"},
				},
				{
					ID:          "payload_filter",
					Title:       "Filter paths",
					Description: "Common payload paths that can be removed before forwarding the request upstream.",
					RuleTargets: []string{"filter"},
					Paths:       []string{"parallel_tool_calls", "response_format", "reasoning.effort", "reasoning.summary", "text.verbosity", "store", "background", "service_tier"},
				},
			},
			FieldHints: []codexPayloadFieldHintResponse{
				{
					Path:        "instructions",
					Label:       "Instructions",
					ValueType:   "string",
					RuleTargets: []string{"default", "override"},
					Description: "Sets top-level instructions for Codex/OpenAI requests.",
					Example:     "Prefer concise answers and keep markdown minimal.",
					Official:    true,
				},
				{
					Path:        "reasoning.effort",
					Label:       "Reasoning effort",
					ValueType:   "string",
					RuleTargets: []string{"override"},
					Description: "Controls GPT-5 reasoning depth.",
					Enum:        []string{"minimal", "low", "medium", "high"},
					Example:     "high",
					Official:    true,
				},
				{
					Path:        "reasoning.summary",
					Label:       "Reasoning summary",
					ValueType:   "string",
					RuleTargets: []string{"default", "override"},
					Description: "Requests a reasoning summary in the Responses API. Use auto unless your upstream documents a narrower mode.",
					Example:     "auto",
					Official:    true,
				},
				{
					Path:        "text.verbosity",
					Label:       "Text verbosity",
					ValueType:   "string",
					RuleTargets: []string{"override"},
					Description: "Controls GPT-5 response verbosity.",
					Enum:        []string{"low", "medium", "high"},
					Example:     "low",
					Official:    true,
				},
				{
					Path:        "max_output_tokens",
					Label:       "Max output tokens",
					ValueType:   "number",
					RuleTargets: []string{"default", "override"},
					Description: "Caps response output tokens for supported Codex/OpenAI requests.",
					Example:     32768,
					Official:    true,
				},
				{
					Path:        "response_format",
					Label:       "Response format",
					ValueType:   "raw_json",
					RuleTargets: []string{"default_raw", "override_raw"},
					Description: "Sets a raw JSON response format object. Use only when your upstream expects the Responses API response_format structure.",
					Example:     "{\"type\":\"json_schema\",\"json_schema\":{\"name\":\"answer\",\"schema\":{\"type\":\"object\"}}}",
					Official:    true,
				},
				{
					Path:        "store",
					Label:       "Store response",
					ValueType:   "boolean",
					RuleTargets: []string{"default", "override"},
					Description: "Controls whether the upstream stores the response for later retrieval when supported.",
					Example:     false,
					Official:    true,
				},
				{
					Path:        "background",
					Label:       "Background mode",
					ValueType:   "boolean",
					RuleTargets: []string{"default", "override"},
					Description: "Runs the request asynchronously through the Responses API background mode. Background mode requires store=true.",
					Example:     true,
					Official:    true,
				},
				{
					Path:        "truncation",
					Label:       "Truncation strategy",
					ValueType:   "string",
					RuleTargets: []string{"default", "override"},
					Description: "Controls what the Responses API does when the input would exceed the model context window.",
					Enum:        []string{"disabled", "auto"},
					Example:     "auto",
					Official:    true,
				},
				{
					Path:        "safety_identifier",
					Label:       "Safety identifier",
					ValueType:   "string",
					RuleTargets: []string{"default", "override"},
					Description: "Stable end-user identifier for OpenAI safety systems. Hash your user identifier before sending it upstream.",
					Example:     "user_123456",
					Official:    true,
				},
				{
					Path:        "service_tier",
					Label:       "Service tier",
					ValueType:   "string",
					RuleTargets: []string{"default", "override"},
					Description: "Request-level processing tier for supported OpenAI endpoints. Use priority only when the upstream project is enabled for it.",
					Example:     "priority",
					Official:    true,
				},
			},
			FilterPaths: []codexFilterPathHintResponse{
				{
					Path:        "parallel_tool_calls",
					Label:       "parallel_tool_calls",
					Description: "Remove this when the upstream does not support parallel tool calls.",
				},
				{
					Path:        "response_format",
					Label:       "response_format",
					Description: "Remove structured output settings for upstreams that only accept plain text.",
				},
				{
					Path:        "reasoning.effort",
					Label:       "reasoning.effort",
					Description: "Remove this when the target model ignores or rejects reasoning controls.",
				},
				{
					Path:        "reasoning.summary",
					Label:       "reasoning.summary",
					Description: "Remove this when the upstream does not expose reasoning summaries.",
				},
				{
					Path:        "text.verbosity",
					Label:       "text.verbosity",
					Description: "Remove this when the upstream does not support GPT-5 verbosity controls.",
				},
				{
					Path:        "store",
					Label:       "store",
					Description: "Remove this when an upstream enforces its own storage policy.",
				},
				{
					Path:        "background",
					Label:       "background",
					Description: "Remove this when the upstream does not support background responses.",
				},
				{
					Path:        "service_tier",
					Label:       "service_tier",
					Description: "Remove this when the upstream ignores or rejects priority processing fields.",
				},
			},
			Presets: []codexPayloadPresetResponse{
				{
					ID:          "gpt5_high_reasoning",
					Title:       "GPT-5 high reasoning",
					Description: "Use payload.override to raise GPT-5 reasoning effort.",
					RuleTarget:  "override",
					Raw:         false,
					Official:    true,
					Models:      []config.PayloadModelRule{{Name: "gpt-5*", Protocol: "codex"}},
					Params:      map[string]any{"reasoning.effort": "high"},
				},
				{
					ID:          "gpt5_low_verbosity",
					Title:       "GPT-5 low verbosity",
					Description: "Use payload.override to make GPT-5 outputs shorter.",
					RuleTarget:  "override",
					Raw:         false,
					Official:    true,
					Models:      []config.PayloadModelRule{{Name: "gpt-5*", Protocol: "codex"}},
					Params:      map[string]any{"text.verbosity": "low"},
				},
				{
					ID:          "shared_instructions",
					Title:       "Shared instructions",
					Description: "Use payload.default to inject shared instructions when the caller does not provide them.",
					RuleTarget:  "default",
					Raw:         false,
					Official:    true,
					Models:      []config.PayloadModelRule{{Name: "*", Protocol: "codex"}},
					Params:      map[string]any{"instructions": "Keep answers concise."},
				},
				{
					ID:          "responses_json_schema",
					Title:       "Responses JSON schema",
					Description: "Use payload.override_raw to force a response_format JSON schema object.",
					RuleTarget:  "override_raw",
					Raw:         true,
					Official:    true,
					Models:      []config.PayloadModelRule{{Name: "gpt-*", Protocol: "codex"}},
					Params: map[string]any{
						"response_format": "{\"type\":\"json_schema\",\"json_schema\":{\"name\":\"answer\",\"schema\":{\"type\":\"object\",\"properties\":{\"result\":{\"type\":\"string\"}},\"required\":[\"result\"]}}}",
					},
				},
				{
					ID:          "drop_parallel_tool_calls",
					Title:       "Remove parallel_tool_calls",
					Description: "Use payload.filter to strip a field before the request reaches an upstream that does not support it.",
					RuleTarget:  "filter",
					Raw:         false,
					Official:    false,
					Models:      []config.PayloadModelRule{{Name: "*", Protocol: "codex"}},
					Paths:       []string{"parallel_tool_calls"},
				},
				{
					ID:          "background_long_tasks",
					Title:       "Background long tasks",
					Description: "Use payload.override to enable background mode together with store=true for long-running GPT-5 style requests.",
					RuleTarget:  "override",
					Raw:         false,
					Official:    true,
					Models:      []config.PayloadModelRule{{Name: "gpt-5*", Protocol: "codex"}},
					Params:      map[string]any{"background": true, "store": true},
				},
				{
					ID:          "priority_processing",
					Title:       "Priority processing",
					Description: "Use payload.override to request service_tier=priority on supported OpenAI upstreams.",
					RuleTarget:  "override",
					Raw:         false,
					Official:    true,
					Models:      []config.PayloadModelRule{{Name: "gpt-*", Protocol: "codex"}},
					Params:      map[string]any{"service_tier": "priority"},
				},
				{
					ID:          "reasoning_summary_auto",
					Title:       "Reasoning summary auto",
					Description: "Use payload.override to request reasoning summaries when the upstream supports them.",
					RuleTarget:  "override",
					Raw:         false,
					Official:    true,
					Models:      []config.PayloadModelRule{{Name: "gpt-5*", Protocol: "codex"}},
					Params:      map[string]any{"reasoning.summary": "auto"},
				},
			},
			OfficialDocs: map[string]string{
				"background":      "https://platform.openai.com/docs/guides/background",
				"models":          "https://platform.openai.com/docs/models",
				"priority":        "https://platform.openai.com/docs/guides/priority-processing",
				"reasoning":       "https://platform.openai.com/docs/guides/reasoning",
				"safety":          "https://platform.openai.com/docs/guides/safety-checks",
				"text_generation": "https://platform.openai.com/docs/guides/text?api-mode=responses",
				"responses_api":   "https://platform.openai.com/docs/api-reference/responses",
			},
		},
		Notes: map[string]any{
			"custom_params":              "Use payload rules to add Codex-specific request fields or upstream-specific custom fields.",
			"long_context_behavior":      "payload.override.truncation=auto only tells the Responses API to trim older input items when a request would exceed the model context window. It does not enable 1M context for models that do not already support it.",
			"one_million_context":        "OpenAI public docs now list gpt-5.4 with a 1.05M-token context window. Official 1M+ context is a model capability, not a separate payload.override switch.",
			"one_million_context_config": "For official OpenAI gpt-5.4, do not add a one_million_context flag. Use payload.override only for normal request fields such as reasoning.effort, text.verbosity, truncation, or service_tier. Only add a custom 1m field when a non-OpenAI compatible upstream explicitly documents one.",
			"recovered_tokens_available": false,
		},
	})
}

func (h *Handler) PutCodexAuthConfig(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	if strings.TrimSpace(h.configFilePath) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config file path unavailable"})
		return
	}

	var body codexAuthConfigRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body", "message": err.Error()})
		return
	}
	if err := validateCodexPayloadRequest(body.Payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload rules", "message": err.Error()})
		return
	}

	headerDefaults := config.CodexHeaderDefaults{
		UserAgent:    strings.TrimSpace(body.CodexHeaderDefaults.UserAgent),
		BetaFeatures: strings.TrimSpace(body.CodexHeaderDefaults.BetaFeatures),
	}

	h.mu.Lock()
	h.cfg.CodexHeaderDefaults = headerDefaults
	h.cfg.Payload.Default = replaceCodexPayloadRules(h.cfg.Payload.Default, body.Payload.Default)
	h.cfg.Payload.DefaultRaw = replaceCodexPayloadRules(h.cfg.Payload.DefaultRaw, body.Payload.DefaultRaw)
	h.cfg.Payload.Override = replaceCodexPayloadRules(h.cfg.Payload.Override, body.Payload.Override)
	h.cfg.Payload.OverrideRaw = replaceCodexPayloadRules(h.cfg.Payload.OverrideRaw, body.Payload.OverrideRaw)
	h.cfg.Payload.Filter = replaceCodexPayloadFilterRules(h.cfg.Payload.Filter, body.Payload.Filter)
	h.cfg.SanitizeCodexHeaderDefaults()
	h.cfg.SanitizePayloadRules()
	h.mu.Unlock()

	h.persist(c)
}

var allowedCodexQuotaSortFields = map[string]struct{}{
	"account":           {},
	"auth_id":           {},
	"auth_index":        {},
	"last_refreshed_at": {},
	"last_requested_at": {},
	"next_recover_at":   {},
	"quota_exceeded":    {},
	"request_count":     {},
	"status":            {},
	"total_tokens":      {},
	"updated_at":        {},
}

var allowedCodexUsageSortFields = map[string]struct{}{
	"account":           {},
	"auth_id":           {},
	"auth_index":        {},
	"avg_total_tokens":  {},
	"cached_tokens":     {},
	"input_tokens":      {},
	"last_requested_at": {},
	"output_tokens":     {},
	"reasoning_tokens":  {},
	"request_count":     {},
	"total_tokens":      {},
	"updated_at":        {},
}

var allowedCodexEventSortFields = map[string]struct{}{
	"auth_id":        {},
	"auth_index":     {},
	"created_at":     {},
	"event_type":     {},
	"http_status":    {},
	"quota_exceeded": {},
	"request_count":  {},
	"total_tokens":   {},
}

func compareCodexSnapshotViews(left, right codexquota.SnapshotView, sortBy string) int {
	switch sortBy {
	case "account":
		return compareStringsFold(left.Account, right.Account)
	case "auth_id":
		return compareStringsFold(left.AuthID, right.AuthID)
	case "last_refreshed_at":
		return compareTimes(left.LastRefreshedAt, right.LastRefreshedAt)
	case "last_requested_at":
		return compareTimes(left.Usage.LastRequestedAt, right.Usage.LastRequestedAt)
	case "next_recover_at":
		return compareTimes(left.NextRecoverAt, right.NextRecoverAt)
	case "quota_exceeded":
		return compareBools(left.QuotaExceeded, right.QuotaExceeded)
	case "request_count":
		return compareInt64s(left.Usage.RequestCount, right.Usage.RequestCount)
	case "status":
		return compareStringsFold(left.Status, right.Status)
	case "total_tokens":
		return compareInt64s(left.Usage.TotalTokens, right.Usage.TotalTokens)
	case "updated_at":
		return compareTimes(left.UpdatedAt, right.UpdatedAt)
	case "auth_index":
		fallthrough
	default:
		return compareStringsFold(left.AuthIndex, right.AuthIndex)
	}
}

func compareCodexUsageRollups(left, right codexquota.UsageRollup, sortBy string) int {
	switch sortBy {
	case "account":
		return compareStringsFold(left.Account, right.Account)
	case "auth_id":
		return compareStringsFold(left.AuthID, right.AuthID)
	case "avg_total_tokens":
		return compareFloat64s(left.AvgTotalTokens, right.AvgTotalTokens)
	case "cached_tokens":
		return compareInt64s(left.CachedTokens, right.CachedTokens)
	case "input_tokens":
		return compareInt64s(left.InputTokens, right.InputTokens)
	case "last_requested_at":
		return compareTimes(left.LastRequestedAt, right.LastRequestedAt)
	case "output_tokens":
		return compareInt64s(left.OutputTokens, right.OutputTokens)
	case "reasoning_tokens":
		return compareInt64s(left.ReasoningTokens, right.ReasoningTokens)
	case "request_count":
		return compareInt64s(left.RequestCount, right.RequestCount)
	case "total_tokens":
		return compareInt64s(left.TotalTokens, right.TotalTokens)
	case "updated_at":
		return compareTimes(left.UpdatedAt, right.UpdatedAt)
	case "auth_index":
		fallthrough
	default:
		return compareStringsFold(left.AuthIndex, right.AuthIndex)
	}
}

func compareCodexEvents(left, right codexquota.Event, sortBy string) int {
	switch sortBy {
	case "auth_id":
		return compareStringsFold(left.AuthID, right.AuthID)
	case "auth_index":
		return compareStringsFold(left.AuthIndex, right.AuthIndex)
	case "event_type":
		return compareStringsFold(left.EventType, right.EventType)
	case "http_status":
		return compareInts(left.HTTPStatus, right.HTTPStatus)
	case "quota_exceeded":
		return compareBools(left.QuotaExceeded, right.QuotaExceeded)
	case "request_count":
		return compareInt64s(left.RequestCount, right.RequestCount)
	case "total_tokens":
		return compareInt64s(left.TotalTokens, right.TotalTokens)
	case "created_at":
		fallthrough
	default:
		return compareTimes(left.CreatedAt, right.CreatedAt)
	}
}

func filterCodexPayloadRules(rules []config.PayloadRule) []config.PayloadRule {
	filtered := make([]config.PayloadRule, 0, len(rules))
	for _, rule := range rules {
		if isPureCodexPayloadRule(rule.Models) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func filterCodexPayloadFilterRules(rules []config.PayloadFilterRule) []config.PayloadFilterRule {
	filtered := make([]config.PayloadFilterRule, 0, len(rules))
	for _, rule := range rules {
		if isPureCodexPayloadRule(rule.Models) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func replaceCodexPayloadRules(existing []config.PayloadRule, incoming []config.PayloadRule) []config.PayloadRule {
	out := make([]config.PayloadRule, 0, len(existing)+len(incoming))
	for _, rule := range existing {
		if !isPureCodexPayloadRule(rule.Models) {
			out = append(out, rule)
		}
	}
	out = append(out, incoming...)
	return out
}

func replaceCodexPayloadFilterRules(existing []config.PayloadFilterRule, incoming []config.PayloadFilterRule) []config.PayloadFilterRule {
	out := make([]config.PayloadFilterRule, 0, len(existing)+len(incoming))
	for _, rule := range existing {
		if !isPureCodexPayloadRule(rule.Models) {
			out = append(out, rule)
		}
	}
	out = append(out, incoming...)
	return out
}

func validateCodexPayloadRequest(payload codexPayloadConfigResponse) error {
	for _, rule := range slices.Concat(payload.Default, payload.DefaultRaw, payload.Override, payload.OverrideRaw) {
		if !isPureCodexPayloadRule(rule.Models) {
			return errInvalidCodexRule("payload rule must contain protocol=codex for every model")
		}
	}
	for _, rule := range payload.Filter {
		if !isPureCodexPayloadRule(rule.Models) {
			return errInvalidCodexRule("payload filter rule must contain protocol=codex for every model")
		}
	}
	for _, rule := range slices.Concat(payload.DefaultRaw, payload.OverrideRaw) {
		for _, value := range rule.Params {
			raw, ok := value.(string)
			if !ok {
				continue
			}
			if !json.Valid([]byte(strings.TrimSpace(raw))) {
				return errInvalidCodexRule("raw payload values must be valid JSON strings")
			}
		}
	}
	return nil
}

func isPureCodexPayloadRule(models []config.PayloadModelRule) bool {
	if len(models) == 0 {
		return false
	}
	for _, model := range models {
		if !strings.EqualFold(strings.TrimSpace(model.Protocol), "codex") {
			return false
		}
	}
	return true
}

type invalidCodexRuleError string

func (e invalidCodexRuleError) Error() string { return string(e) }

func errInvalidCodexRule(message string) error {
	return invalidCodexRuleError(message)
}
