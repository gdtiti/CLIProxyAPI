package executor

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type usageReporter struct {
	provider    string
	model       string
	authID      string
	authIndex   string
	apiKey      string
	source      string
	requestedAt time.Time
	once        sync.Once
}

func newUsageReporter(ctx context.Context, provider, model string, auth *cliproxyauth.Auth) *usageReporter {
	apiKey := apiKeyFromContext(ctx)
	reporter := &usageReporter{
		provider:    provider,
		model:       model,
		requestedAt: time.Now(),
		apiKey:      apiKey,
		source:      resolveUsageSource(auth, apiKey),
	}
	if auth != nil {
		reporter.authID = auth.ID
		reporter.authIndex = auth.EnsureIndex()
	}
	return reporter
}

func (r *usageReporter) publish(ctx context.Context, detail usage.Detail) {
	r.publishWithOutcome(ctx, detail, false)
}

func (r *usageReporter) publishFailure(ctx context.Context) {
	r.publishWithOutcome(ctx, usage.Detail{}, true)
}

func (r *usageReporter) trackFailure(ctx context.Context, errPtr *error) {
	if r == nil || errPtr == nil {
		return
	}
	if *errPtr != nil {
		r.publishFailure(ctx)
	}
}

func (r *usageReporter) publishWithOutcome(ctx context.Context, detail usage.Detail, failed bool) {
	if r == nil {
		return
	}
	if detail.TotalTokens == 0 {
		total := detail.InputTokens + detail.OutputTokens + detail.ReasoningTokens
		if total > 0 {
			detail.TotalTokens = total
		}
	}
	if detail.InputTokens == 0 && detail.OutputTokens == 0 && detail.ReasoningTokens == 0 && detail.CachedTokens == 0 && detail.TotalTokens == 0 && !failed {
		return
	}
	r.once.Do(func() {
		usage.PublishRecord(ctx, r.buildRecord(detail, failed))
	})
}

// ensurePublished guarantees that a usage record is emitted exactly once.
// It is safe to call multiple times; only the first call wins due to once.Do.
// This is used to ensure request counting even when upstream responses do not
// include any usage fields (tokens), especially for streaming paths.
func (r *usageReporter) ensurePublished(ctx context.Context) {
	if r == nil {
		return
	}
	r.once.Do(func() {
		usage.PublishRecord(ctx, r.buildRecord(usage.Detail{}, false))
	})
}

func (r *usageReporter) buildRecord(detail usage.Detail, failed bool) usage.Record {
	if r == nil {
		return usage.Record{Detail: detail, Failed: failed}
	}
	return usage.Record{
		Provider:    r.provider,
		Model:       r.model,
		Source:      r.source,
		APIKey:      r.apiKey,
		AuthID:      r.authID,
		AuthIndex:   r.authIndex,
		RequestedAt: r.requestedAt,
		Latency:     r.latency(),
		Failed:      failed,
		Detail:      detail,
	}
}

func (r *usageReporter) latency() time.Duration {
	if r == nil || r.requestedAt.IsZero() {
		return 0
	}
	latency := time.Since(r.requestedAt)
	if latency < 0 {
		return 0
	}
	return latency
}

func apiKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	ginCtx, ok := ctx.Value("gin").(*gin.Context)
	if !ok || ginCtx == nil {
		return ""
	}
	if v, exists := ginCtx.Get("apiKey"); exists {
		switch value := v.(type) {
		case string:
			return value
		case fmt.Stringer:
			return value.String()
		default:
			return fmt.Sprintf("%v", value)
		}
	}
	return ""
}

func resolveUsageSource(auth *cliproxyauth.Auth, ctxAPIKey string) string {
	if auth != nil {
		provider := strings.TrimSpace(auth.Provider)
		if strings.EqualFold(provider, "gemini-cli") {
			if id := strings.TrimSpace(auth.ID); id != "" {
				return id
			}
		}
		if strings.EqualFold(provider, "vertex") {
			if auth.Metadata != nil {
				if projectID, ok := auth.Metadata["project_id"].(string); ok {
					if trimmed := strings.TrimSpace(projectID); trimmed != "" {
						return trimmed
					}
				}
				if project, ok := auth.Metadata["project"].(string); ok {
					if trimmed := strings.TrimSpace(project); trimmed != "" {
						return trimmed
					}
				}
			}
		}
		if _, value := auth.AccountInfo(); value != "" {
			return strings.TrimSpace(value)
		}
		if auth.Metadata != nil {
			if email, ok := auth.Metadata["email"].(string); ok {
				if trimmed := strings.TrimSpace(email); trimmed != "" {
					return trimmed
				}
			}
		}
		if auth.Attributes != nil {
			if key := strings.TrimSpace(auth.Attributes["api_key"]); key != "" {
				return key
			}
		}
	}
	if trimmed := strings.TrimSpace(ctxAPIKey); trimmed != "" {
		return trimmed
	}
	return ""
}

func parseCodexUsage(data []byte) (usage.Detail, bool) {
	usageNode := gjson.ParseBytes(data).Get("response.usage")
	if !usageNode.Exists() {
		return usage.Detail{}, false
	}
	detail := usage.Detail{
		InputTokens:  usageNode.Get("input_tokens").Int(),
		OutputTokens: usageNode.Get("output_tokens").Int(),
		TotalTokens:  usageNode.Get("total_tokens").Int(),
	}
	if cached := usageNode.Get("input_tokens_details.cached_tokens"); cached.Exists() {
		detail.CachedTokens = cached.Int()
	}
	if reasoning := usageNode.Get("output_tokens_details.reasoning_tokens"); reasoning.Exists() {
		detail.ReasoningTokens = reasoning.Int()
	}
	return detail, true
}

func parseCodexUsageLimitRetryAfter(body []byte, now time.Time) *time.Duration {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}
	root := gjson.ParseBytes(body)
	if d := parseUsageLimitRetryAfterFromNode(root, now); d != nil {
		return d
	}
	for _, key := range [...]string{"status_message", "message", "error.message"} {
		nestedRaw := strings.TrimSpace(root.Get(key).String())
		if nestedRaw == "" || !gjson.Valid(nestedRaw) {
			continue
		}
		nested := gjson.Parse(nestedRaw)
		if d := parseUsageLimitRetryAfterFromNode(nested, now); d != nil {
			return d
		}
	}
	return nil
}

func isCodexUsageLimitReached(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	root := gjson.ParseBytes(body)
	if isUsageLimitNode(root) {
		return true
	}
	for _, key := range [...]string{"status_message", "message", "error.message"} {
		nestedRaw := strings.TrimSpace(root.Get(key).String())
		if nestedRaw == "" || !gjson.Valid(nestedRaw) {
			continue
		}
		if isUsageLimitNode(gjson.Parse(nestedRaw)) {
			return true
		}
	}
	return false
}

func parseCodexQuotaRetryAfter(body []byte, now time.Time) *time.Duration {
	decision := parseCodexQuotaRetryDecision(body, now)
	return decision.RetryAfter
}

type codexQuotaRetryDecision struct {
	RetryAfter *time.Duration
	Window     string
}

type codexQuotaRetryCandidate struct {
	Duration      time.Duration
	Window        string
	WindowSeconds int64
}

func parseCodexQuotaRetryDecision(body []byte, now time.Time) codexQuotaRetryDecision {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return codexQuotaRetryDecision{}
	}
	root := gjson.ParseBytes(body)
	candidates := collectCodexQuotaRetryCandidates(root.Get("rate_limit"), now)
	candidates = append(candidates, collectCodexQuotaRetryCandidates(root.Get("rateLimit"), now)...)
	candidates = append(candidates, collectCodexQuotaRetryCandidates(root.Get("code_review_rate_limit"), now)...)
	candidates = append(candidates, collectCodexQuotaRetryCandidates(root.Get("codeReviewRateLimit"), now)...)
	if len(candidates) == 0 {
		return codexQuotaRetryDecision{}
	}
	best := candidates[0]
	for i := 1; i < len(candidates); i++ {
		if candidates[i].Duration > best.Duration {
			best = candidates[i]
			continue
		}
		if candidates[i].Duration == best.Duration && candidates[i].WindowSeconds > best.WindowSeconds {
			best = candidates[i]
		}
	}
	if best.Duration < 0 {
		best.Duration = 0
	}
	retryAfter := best.Duration
	return codexQuotaRetryDecision{RetryAfter: &retryAfter, Window: best.Window}
}

func collectCodexQuotaRetryCandidates(limitNode gjson.Result, now time.Time) []codexQuotaRetryCandidate {
	if !limitNode.Exists() {
		return nil
	}
	limited := false
	if allowed := limitNode.Get("allowed"); allowed.Exists() && !allowed.Bool() {
		limited = true
	}
	if reached := limitNode.Get("limit_reached"); reached.Exists() && reached.Bool() {
		limited = true
	}
	if reached := limitNode.Get("limitReached"); reached.Exists() && reached.Bool() {
		limited = true
	}
	if !limited {
		return nil
	}

	windows := []gjson.Result{
		limitNode.Get("primary_window"),
		limitNode.Get("primaryWindow"),
		limitNode.Get("secondary_window"),
		limitNode.Get("secondaryWindow"),
	}
	candidates := make([]codexQuotaRetryCandidate, 0, len(windows))
	for i := 0; i < len(windows); i++ {
		window := windows[i]
		if !window.Exists() {
			continue
		}
		usedPercent := codexWindowUsedPercent(window)
		if usedPercent < 99.5 {
			continue
		}
		if candidate, ok := codexWindowRetryCandidate(window, now); ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func codexWindowUsedPercent(window gjson.Result) float64 {
	for _, path := range [...]string{"used_percent", "usedPercent"} {
		value := window.Get(path)
		if !value.Exists() {
			continue
		}
		if value.Type == gjson.String {
			parsed := strings.TrimSpace(value.String())
			if parsed == "" {
				continue
			}
			if parsedValue := gjson.Parse(parsed); parsedValue.Exists() {
				return parsedValue.Float()
			}
		}
		return value.Float()
	}
	return 0
}

func codexWindowRetryCandidate(window gjson.Result, now time.Time) (codexQuotaRetryCandidate, bool) {
	windowSeconds := int64(0)
	for _, path := range [...]string{"limit_window_seconds", "limitWindowSeconds"} {
		node := window.Get(path)
		if !node.Exists() {
			continue
		}
		windowSeconds = node.Int()
		if windowSeconds > 0 {
			break
		}
	}
	candidate := codexQuotaRetryCandidate{WindowSeconds: windowSeconds, Window: codexWindowName(windowSeconds)}
	for _, path := range [...]string{"reset_after_seconds", "resetAfterSeconds"} {
		secondsNode := window.Get(path)
		if !secondsNode.Exists() {
			continue
		}
		seconds := secondsNode.Int()
		if seconds <= 0 {
			continue
		}
		candidate.Duration = time.Duration(seconds) * time.Second
		return candidate, true
	}
	for _, path := range [...]string{"reset_at", "resetAt"} {
		tsNode := window.Get(path)
		if !tsNode.Exists() {
			continue
		}
		raw := tsNode.Int()
		if raw <= 0 {
			continue
		}
		resetAt := time.Unix(raw, 0)
		if raw > 1_000_000_000_000 {
			resetAt = time.UnixMilli(raw)
		}
		d := resetAt.Sub(now)
		if d < 0 {
			d = 0
		}
		candidate.Duration = d
		return candidate, true
	}
	return codexQuotaRetryCandidate{}, false
}

func codexWindowName(limitWindowSeconds int64) string {
	switch limitWindowSeconds {
	case 18_000:
		return "five_hour"
	case 604_800:
		return "weekly"
	default:
		return ""
	}
}

func parseUsageLimitRetryAfterFromNode(node gjson.Result, now time.Time) *time.Duration {
	if !node.Exists() {
		return nil
	}
	if !isUsageLimitNode(node) {
		return nil
	}

	for _, path := range [...]string{"error.resets_in_seconds", "resets_in_seconds", "error.reset_after_seconds", "reset_after_seconds"} {
		secNode := node.Get(path)
		if !secNode.Exists() {
			continue
		}
		seconds := secNode.Int()
		if seconds <= 0 {
			continue
		}
		d := time.Duration(seconds) * time.Second
		return &d
	}

	for _, path := range [...]string{"error.resets_at", "resets_at", "error.reset_at", "reset_at"} {
		tsNode := node.Get(path)
		if !tsNode.Exists() {
			continue
		}
		raw := tsNode.Int()
		if raw <= 0 {
			continue
		}
		resetAt := time.Unix(raw, 0)
		if raw > 1_000_000_000_000 {
			resetAt = time.UnixMilli(raw)
		}
		d := resetAt.Sub(now)
		if d < 0 {
			d = 0
		}
		return &d
	}

	return nil
}

func isUsageLimitNode(node gjson.Result) bool {
	if !node.Exists() {
		return false
	}
	typeNode := strings.TrimSpace(node.Get("error.type").String())
	if typeNode == "" {
		typeNode = strings.TrimSpace(node.Get("type").String())
	}
	return strings.EqualFold(typeNode, "usage_limit_reached")
}

func parseOpenAIUsage(data []byte) usage.Detail {
	usageNode := gjson.ParseBytes(data).Get("usage")
	if !usageNode.Exists() {
		return usage.Detail{}
	}
	inputNode := usageNode.Get("prompt_tokens")
	if !inputNode.Exists() {
		inputNode = usageNode.Get("input_tokens")
	}
	outputNode := usageNode.Get("completion_tokens")
	if !outputNode.Exists() {
		outputNode = usageNode.Get("output_tokens")
	}
	detail := usage.Detail{
		InputTokens:  inputNode.Int(),
		OutputTokens: outputNode.Int(),
		TotalTokens:  usageNode.Get("total_tokens").Int(),
	}
	cached := usageNode.Get("prompt_tokens_details.cached_tokens")
	if !cached.Exists() {
		cached = usageNode.Get("input_tokens_details.cached_tokens")
	}
	if cached.Exists() {
		detail.CachedTokens = cached.Int()
	}
	reasoning := usageNode.Get("completion_tokens_details.reasoning_tokens")
	if !reasoning.Exists() {
		reasoning = usageNode.Get("output_tokens_details.reasoning_tokens")
	}
	if reasoning.Exists() {
		detail.ReasoningTokens = reasoning.Int()
	}
	return detail
}

func parseOpenAIStreamUsage(line []byte) (usage.Detail, bool) {
	payload := jsonPayload(line)
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return usage.Detail{}, false
	}
	usageNode := gjson.GetBytes(payload, "usage")
	if !usageNode.Exists() {
		return usage.Detail{}, false
	}
	detail := usage.Detail{
		InputTokens:  usageNode.Get("prompt_tokens").Int(),
		OutputTokens: usageNode.Get("completion_tokens").Int(),
		TotalTokens:  usageNode.Get("total_tokens").Int(),
	}
	if cached := usageNode.Get("prompt_tokens_details.cached_tokens"); cached.Exists() {
		detail.CachedTokens = cached.Int()
	}
	if reasoning := usageNode.Get("completion_tokens_details.reasoning_tokens"); reasoning.Exists() {
		detail.ReasoningTokens = reasoning.Int()
	}
	return detail, true
}

func parseOpenAIResponsesUsageDetail(usageNode gjson.Result) usage.Detail {
	detail := usage.Detail{
		InputTokens:  usageNode.Get("input_tokens").Int(),
		OutputTokens: usageNode.Get("output_tokens").Int(),
		TotalTokens:  usageNode.Get("total_tokens").Int(),
	}
	if detail.TotalTokens == 0 {
		detail.TotalTokens = detail.InputTokens + detail.OutputTokens
	}
	if cached := usageNode.Get("input_tokens_details.cached_tokens"); cached.Exists() {
		detail.CachedTokens = cached.Int()
	}
	if reasoning := usageNode.Get("output_tokens_details.reasoning_tokens"); reasoning.Exists() {
		detail.ReasoningTokens = reasoning.Int()
	}
	return detail
}

func parseOpenAIResponsesUsage(data []byte) usage.Detail {
	usageNode := gjson.ParseBytes(data).Get("usage")
	if !usageNode.Exists() {
		return usage.Detail{}
	}
	return parseOpenAIResponsesUsageDetail(usageNode)
}

func parseOpenAIResponsesStreamUsage(line []byte) (usage.Detail, bool) {
	payload := jsonPayload(line)
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return usage.Detail{}, false
	}
	usageNode := gjson.GetBytes(payload, "usage")
	if !usageNode.Exists() {
		return usage.Detail{}, false
	}
	return parseOpenAIResponsesUsageDetail(usageNode), true
}

func parseClaudeUsage(data []byte) usage.Detail {
	usageNode := gjson.ParseBytes(data).Get("usage")
	if !usageNode.Exists() {
		return usage.Detail{}
	}
	detail := usage.Detail{
		InputTokens:  usageNode.Get("input_tokens").Int(),
		OutputTokens: usageNode.Get("output_tokens").Int(),
		CachedTokens: usageNode.Get("cache_read_input_tokens").Int(),
	}
	if detail.CachedTokens == 0 {
		// fall back to creation tokens when read tokens are absent
		detail.CachedTokens = usageNode.Get("cache_creation_input_tokens").Int()
	}
	detail.TotalTokens = detail.InputTokens + detail.OutputTokens
	return detail
}

func parseClaudeStreamUsage(line []byte) (usage.Detail, bool) {
	payload := jsonPayload(line)
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return usage.Detail{}, false
	}
	usageNode := gjson.GetBytes(payload, "usage")
	if !usageNode.Exists() {
		return usage.Detail{}, false
	}
	detail := usage.Detail{
		InputTokens:  usageNode.Get("input_tokens").Int(),
		OutputTokens: usageNode.Get("output_tokens").Int(),
		CachedTokens: usageNode.Get("cache_read_input_tokens").Int(),
	}
	if detail.CachedTokens == 0 {
		detail.CachedTokens = usageNode.Get("cache_creation_input_tokens").Int()
	}
	detail.TotalTokens = detail.InputTokens + detail.OutputTokens
	return detail, true
}

func parseGeminiFamilyUsageDetail(node gjson.Result) usage.Detail {
	detail := usage.Detail{
		InputTokens:     node.Get("promptTokenCount").Int(),
		OutputTokens:    node.Get("candidatesTokenCount").Int(),
		ReasoningTokens: node.Get("thoughtsTokenCount").Int(),
		TotalTokens:     node.Get("totalTokenCount").Int(),
		CachedTokens:    node.Get("cachedContentTokenCount").Int(),
	}
	if detail.TotalTokens == 0 {
		detail.TotalTokens = detail.InputTokens + detail.OutputTokens + detail.ReasoningTokens
	}
	return detail
}

func parseGeminiCLIUsage(data []byte) usage.Detail {
	usageNode := gjson.ParseBytes(data)
	node := usageNode.Get("response.usageMetadata")
	if !node.Exists() {
		node = usageNode.Get("response.usage_metadata")
	}
	if !node.Exists() {
		return usage.Detail{}
	}
	return parseGeminiFamilyUsageDetail(node)
}

func parseGeminiUsage(data []byte) usage.Detail {
	usageNode := gjson.ParseBytes(data)
	node := usageNode.Get("usageMetadata")
	if !node.Exists() {
		node = usageNode.Get("usage_metadata")
	}
	if !node.Exists() {
		return usage.Detail{}
	}
	return parseGeminiFamilyUsageDetail(node)
}

func parseGeminiStreamUsage(line []byte) (usage.Detail, bool) {
	payload := jsonPayload(line)
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return usage.Detail{}, false
	}
	node := gjson.GetBytes(payload, "usageMetadata")
	if !node.Exists() {
		node = gjson.GetBytes(payload, "usage_metadata")
	}
	if !node.Exists() {
		return usage.Detail{}, false
	}
	return parseGeminiFamilyUsageDetail(node), true
}

func parseGeminiCLIStreamUsage(line []byte) (usage.Detail, bool) {
	payload := jsonPayload(line)
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return usage.Detail{}, false
	}
	node := gjson.GetBytes(payload, "response.usageMetadata")
	if !node.Exists() {
		node = gjson.GetBytes(payload, "usage_metadata")
	}
	if !node.Exists() {
		return usage.Detail{}, false
	}
	return parseGeminiFamilyUsageDetail(node), true
}

func parseAntigravityUsage(data []byte) usage.Detail {
	usageNode := gjson.ParseBytes(data)
	node := usageNode.Get("response.usageMetadata")
	if !node.Exists() {
		node = usageNode.Get("usageMetadata")
	}
	if !node.Exists() {
		node = usageNode.Get("usage_metadata")
	}
	if !node.Exists() {
		return usage.Detail{}
	}
	return parseGeminiFamilyUsageDetail(node)
}

func parseAntigravityStreamUsage(line []byte) (usage.Detail, bool) {
	payload := jsonPayload(line)
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return usage.Detail{}, false
	}
	node := gjson.GetBytes(payload, "response.usageMetadata")
	if !node.Exists() {
		node = gjson.GetBytes(payload, "usageMetadata")
	}
	if !node.Exists() {
		node = gjson.GetBytes(payload, "usage_metadata")
	}
	if !node.Exists() {
		return usage.Detail{}, false
	}
	return parseGeminiFamilyUsageDetail(node), true
}

var stopChunkWithoutUsage sync.Map

func rememberStopWithoutUsage(traceID string) {
	stopChunkWithoutUsage.Store(traceID, struct{}{})
	time.AfterFunc(10*time.Minute, func() { stopChunkWithoutUsage.Delete(traceID) })
}

// FilterSSEUsageMetadata removes usageMetadata from SSE events that are not
// terminal (finishReason != "stop"). Stop chunks are left untouched. This
// function is shared between aistudio and antigravity executors.
func FilterSSEUsageMetadata(payload []byte) []byte {
	if len(payload) == 0 {
		return payload
	}

	lines := bytes.Split(payload, []byte("\n"))
	modified := false
	foundData := false
	for idx, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || !bytes.HasPrefix(trimmed, []byte("data:")) {
			continue
		}
		foundData = true
		dataIdx := bytes.Index(line, []byte("data:"))
		if dataIdx < 0 {
			continue
		}
		rawJSON := bytes.TrimSpace(line[dataIdx+5:])
		traceID := gjson.GetBytes(rawJSON, "traceId").String()
		if isStopChunkWithoutUsage(rawJSON) && traceID != "" {
			rememberStopWithoutUsage(traceID)
			continue
		}
		if traceID != "" {
			if _, ok := stopChunkWithoutUsage.Load(traceID); ok && hasUsageMetadata(rawJSON) {
				stopChunkWithoutUsage.Delete(traceID)
				continue
			}
		}

		cleaned, changed := StripUsageMetadataFromJSON(rawJSON)
		if !changed {
			continue
		}
		var rebuilt []byte
		rebuilt = append(rebuilt, line[:dataIdx]...)
		rebuilt = append(rebuilt, []byte("data:")...)
		if len(cleaned) > 0 {
			rebuilt = append(rebuilt, ' ')
			rebuilt = append(rebuilt, cleaned...)
		}
		lines[idx] = rebuilt
		modified = true
	}
	if !modified {
		if !foundData {
			// Handle payloads that are raw JSON without SSE data: prefix.
			trimmed := bytes.TrimSpace(payload)
			cleaned, changed := StripUsageMetadataFromJSON(trimmed)
			if !changed {
				return payload
			}
			return cleaned
		}
		return payload
	}
	return bytes.Join(lines, []byte("\n"))
}

// StripUsageMetadataFromJSON drops usageMetadata unless finishReason is present (terminal).
// It handles both formats:
// - Aistudio: candidates.0.finishReason
// - Antigravity: response.candidates.0.finishReason
func StripUsageMetadataFromJSON(rawJSON []byte) ([]byte, bool) {
	jsonBytes := bytes.TrimSpace(rawJSON)
	if len(jsonBytes) == 0 || !gjson.ValidBytes(jsonBytes) {
		return rawJSON, false
	}

	// Check for finishReason in both aistudio and antigravity formats
	finishReason := gjson.GetBytes(jsonBytes, "candidates.0.finishReason")
	if !finishReason.Exists() {
		finishReason = gjson.GetBytes(jsonBytes, "response.candidates.0.finishReason")
	}
	terminalReason := finishReason.Exists() && strings.TrimSpace(finishReason.String()) != ""

	usageMetadata := gjson.GetBytes(jsonBytes, "usageMetadata")
	if !usageMetadata.Exists() {
		usageMetadata = gjson.GetBytes(jsonBytes, "response.usageMetadata")
	}

	// Terminal chunk: keep as-is.
	if terminalReason {
		return rawJSON, false
	}

	// Nothing to strip
	if !usageMetadata.Exists() {
		return rawJSON, false
	}

	// Remove usageMetadata from both possible locations
	cleaned := jsonBytes
	var changed bool

	if usageMetadata = gjson.GetBytes(cleaned, "usageMetadata"); usageMetadata.Exists() {
		// Rename usageMetadata to cpaUsageMetadata in the message_start event of Claude
		cleaned, _ = sjson.SetRawBytes(cleaned, "cpaUsageMetadata", []byte(usageMetadata.Raw))
		cleaned, _ = sjson.DeleteBytes(cleaned, "usageMetadata")
		changed = true
	}

	if usageMetadata = gjson.GetBytes(cleaned, "response.usageMetadata"); usageMetadata.Exists() {
		// Rename usageMetadata to cpaUsageMetadata in the message_start event of Claude
		cleaned, _ = sjson.SetRawBytes(cleaned, "response.cpaUsageMetadata", []byte(usageMetadata.Raw))
		cleaned, _ = sjson.DeleteBytes(cleaned, "response.usageMetadata")
		changed = true
	}

	return cleaned, changed
}

func hasUsageMetadata(jsonBytes []byte) bool {
	if len(jsonBytes) == 0 || !gjson.ValidBytes(jsonBytes) {
		return false
	}
	if gjson.GetBytes(jsonBytes, "usageMetadata").Exists() {
		return true
	}
	if gjson.GetBytes(jsonBytes, "response.usageMetadata").Exists() {
		return true
	}
	return false
}

func isStopChunkWithoutUsage(jsonBytes []byte) bool {
	if len(jsonBytes) == 0 || !gjson.ValidBytes(jsonBytes) {
		return false
	}
	finishReason := gjson.GetBytes(jsonBytes, "candidates.0.finishReason")
	if !finishReason.Exists() {
		finishReason = gjson.GetBytes(jsonBytes, "response.candidates.0.finishReason")
	}
	trimmed := strings.TrimSpace(finishReason.String())
	if !finishReason.Exists() || trimmed == "" {
		return false
	}
	return !hasUsageMetadata(jsonBytes)
}

func jsonPayload(line []byte) []byte {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil
	}
	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil
	}
	if bytes.HasPrefix(trimmed, []byte("event:")) {
		return nil
	}
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		trimmed = bytes.TrimSpace(trimmed[len("data:"):])
	}
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	return trimmed
}
