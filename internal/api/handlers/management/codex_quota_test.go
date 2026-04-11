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
	t.Parallel()
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
	if configBody.Guide.ContextWindows.GPT5MaxContextTokens != 400000 {
		t.Fatalf("guide.context_windows.gpt5_max_context_tokens = %d, want 400000", configBody.Guide.ContextWindows.GPT5MaxContextTokens)
	}
	if configBody.Guide.ContextWindows.GPT5SupportsOfficialOneMillion {
		t.Fatalf("guide.context_windows.gpt5_supports_official_one_million = true, want false")
	}
	if configBody.Guide.ContextWindows.OfficialOneMillionRecommendedFamily != "gpt-4.1" {
		t.Fatalf("guide.context_windows.official_one_million_recommended_family = %q, want %q", configBody.Guide.ContextWindows.OfficialOneMillionRecommendedFamily, "gpt-4.1")
	}
	if len(configBody.Guide.FieldHints) == 0 {
		t.Fatalf("len(configBody.Guide.FieldHints) = 0, want hints")
	}
	if len(configBody.Guide.RuleTargets) < 5 {
		t.Fatalf("len(configBody.Guide.RuleTargets) = %d, want at least 5", len(configBody.Guide.RuleTargets))
	}
	if len(configBody.Guide.Presets) == 0 {
		t.Fatalf("len(configBody.Guide.Presets) = 0, want presets")
	}
	if configBody.Notes["one_million_context"] == nil {
		t.Fatalf("notes.one_million_context missing")
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
