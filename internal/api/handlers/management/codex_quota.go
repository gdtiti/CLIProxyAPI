package management

import (
	"encoding/json"
	"net/http"
	"slices"
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
	Notes               map[string]any              `json:"notes"`
}

type codexAuthConfigRequest struct {
	CodexHeaderDefaults codexHeaderDefaultsResponse `json:"codex_header_defaults"`
	Payload             codexPayloadConfigResponse  `json:"payload"`
}

func (h *Handler) GetCodexAuthQuota(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": service.ListSnapshots()})
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
	c.JSON(http.StatusOK, gin.H{"events": service.ListEvents(authIndex, limit)})
}

func (h *Handler) GetCodexAuthUsage(c *gin.Context) {
	service := codexquota.DefaultService()
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "codex quota service unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"usage": service.ListRollups()})
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
		Notes: map[string]any{
			"custom_params":              "Use payload rules to add Codex-specific request fields such as instructions or custom context controls.",
			"one_million_context":        "This repository does not expose a dedicated 1m context field. Configure it through Codex payload rules if your upstream accepts it.",
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
