package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/codexquota"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestCodexQuotaHandlersExposePersistedDataAndConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	initialConfig := []byte("auth-dir: \"" + filepath.ToSlash(authDir) + "\"\n")
	if err := os.WriteFile(configPath, initialConfig, 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	cfg := &config.Config{
		AuthDir: authDir,
		CodexHeaderDefaults: config.CodexHeaderDefaults{
			UserAgent:    "codex-client",
			BetaFeatures: "beta-a",
		},
		Payload: config.PayloadConfig{
			Default: []config.PayloadRule{
				{Models: []config.PayloadModelRule{{Name: "gpt-*", Protocol: "codex"}}, Params: map[string]any{"instructions": "old"}},
				{Models: []config.PayloadModelRule{{Name: "gemini-*", Protocol: "gemini"}}, Params: map[string]any{"temperature": 1}},
			},
		},
	}

	service, err := codexquota.NewService(authDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	codexquota.SetDefaultService(service)
	t.Cleanup(func() { codexquota.SetDefaultService(nil) })

	manager := coreauth.NewManager(nil, nil, service.Hook())
	service.SetAuthManager(manager)
	handler := NewHandler(cfg, configPath, manager)

	auth := &coreauth.Auth{
		ID:       "codex-auth-1",
		Provider: "codex",
		FileName: "auths/alpha.json",
		Status:   coreauth.StatusActive,
		Metadata: map[string]any{
			"auth_method": "oauth",
			"email":       "alpha@example.com",
		},
	}
	if _, errRegister := manager.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("Register() error = %v", errRegister)
	}
	service.ApplyUsage(coreusage.Record{
		Provider:    "codex",
		AuthID:      auth.ID,
		AuthIndex:   auth.EnsureIndex(),
		RequestedAt: time.Now().UTC(),
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 4,
			TotalTokens:  14,
		},
	})

	router := gin.New()
	router.GET("/v0/management/codex-auth-quota", handler.GetCodexAuthQuota)
	router.GET("/v0/management/codex-auth-quota/:auth_index", handler.GetCodexAuthQuotaByIndex)
	router.GET("/v0/management/codex-auth-events", handler.GetCodexAuthEvents)
	router.GET("/v0/management/codex-auth-usage", handler.GetCodexAuthUsage)
	router.GET("/v0/management/codex-auth-config", handler.GetCodexAuthConfig)
	router.PUT("/v0/management/codex-auth-config", handler.PutCodexAuthConfig)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-quota", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("quota list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listBody struct {
		Accounts []codexquota.SnapshotView `json:"accounts"`
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &listBody); errDecode != nil {
		t.Fatalf("quota list decode error = %v", errDecode)
	}
	if len(listBody.Accounts) != 1 {
		t.Fatalf("len(accounts) = %d, want 1", len(listBody.Accounts))
	}
	if listBody.Accounts[0].Usage.TotalTokens != 14 {
		t.Fatalf("accounts[0].usage.total_tokens = %d, want 14", listBody.Accounts[0].Usage.TotalTokens)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-quota/"+auth.EnsureIndex(), nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("quota detail status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-config", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config get status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var configBody codexAuthConfigResponse
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &configBody); errDecode != nil {
		t.Fatalf("config decode error = %v", errDecode)
	}
	if len(configBody.Payload.Default) != 1 {
		t.Fatalf("len(configBody.Payload.Default) = %d, want 1 pure codex rule", len(configBody.Payload.Default))
	}
	if configBody.Guide.ContextWindows.GPT5MaxContextTokens != 1050000 {
		t.Fatalf("guide.context_windows.gpt5_max_context_tokens = %d, want 1050000", configBody.Guide.ContextWindows.GPT5MaxContextTokens)
	}
	if !configBody.Guide.ContextWindows.GPT5SupportsOfficialOneMillion {
		t.Fatalf("guide.context_windows.gpt5_supports_official_one_million = false, want true")
	}
	if configBody.Guide.ContextWindows.OfficialOneMillionRecommendedFamily != "gpt-5.4" {
		t.Fatalf("guide.context_windows.official_one_million_recommended_family = %q, want %q", configBody.Guide.ContextWindows.OfficialOneMillionRecommendedFamily, "gpt-5.4")
	}
	if len(configBody.Guide.FieldHints) == 0 {
		t.Fatalf("len(configBody.Guide.FieldHints) = 0, want hints")
	}
	if len(configBody.Guide.HeaderFields) < 2 {
		t.Fatalf("len(configBody.Guide.HeaderFields) = %d, want at least 2", len(configBody.Guide.HeaderFields))
	}
	if len(configBody.Guide.RuleTargets) < 5 {
		t.Fatalf("len(configBody.Guide.RuleTargets) = %d, want at least 5", len(configBody.Guide.RuleTargets))
	}
	if len(configBody.Guide.FieldGroups) < 4 {
		t.Fatalf("len(configBody.Guide.FieldGroups) = %d, want at least 4", len(configBody.Guide.FieldGroups))
	}
	if len(configBody.Guide.FilterPaths) < 4 {
		t.Fatalf("len(configBody.Guide.FilterPaths) = %d, want at least 4", len(configBody.Guide.FilterPaths))
	}
	if len(configBody.Guide.Presets) == 0 {
		t.Fatalf("len(configBody.Guide.Presets) = 0, want presets")
	}
	if !containsCodexFieldHint(configBody.Guide.FieldHints, "background") {
		t.Fatalf("guide.field_hints missing background")
	}
	if !containsCodexFieldHint(configBody.Guide.FieldHints, "truncation") {
		t.Fatalf("guide.field_hints missing truncation")
	}
	if !containsCodexFieldHint(configBody.Guide.FieldHints, "service_tier") {
		t.Fatalf("guide.field_hints missing service_tier")
	}
	if !containsCodexPreset(configBody.Guide.Presets, "background_long_tasks") {
		t.Fatalf("guide.presets missing background_long_tasks")
	}
	if configBody.Notes["one_million_context"] == nil {
		t.Fatalf("notes.one_million_context missing")
	}
	if !strings.Contains(configBody.Notes["one_million_context"].(string), "gpt-5.4") {
		t.Fatalf("notes.one_million_context = %q, want gpt-5.4 guidance", configBody.Notes["one_million_context"])
	}
	if !strings.Contains(configBody.Notes["one_million_context_config"].(string), "do not add a one_million_context flag") {
		t.Fatalf("notes.one_million_context_config = %q, want no-switch guidance", configBody.Notes["one_million_context_config"])
	}
	if configBody.Notes["long_context_behavior"] == nil {
		t.Fatalf("notes.long_context_behavior missing")
	}

	putBody := `{"codex_header_defaults":{"user_agent":"new-agent","beta_features":"beta-b"},"payload":{"default":[{"models":[{"name":"gpt-*","protocol":"codex"}],"params":{"instructions":"new"}}],"default_raw":[],"override":[],"override_raw":[],"filter":[]}}`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/v0/management/codex-auth-config", strings.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config put status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if cfg.CodexHeaderDefaults.UserAgent != "new-agent" {
		t.Fatalf("cfg.CodexHeaderDefaults.UserAgent = %q, want %q", cfg.CodexHeaderDefaults.UserAgent, "new-agent")
	}
	if len(cfg.Payload.Default) != 2 {
		t.Fatalf("len(cfg.Payload.Default) = %d, want 2", len(cfg.Payload.Default))
	}
	if cfg.Payload.Default[0].Models[0].Protocol != "gemini" {
		t.Fatalf("cfg.Payload.Default[0].Models[0].Protocol = %q, want gemini", cfg.Payload.Default[0].Models[0].Protocol)
	}
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(configPath) error = %v", err)
	}
	if !strings.Contains(string(configBytes), "new-agent") {
		t.Fatalf("updated config file does not contain new-agent: %s", string(configBytes))
	}
}

func TestCodexQuotaHandlers_SupportSortAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	service, err := codexquota.NewService(authDir)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	codexquota.SetDefaultService(service)
	t.Cleanup(func() { codexquota.SetDefaultService(nil) })

	manager := coreauth.NewManager(nil, nil, service.Hook())
	service.SetAuthManager(manager)
	handler := &Handler{}

	registerAuth := func(id, fileName, account string) *coreauth.Auth {
		t.Helper()
		auth := &coreauth.Auth{
			ID:       id,
			Provider: "codex",
			FileName: fileName,
			Status:   coreauth.StatusActive,
			Metadata: map[string]any{
				"auth_method": "oauth",
				"email":       account + "@example.com",
			},
		}
		if _, errRegister := manager.Register(context.Background(), auth); errRegister != nil {
			t.Fatalf("Register(%s) error = %v", id, errRegister)
		}
		return auth
	}

	alpha := registerAuth("codex-auth-alpha", "alpha.json", "alpha")
	time.Sleep(10 * time.Millisecond)
	beta := registerAuth("codex-auth-beta", "beta.json", "beta")

	service.ApplyUsage(coreusage.Record{
		Provider:    "codex",
		AuthID:      alpha.ID,
		AuthIndex:   alpha.EnsureIndex(),
		RequestedAt: time.Now().UTC().Add(-time.Minute),
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	})
	service.ApplyUsage(coreusage.Record{
		Provider:    "codex",
		AuthID:      beta.ID,
		AuthIndex:   beta.EnsureIndex(),
		RequestedAt: time.Now().UTC(),
		Detail: coreusage.Detail{
			InputTokens:  20,
			OutputTokens: 7,
			TotalTokens:  27,
		},
	})
	service.ApplyUsage(coreusage.Record{
		Provider:    "codex",
		AuthID:      beta.ID,
		AuthIndex:   beta.EnsureIndex(),
		RequestedAt: time.Now().UTC().Add(time.Second),
		Detail: coreusage.Detail{
			InputTokens:  5,
			OutputTokens: 3,
			TotalTokens:  8,
		},
	})

	router := gin.New()
	router.GET("/v0/management/codex-auth-quota", handler.GetCodexAuthQuota)
	router.GET("/v0/management/codex-auth-events", handler.GetCodexAuthEvents)
	router.GET("/v0/management/codex-auth-usage", handler.GetCodexAuthUsage)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-quota?sort_by=total_tokens&sort_order=desc&page=1&page_size=1", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("quota status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var quotaBody struct {
		Accounts  []codexquota.SnapshotView `json:"accounts"`
		Total     int                       `json:"total"`
		Page      int                       `json:"page"`
		PageSize  int                       `json:"page_size"`
		SortBy    string                    `json:"sort_by"`
		SortOrder string                    `json:"sort_order"`
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &quotaBody); errDecode != nil {
		t.Fatalf("quota decode error = %v", errDecode)
	}
	if quotaBody.Total != 2 || quotaBody.Page != 1 || quotaBody.PageSize != 1 {
		t.Fatalf("quota page info = (%d,%d,%d), want (2,1,1)", quotaBody.Total, quotaBody.Page, quotaBody.PageSize)
	}
	if len(quotaBody.Accounts) != 1 || quotaBody.Accounts[0].AuthID != beta.ID {
		t.Fatalf("quota accounts[0].auth_id = %v, want %q", quotaBody.Accounts, beta.ID)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-usage?sort_by=request_count&sort_order=desc", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("usage status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var usageBody struct {
		Usage []codexquota.UsageRollup `json:"usage"`
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &usageBody); errDecode != nil {
		t.Fatalf("usage decode error = %v", errDecode)
	}
	if len(usageBody.Usage) != 2 {
		t.Fatalf("len(usage) = %d, want 2", len(usageBody.Usage))
	}
	if usageBody.Usage[0].AuthID != beta.ID {
		t.Fatalf("usage[0].auth_id = %q, want %q", usageBody.Usage[0].AuthID, beta.ID)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-events?sort_order=asc", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("events status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var eventBody struct {
		Events []codexquota.Event `json:"events"`
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &eventBody); errDecode != nil {
		t.Fatalf("events decode error = %v", errDecode)
	}
	if len(eventBody.Events) < 2 {
		t.Fatalf("len(events) = %d, want >= 2", len(eventBody.Events))
	}
	if eventBody.Events[0].CreatedAt.After(eventBody.Events[len(eventBody.Events)-1].CreatedAt) {
		t.Fatalf("events not sorted ascending by created_at")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-quota?keyword=beta&quota_exceeded=false", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("quota filter status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &quotaBody); errDecode != nil {
		t.Fatalf("quota filter decode error = %v", errDecode)
	}
	if len(quotaBody.Accounts) != 1 || quotaBody.Accounts[0].AuthID != beta.ID {
		t.Fatalf("quota filtered accounts = %v, want only %q", quotaBody.Accounts, beta.ID)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-usage?keyword=alpha", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("usage filter status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &usageBody); errDecode != nil {
		t.Fatalf("usage filter decode error = %v", errDecode)
	}
	if len(usageBody.Usage) != 1 || usageBody.Usage[0].AuthID != alpha.ID {
		t.Fatalf("usage filtered entries = %v, want only %q", usageBody.Usage, alpha.ID)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-events?auth_id="+beta.ID+"&event_type=registered&quota_exceeded=false", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("events filter status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &eventBody); errDecode != nil {
		t.Fatalf("events filter decode error = %v", errDecode)
	}
	if len(eventBody.Events) != 1 {
		t.Fatalf("len(filtered events) = %d, want 1", len(eventBody.Events))
	}
	for _, event := range eventBody.Events {
		if event.AuthID != beta.ID || event.EventType != "registered" || event.QuotaExceeded {
			t.Fatalf("unexpected filtered event = %+v", event)
		}
	}
}

func TestCodexQuotaHandlers_RejectInvalidSortBy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service, err := codexquota.NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	codexquota.SetDefaultService(service)
	t.Cleanup(func() { codexquota.SetDefaultService(nil) })

	router := gin.New()
	handler := &Handler{}
	router.GET("/v0/management/codex-auth-quota", handler.GetCodexAuthQuota)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-quota?sort_by=unknown", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("quota status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex-auth-quota?quota_exceeded=maybe", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("quota invalid bool status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func containsCodexFieldHint(hints []codexPayloadFieldHintResponse, path string) bool {
	for _, hint := range hints {
		if hint.Path == path {
			return true
		}
	}
	return false
}

func containsCodexPreset(presets []codexPayloadPresetResponse, id string) bool {
	for _, preset := range presets {
		if preset.ID == id {
			return true
		}
	}
	return false
}
