// Package executor provides runtime execution capabilities for various AI service providers.
// It includes stateless executors that handle API requests, streaming responses,
// token counting, and authentication refresh for different AI service providers.
package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// glEndpoint is the base URL for the Google Generative Language API.
	glEndpoint = "https://generativelanguage.googleapis.com"

	// glAPIVersion is the API version used for Gemini requests.
	glAPIVersion = "v1beta"
)

// GeminiExecutor is a stateless executor for the official Gemini API using API keys.
// It handles both API key and OAuth bearer token authentication, supporting both
// regular and streaming requests to the Google Generative Language API.
type GeminiExecutor struct {
	// cfg holds the application configuration.
	cfg *config.Config
}

// NewGeminiExecutor creates a new Gemini executor instance.
//
// Parameters:
//   - cfg: The application configuration
//
// Returns:
//   - *GeminiExecutor: A new Gemini executor instance
func NewGeminiExecutor(cfg *config.Config) *GeminiExecutor { return &GeminiExecutor{cfg: cfg} }

// Identifier returns the executor identifier for Gemini.
func (e *GeminiExecutor) Identifier() string { return "gemini" }

// PrepareRequest prepares the HTTP request for execution (no-op for Gemini).
func (e *GeminiExecutor) PrepareRequest(_ *http.Request, _ *cliproxyauth.Auth) error { return nil }

// Execute performs a non-streaming request to the Gemini API.
// It translates the request to Gemini format, sends it to the API, and translates
// the response back to the requested format.
//
// Parameters:
//   - ctx: The context for the request
//   - auth: The authentication information
//   - req: The request to execute
//   - opts: Additional execution options
//
// Returns:
//   - cliproxyexecutor.Response: The response from the API
//   - error: An error if the request fails
func (e *GeminiExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	apiKey, bearer := geminiCreds(auth)

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)

	// Official Gemini API via API key or OAuth bearer
	from := opts.SourceFormat
	to := sdktranslator.FromString("gemini")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), false)
	body = disableGeminiThinkingConfig(body, req.Model)

	action := "generateContent"
	if req.Metadata != nil {
		if a, _ := req.Metadata["action"].(string); a == "countTokens" {
			action = "countTokens"
		}
	}
	url := fmt.Sprintf("%s/%s/models/%s:%s", glEndpoint, glAPIVersion, req.Model, action)
	if opts.Alt != "" && action != "countTokens" {
		url = url + fmt.Sprintf("?$alt=%s", opts.Alt)
	}

	body, _ = sjson.DeleteBytes(body, "session_id")

	recordAPIRequest(ctx, e.cfg, body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", apiKey)
	} else if bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+bearer)
	}

	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		appendAPIResponseChunk(ctx, e.cfg, b)
		log.Debugf("request error, error status: %d, error body: %s", resp.StatusCode, string(b))
		return cliproxyexecutor.Response{}, statusErr{code: resp.StatusCode, msg: string(b)}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	appendAPIResponseChunk(ctx, e.cfg, data)
	reporter.publish(ctx, parseGeminiUsage(data))
	var param any
	out := sdktranslator.TranslateNonStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), body, data, &param)
	return cliproxyexecutor.Response{Payload: []byte(out)}, nil
}

func (e *GeminiExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (<-chan cliproxyexecutor.StreamChunk, error) {
	apiKey, bearer := geminiCreds(auth)

	reporter := newUsageReporter(ctx, e.Identifier(), req.Model, auth)

	from := opts.SourceFormat
	to := sdktranslator.FromString("gemini")
	body := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), true)
	body = disableGeminiThinkingConfig(body, req.Model)

	url := fmt.Sprintf("%s/%s/models/%s:%s", glEndpoint, glAPIVersion, req.Model, "streamGenerateContent")
	if opts.Alt == "" {
		url = url + "?alt=sse"
	} else {
		url = url + fmt.Sprintf("?$alt=%s", opts.Alt)
	}

	body, _ = sjson.DeleteBytes(body, "session_id")

	recordAPIRequest(ctx, e.cfg, body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", apiKey)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+bearer)
	}

	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		appendAPIResponseChunk(ctx, e.cfg, b)
		log.Debugf("request error, error status: %d, error body: %s", resp.StatusCode, string(b))
		return nil, statusErr{code: resp.StatusCode, msg: string(b)}
	}
	out := make(chan cliproxyexecutor.StreamChunk)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 20_971_520)
		scanner.Buffer(buf, 20_971_520)
		var param any
		for scanner.Scan() {
			line := scanner.Bytes()
			appendAPIResponseChunk(ctx, e.cfg, line)
			if detail, ok := parseGeminiStreamUsage(line); ok {
				reporter.publish(ctx, detail)
			}
			lines := sdktranslator.TranslateStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), body, bytes.Clone(line), &param)
			for i := range lines {
				out <- cliproxyexecutor.StreamChunk{Payload: []byte(lines[i])}
			}
		}
		lines := sdktranslator.TranslateStream(ctx, to, from, req.Model, bytes.Clone(opts.OriginalRequest), body, bytes.Clone([]byte("[DONE]")), &param)
		for i := range lines {
			out <- cliproxyexecutor.StreamChunk{Payload: []byte(lines[i])}
		}
		if err = scanner.Err(); err != nil {
			out <- cliproxyexecutor.StreamChunk{Err: err}
		}
	}()
	return out, nil
}

func (e *GeminiExecutor) CountTokens(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	apiKey, bearer := geminiCreds(auth)

	from := opts.SourceFormat
	to := sdktranslator.FromString("gemini")
	translatedReq := sdktranslator.TranslateRequest(from, to, req.Model, bytes.Clone(req.Payload), false)
	translatedReq = disableGeminiThinkingConfig(translatedReq, req.Model)
	respCtx := context.WithValue(ctx, "alt", opts.Alt)
	translatedReq, _ = sjson.DeleteBytes(translatedReq, "tools")
	translatedReq, _ = sjson.DeleteBytes(translatedReq, "generationConfig")

	url := fmt.Sprintf("%s/%s/models/%s:%s", glEndpoint, glAPIVersion, req.Model, "countTokens")
	recordAPIRequest(ctx, e.cfg, translatedReq)

	requestBody := bytes.NewReader(translatedReq)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, requestBody)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", apiKey)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+bearer)
	}

	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return cliproxyexecutor.Response{}, err
	}
	appendAPIResponseChunk(ctx, e.cfg, data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Debugf("request error, error status: %d, error body: %s", resp.StatusCode, string(data))
		return cliproxyexecutor.Response{}, statusErr{code: resp.StatusCode, msg: string(data)}
	}

	count := gjson.GetBytes(data, "totalTokens").Int()
	translated := sdktranslator.TranslateTokenCount(respCtx, to, from, count, data)
	return cliproxyexecutor.Response{Payload: []byte(translated)}, nil
}

func (e *GeminiExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	log.Debugf("gemini executor: refresh called")
	// OAuth bearer token refresh for official Gemini API.
	if auth == nil {
		return nil, fmt.Errorf("gemini executor: auth is nil")
	}
	if auth.Metadata == nil {
		return auth, nil
	}
	// Token data is typically nested under "token" map in Gemini files.
	tokenMap, _ := auth.Metadata["token"].(map[string]any)
	var refreshToken, accessToken, clientID, clientSecret, tokenURI, expiryStr string
	if tokenMap != nil {
		if v, ok := tokenMap["refresh_token"].(string); ok {
			refreshToken = v
		}
		if v, ok := tokenMap["access_token"].(string); ok {
			accessToken = v
		}
		if v, ok := tokenMap["client_id"].(string); ok {
			clientID = v
		}
		if v, ok := tokenMap["client_secret"].(string); ok {
			clientSecret = v
		}
		if v, ok := tokenMap["token_uri"].(string); ok {
			tokenURI = v
		}
		if v, ok := tokenMap["expiry"].(string); ok {
			expiryStr = v
		}
	} else {
		// Fallback to top-level keys if present
		if v, ok := auth.Metadata["refresh_token"].(string); ok {
			refreshToken = v
		}
		if v, ok := auth.Metadata["access_token"].(string); ok {
			accessToken = v
		}
		if v, ok := auth.Metadata["client_id"].(string); ok {
			clientID = v
		}
		if v, ok := auth.Metadata["client_secret"].(string); ok {
			clientSecret = v
		}
		if v, ok := auth.Metadata["token_uri"].(string); ok {
			tokenURI = v
		}
		if v, ok := auth.Metadata["expiry"].(string); ok {
			expiryStr = v
		}
	}
	if refreshToken == "" {
		// Nothing to do for API key or cookie based entries
		return auth, nil
	}

	// Prepare oauth2 config; default to Google endpoints
	endpoint := google.Endpoint
	if tokenURI != "" {
		endpoint.TokenURL = tokenURI
	}
	conf := &oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, Endpoint: endpoint}

	// Ensure proxy-aware HTTP client for token refresh
	httpClient := util.SetProxy(&e.cfg.SDKConfig, &http.Client{})
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	// Build base token
	tok := &oauth2.Token{AccessToken: accessToken, RefreshToken: refreshToken}
	if t, err := time.Parse(time.RFC3339, expiryStr); err == nil {
		tok.Expiry = t
	}
	newTok, err := conf.TokenSource(ctx, tok).Token()
	if err != nil {
		return nil, err
	}

	// Persist back to metadata; prefer nested token map if present
	if tokenMap == nil {
		tokenMap = make(map[string]any)
	}
	tokenMap["access_token"] = newTok.AccessToken
	tokenMap["refresh_token"] = newTok.RefreshToken
	tokenMap["expiry"] = newTok.Expiry.Format(time.RFC3339)
	if clientID != "" {
		tokenMap["client_id"] = clientID
	}
	if clientSecret != "" {
		tokenMap["client_secret"] = clientSecret
	}
	if tokenURI != "" {
		tokenMap["token_uri"] = tokenURI
	}
	auth.Metadata["token"] = tokenMap

	// Also mirror top-level access_token for compatibility if previously present
	if _, ok := auth.Metadata["access_token"]; ok {
		auth.Metadata["access_token"] = newTok.AccessToken
	}
	return auth, nil
}

func geminiCreds(a *cliproxyauth.Auth) (apiKey, bearer string) {
	if a == nil {
		return "", ""
	}
	if a.Attributes != nil {
		if v := a.Attributes["api_key"]; v != "" {
			apiKey = v
		}
	}
	if a.Metadata != nil {
		// GeminiTokenStorage.Token is a map that may contain access_token
		if v, ok := a.Metadata["access_token"].(string); ok && v != "" {
			bearer = v
		}
		if token, ok := a.Metadata["token"].(map[string]any); ok && token != nil {
			if v, ok2 := token["access_token"].(string); ok2 && v != "" {
				bearer = v
			}
		}
	}
	return
}
