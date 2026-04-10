package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	"github.com/tidwall/gjson"
)

func TestBuildCodexWebsocketRequestBodyPreservesPreviousResponseID(t *testing.T) {
	body := []byte(`{"model":"gpt-5-codex","previous_response_id":"resp-1","input":[{"type":"message","id":"msg-1"}]}`)

	wsReqBody := buildCodexWebsocketRequestBody(body)

	if got := gjson.GetBytes(wsReqBody, "type").String(); got != "response.create" {
		t.Fatalf("type = %s, want response.create", got)
	}
	if got := gjson.GetBytes(wsReqBody, "previous_response_id").String(); got != "resp-1" {
		t.Fatalf("previous_response_id = %s, want resp-1", got)
	}
	if gjson.GetBytes(wsReqBody, "input.0.id").String() != "msg-1" {
		t.Fatalf("input item id mismatch")
	}
	if got := gjson.GetBytes(wsReqBody, "type").String(); got == "response.append" {
		t.Fatalf("unexpected websocket request type: %s", got)
	}
}

func TestApplyCodexWebsocketHeadersDefaultsToCurrentResponsesBeta(t *testing.T) {
	headers := applyCodexWebsocketHeaders(context.Background(), http.Header{}, nil, "", nil)

	if got := headers.Get("OpenAI-Beta"); got != codexResponsesWebsocketBetaHeaderValue {
		t.Fatalf("OpenAI-Beta = %s, want %s", got, codexResponsesWebsocketBetaHeaderValue)
	}
	if got := headers.Get("User-Agent"); got != "" {
		t.Fatalf("User-Agent = %s, want empty", got)
	}
	if got := headers.Get("Version"); got != "" {
		t.Fatalf("Version = %q, want empty", got)
	}
	if got := headers.Get("x-codex-beta-features"); got != "" {
		t.Fatalf("x-codex-beta-features = %q, want empty", got)
	}
	if got := headers.Get("X-Codex-Turn-Metadata"); got != "" {
		t.Fatalf("X-Codex-Turn-Metadata = %q, want empty", got)
	}
	if got := headers.Get("X-Client-Request-Id"); got != "" {
		t.Fatalf("X-Client-Request-Id = %q, want empty", got)
	}
}

func TestApplyCodexWebsocketHeadersPassesThroughClientIdentityHeaders(t *testing.T) {
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	ctx := contextWithGinHeaders(map[string]string{
		"Originator":            "Codex Desktop",
		"Version":               "0.115.0-alpha.27",
		"X-Codex-Turn-Metadata": `{"turn_id":"turn-1"}`,
		"X-Client-Request-Id":   "019d2233-e240-7162-992d-38df0a2a0e0d",
	})

	headers := applyCodexWebsocketHeaders(ctx, http.Header{}, auth, "", nil)

	if got := headers.Get("Originator"); got != "Codex Desktop" {
		t.Fatalf("Originator = %s, want %s", got, "Codex Desktop")
	}
	if got := headers.Get("Version"); got != "0.115.0-alpha.27" {
		t.Fatalf("Version = %s, want %s", got, "0.115.0-alpha.27")
	}
	if got := headers.Get("X-Codex-Turn-Metadata"); got != `{"turn_id":"turn-1"}` {
		t.Fatalf("X-Codex-Turn-Metadata = %s, want %s", got, `{"turn_id":"turn-1"}`)
	}
	if got := headers.Get("X-Client-Request-Id"); got != "019d2233-e240-7162-992d-38df0a2a0e0d" {
		t.Fatalf("X-Client-Request-Id = %s, want %s", got, "019d2233-e240-7162-992d-38df0a2a0e0d")
	}
}

func TestApplyCodexWebsocketHeadersUsesConfigDefaultsForOAuth(t *testing.T) {
	cfg := &config.Config{
		CodexHeaderDefaults: config.CodexHeaderDefaults{
			UserAgent:    "my-codex-client/1.0",
			BetaFeatures: "feature-a,feature-b",
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}

	headers := applyCodexWebsocketHeaders(context.Background(), http.Header{}, auth, "", cfg)

	if got := headers.Get("User-Agent"); got != "" {
		t.Fatalf("User-Agent = %s, want empty", got)
	}
	if got := headers.Get("x-codex-beta-features"); got != "feature-a,feature-b" {
		t.Fatalf("x-codex-beta-features = %s, want %s", got, "feature-a,feature-b")
	}
	if got := headers.Get("OpenAI-Beta"); got != codexResponsesWebsocketBetaHeaderValue {
		t.Fatalf("OpenAI-Beta = %s, want %s", got, codexResponsesWebsocketBetaHeaderValue)
	}
}

func TestApplyCodexWebsocketHeadersPrefersExistingHeadersOverClientAndConfig(t *testing.T) {
	cfg := &config.Config{
		CodexHeaderDefaults: config.CodexHeaderDefaults{
			UserAgent:    "config-ua",
			BetaFeatures: "config-beta",
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	ctx := contextWithGinHeaders(map[string]string{
		"User-Agent":            "client-ua",
		"X-Codex-Beta-Features": "client-beta",
	})
	headers := http.Header{}
	headers.Set("User-Agent", "existing-ua")
	headers.Set("X-Codex-Beta-Features", "existing-beta")

	got := applyCodexWebsocketHeaders(ctx, headers, auth, "", cfg)

	if gotVal := got.Get("User-Agent"); gotVal != "" {
		t.Fatalf("User-Agent = %s, want empty", gotVal)
	}
	if gotVal := got.Get("x-codex-beta-features"); gotVal != "existing-beta" {
		t.Fatalf("x-codex-beta-features = %s, want %s", gotVal, "existing-beta")
	}
}

func TestApplyCodexWebsocketHeadersConfigUserAgentOverridesClientHeader(t *testing.T) {
	cfg := &config.Config{
		CodexHeaderDefaults: config.CodexHeaderDefaults{
			UserAgent:    "config-ua",
			BetaFeatures: "config-beta",
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	ctx := contextWithGinHeaders(map[string]string{
		"User-Agent":            "client-ua",
		"X-Codex-Beta-Features": "client-beta",
	})

	headers := applyCodexWebsocketHeaders(ctx, http.Header{}, auth, "", cfg)

	if got := headers.Get("User-Agent"); got != "" {
		t.Fatalf("User-Agent = %s, want empty", got)
	}
	if got := headers.Get("x-codex-beta-features"); got != "client-beta" {
		t.Fatalf("x-codex-beta-features = %s, want %s", got, "client-beta")
	}
}

func TestApplyCodexWebsocketHeadersIgnoresConfigForAPIKeyAuth(t *testing.T) {
	cfg := &config.Config{
		CodexHeaderDefaults: config.CodexHeaderDefaults{
			UserAgent:    "config-ua",
			BetaFeatures: "config-beta",
		},
	}
	auth := &cliproxyauth.Auth{
		Provider:   "codex",
		Attributes: map[string]string{"api_key": "sk-test"},
	}

	headers := applyCodexWebsocketHeaders(context.Background(), http.Header{}, auth, "sk-test", cfg)

	if got := headers.Get("User-Agent"); got != "" {
		t.Fatalf("User-Agent = %s, want empty", got)
	}
	if got := headers.Get("x-codex-beta-features"); got != "" {
		t.Fatalf("x-codex-beta-features = %q, want empty", got)
	}
}

func TestApplyCodexHeadersUsesConfigUserAgentForOAuth(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	cfg := &config.Config{
		CodexHeaderDefaults: config.CodexHeaderDefaults{
			UserAgent:    "config-ua",
			BetaFeatures: "config-beta",
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	req = req.WithContext(contextWithGinHeaders(map[string]string{
		"User-Agent": "client-ua",
	}))

	applyCodexHeaders(req, auth, "oauth-token", true, cfg)

	if got := req.Header.Get("User-Agent"); got != "config-ua" {
		t.Fatalf("User-Agent = %s, want %s", got, "config-ua")
	}
	if got := req.Header.Get("x-codex-beta-features"); got != "" {
		t.Fatalf("x-codex-beta-features = %q, want empty", got)
	}
}

func TestApplyCodexHeadersPassesThroughClientIdentityHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	req = req.WithContext(contextWithGinHeaders(map[string]string{
		"Originator":            "Codex Desktop",
		"Version":               "0.115.0-alpha.27",
		"X-Codex-Turn-Metadata": `{"turn_id":"turn-1"}`,
		"X-Client-Request-Id":   "019d2233-e240-7162-992d-38df0a2a0e0d",
	}))

	applyCodexHeaders(req, auth, "oauth-token", true, nil)

	if got := req.Header.Get("Originator"); got != "Codex Desktop" {
		t.Fatalf("Originator = %s, want %s", got, "Codex Desktop")
	}
	if got := req.Header.Get("Version"); got != "0.115.0-alpha.27" {
		t.Fatalf("Version = %s, want %s", got, "0.115.0-alpha.27")
	}
	if got := req.Header.Get("X-Codex-Turn-Metadata"); got != `{"turn_id":"turn-1"}` {
		t.Fatalf("X-Codex-Turn-Metadata = %s, want %s", got, `{"turn_id":"turn-1"}`)
	}
	if got := req.Header.Get("X-Client-Request-Id"); got != "019d2233-e240-7162-992d-38df0a2a0e0d" {
		t.Fatalf("X-Client-Request-Id = %s, want %s", got, "019d2233-e240-7162-992d-38df0a2a0e0d")
	}
}

func TestApplyCodexHeadersDoesNotInjectClientOnlyHeadersByDefault(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	applyCodexHeaders(req, nil, "oauth-token", true, nil)

	if got := req.Header.Get("Version"); got != "" {
		t.Fatalf("Version = %q, want empty", got)
	}
	if got := req.Header.Get("X-Codex-Turn-Metadata"); got != "" {
		t.Fatalf("X-Codex-Turn-Metadata = %q, want empty", got)
	}
	if got := req.Header.Get("X-Client-Request-Id"); got != "" {
		t.Fatalf("X-Client-Request-Id = %q, want empty", got)
	}
}

func TestApplyCodexHeadersMimicSafeInjectsCodexIdentityDefaults(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	cfg := &config.Config{
		SDKConfig: config.SDKConfig{
			CodexMimic: config.CodexMimicConfig{Mode: config.CodexMimicModeSafe},
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}

	applyCodexHeaders(req, auth, "oauth-token", true, cfg)

	if got := req.Header.Get("Version"); got != codexVersion {
		t.Fatalf("Version = %q, want %q", got, codexVersion)
	}
	if got := req.Header.Get("Originator"); got != codexOriginator {
		t.Fatalf("Originator = %q, want %q", got, codexOriginator)
	}
	if got := req.Header.Get("X-Client-Request-Id"); got == "" {
		t.Fatal("X-Client-Request-Id = empty, want generated value")
	}
}

func TestApplyCodexWebsocketHeadersMimicStrictOverridesClientSignature(t *testing.T) {
	cfg := &config.Config{
		SDKConfig: config.SDKConfig{
			CodexMimic: config.CodexMimicConfig{
				Mode: config.CodexMimicModeStrict,
				Strict: config.CodexMimicStrictConfig{
					ForceTurnMetadata:    true,
					ForceTurnState:       true,
					IncludeTimingMetrics: true,
				},
			},
		},
		CodexHeaderDefaults: config.CodexHeaderDefaults{
			UserAgent: "config-ua",
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	ctx := contextWithGinHeaders(map[string]string{
		"User-Agent":  "client-ua",
		"Originator":  "client-originator",
		"Version":     "client-version",
		"OpenAI-Beta": "responses_websockets=v0",
	})

	headers := applyCodexWebsocketHeaders(ctx, http.Header{}, auth, "", cfg)

	if got := headers.Get("User-Agent"); got != "config-ua" {
		t.Fatalf("User-Agent = %q, want %q", got, "config-ua")
	}
	if got := headers.Get("Originator"); got != codexOriginator {
		t.Fatalf("Originator = %q, want %q", got, codexOriginator)
	}
	if got := headers.Get("Version"); got != codexVersion {
		t.Fatalf("Version = %q, want %q", got, codexVersion)
	}
	if got := headers.Get("x-client-request-id"); got == "" {
		t.Fatal("x-client-request-id = empty, want generated value")
	}
	if got := headers.Get("x-codex-turn-metadata"); got != "{}" {
		t.Fatalf("x-codex-turn-metadata = %q, want %q", got, "{}")
	}
	if got := headers.Get("x-codex-turn-state"); got != "{}" {
		t.Fatalf("x-codex-turn-state = %q, want %q", got, "{}")
	}
	if got := headers.Get("x-responsesapi-include-timing-metrics"); got != "true" {
		t.Fatalf("x-responsesapi-include-timing-metrics = %q, want %q", got, "true")
	}
}

func TestApplyCodexHeadersMimicStrictCanForceTurnMetadata(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	cfg := &config.Config{
		SDKConfig: config.SDKConfig{
			CodexMimic: config.CodexMimicConfig{
				Mode: config.CodexMimicModeStrict,
				Strict: config.CodexMimicStrictConfig{
					ForceTurnMetadata: true,
				},
			},
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}

	applyCodexHeaders(req, auth, "oauth-token", true, cfg)

	if got := req.Header.Get("X-Codex-Turn-Metadata"); got != "{}" {
		t.Fatalf("X-Codex-Turn-Metadata = %q, want %q", got, "{}")
	}
}

func TestCodexStableDerivedIDUsesContinuitySeed(t *testing.T) {
	headersA := make(http.Header)
	headersA.Set("session_id", "continuity-a")
	headersB := make(http.Header)
	headersB.Set("session_id", "continuity-b")

	got1 := codexStableDerivedID("request-id", headersA, nil)
	got2 := codexStableDerivedID("request-id", headersA, nil)
	got3 := codexStableDerivedID("request-id", headersB, nil)

	want := uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:request-id:continuity-a")).String()
	if got1 != want {
		t.Fatalf("codexStableDerivedID() = %q, want %q", got1, want)
	}
	if got2 != want {
		t.Fatalf("codexStableDerivedID() second call = %q, want %q", got2, want)
	}
	if got3 == got1 {
		t.Fatalf("different continuity seeds should produce different IDs, both got %q", got3)
	}
}

func TestApplyCodexHeadersMimicStrictCanStabilizeRequestAndTurnMetadata(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	applyCodexContinuityHeaders(req.Header, codexContinuity{Key: "continuity-http"})

	cfg := &config.Config{
		SDKConfig: config.SDKConfig{
			CodexMimic: config.CodexMimicConfig{
				Mode: config.CodexMimicModeStrict,
				Strict: config.CodexMimicStrictConfig{
					StableRequestID: true,
					StableTurnID:    true,
				},
			},
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}

	applyCodexHeaders(req, auth, "oauth-token", true, cfg)

	wantRequestID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:request-id:continuity-http")).String()
	wantTurnID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:turn-id:continuity-http")).String()
	if got := req.Header.Get("X-Client-Request-Id"); got != wantRequestID {
		t.Fatalf("X-Client-Request-Id = %q, want %q", got, wantRequestID)
	}
	if got := req.Header.Get("X-Codex-Turn-Metadata"); got != `{"turn_id":"`+wantTurnID+`"}` {
		t.Fatalf("X-Codex-Turn-Metadata = %q, want %q", got, `{"turn_id":"`+wantTurnID+`"}`)
	}
}

func TestApplyCodexWebsocketHeadersMimicStrictCanStabilizeRequestAndTurnMetadata(t *testing.T) {
	cfg := &config.Config{
		SDKConfig: config.SDKConfig{
			CodexMimic: config.CodexMimicConfig{
				Mode: config.CodexMimicModeStrict,
				Strict: config.CodexMimicStrictConfig{
					StableRequestID: true,
					StableTurnID:    true,
				},
			},
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	headers := http.Header{}
	applyCodexContinuityHeaders(headers, codexContinuity{Key: "continuity-ws"})

	got := applyCodexWebsocketHeaders(context.Background(), headers, auth, "", cfg)

	wantRequestID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:request-id:continuity-ws")).String()
	wantTurnID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:turn-id:continuity-ws")).String()
	if gotValue := got.Get("x-client-request-id"); gotValue != wantRequestID {
		t.Fatalf("x-client-request-id = %q, want %q", gotValue, wantRequestID)
	}
	if gotValue := got.Get("x-codex-turn-metadata"); gotValue != `{"turn_id":"`+wantTurnID+`"}` {
		t.Fatalf("x-codex-turn-metadata = %q, want %q", gotValue, `{"turn_id":"`+wantTurnID+`"}`)
	}
}

func TestApplyCodexWebsocketHeadersMimicStrictCanStabilizeTurnState(t *testing.T) {
	cfg := &config.Config{
		SDKConfig: config.SDKConfig{
			CodexMimic: config.CodexMimicConfig{
				Mode: config.CodexMimicModeStrict,
				Strict: config.CodexMimicStrictConfig{
					ForceTurnState:  true,
					StableRequestID: true,
					StableTurnID:    true,
				},
			},
		},
	}
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{"email": "user@example.com"},
	}
	headers := http.Header{}
	applyCodexContinuityHeaders(headers, codexContinuity{Key: "continuity-state"})

	got := applyCodexWebsocketHeaders(context.Background(), headers, auth, "", cfg)

	wantRequestID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:request-id:continuity-state")).String()
	wantTurnID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:turn-id:continuity-state")).String()
	state := got.Get("x-codex-turn-state")
	if gotValue := gjson.Get(state, "turn_id").String(); gotValue != wantTurnID {
		t.Fatalf("x-codex-turn-state.turn_id = %q, want %q", gotValue, wantTurnID)
	}
	if gotValue := gjson.Get(state, "request_id").String(); gotValue != wantRequestID {
		t.Fatalf("x-codex-turn-state.request_id = %q, want %q", gotValue, wantRequestID)
	}
}

func contextWithGinHeaders(headers map[string]string) context.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ginCtx.Request.Header = make(http.Header, len(headers))
	for key, value := range headers {
		ginCtx.Request.Header.Set(key, value)
	}
	return context.WithValue(context.Background(), "gin", ginCtx)
}

func TestNewProxyAwareWebsocketDialerDirectDisablesProxy(t *testing.T) {
	t.Parallel()

	dialer := newProxyAwareWebsocketDialer(
		&config.Config{SDKConfig: sdkconfig.SDKConfig{ProxyURL: "http://global-proxy.example.com:8080"}},
		&cliproxyauth.Auth{ProxyURL: "direct"},
	)

	if dialer.Proxy != nil {
		t.Fatal("expected websocket proxy function to be nil for direct mode")
	}
}

func TestNewProxyAwareWebsocketDialerIgnoresFileAuthProxyWhenConfigured(t *testing.T) {
	t.Parallel()

	dialer := newProxyAwareWebsocketDialer(
		&config.Config{SDKConfig: sdkconfig.SDKConfig{
			ProxyURL:               "http://global-proxy.example.com:8080",
			IgnoreAuthFileProxyURL: true,
		}},
		&cliproxyauth.Auth{
			FileName: "auths/codex.json",
			ProxyURL: "http://file-proxy.example.com:8080",
		},
	)

	if dialer.Proxy == nil {
		t.Fatal("expected websocket proxy function to use global proxy")
	}

	req, errRequest := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errRequest != nil {
		t.Fatalf("http.NewRequest returned error: %v", errRequest)
	}

	proxyURL, errProxy := dialer.Proxy(req)
	if errProxy != nil {
		t.Fatalf("dialer.Proxy returned error: %v", errProxy)
	}
	if proxyURL == nil || proxyURL.String() != "http://global-proxy.example.com:8080" {
		t.Fatalf("proxy URL = %v, want http://global-proxy.example.com:8080", proxyURL)
	}
}
