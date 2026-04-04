package chat_completions

import "testing"

func TestParsedRequestCache_HitAndEviction(t *testing.T) {
	cache := newParsedRequestCache(2)

	rawA := []byte(`{"messages":[{"role":"user","content":"a"}]}`)
	rawB := []byte(`{"messages":[{"role":"user","content":"b"}]}`)
	rawC := []byte(`{"messages":[{"role":"user","content":"c"}]}`)

	cache.put(requestCacheKey(rawA), chatReqInput{ReasoningEffort: "low"})
	cache.put(requestCacheKey(rawB), chatReqInput{ReasoningEffort: "medium"})

	if req, ok := cache.get(requestCacheKey(rawA)); !ok || req.ReasoningEffort != "low" {
		t.Fatalf("expected cache hit for rawA, got ok=%v req=%+v", ok, req)
	}

	cache.put(requestCacheKey(rawC), chatReqInput{ReasoningEffort: "high"})

	if _, ok := cache.get(requestCacheKey(rawB)); ok {
		t.Fatal("expected rawB to be evicted after rawA promotion and rawC insert")
	}
	if req, ok := cache.get(requestCacheKey(rawA)); !ok || req.ReasoningEffort != "low" {
		t.Fatalf("expected rawA to remain cached, got ok=%v req=%+v", ok, req)
	}
	if req, ok := cache.get(requestCacheKey(rawC)); !ok || req.ReasoningEffort != "high" {
		t.Fatalf("expected rawC to be cached, got ok=%v req=%+v", ok, req)
	}
}

func TestPrimeOpenAIRequest_EmptyInputNoop(t *testing.T) {
	previous := openAIRequestCache
	openAIRequestCache = newParsedRequestCache(2)
	t.Cleanup(func() { openAIRequestCache = previous })

	PrimeOpenAIRequest(nil)
	PrimeOpenAIRequest([]byte{})

	if openAIRequestCache.order.Len() != 0 {
		t.Fatalf("expected empty input to keep cache empty, len=%d", openAIRequestCache.order.Len())
	}
}

func TestPrimeOpenAIRequest_InvalidJSONNoCache(t *testing.T) {
	previous := openAIRequestCache
	openAIRequestCache = newParsedRequestCache(2)
	t.Cleanup(func() { openAIRequestCache = previous })

	raw := []byte(`{"messages":[`)
	PrimeOpenAIRequest(raw)

	if _, ok := cachedOpenAIRequest(raw); ok {
		t.Fatal("expected invalid JSON not to be cached")
	}
	if openAIRequestCache.order.Len() != 0 {
		t.Fatalf("expected invalid JSON not to populate cache, len=%d", openAIRequestCache.order.Len())
	}
}

func TestPrimeOpenAIRequest_CachesParsedRequest(t *testing.T) {
	previous := openAIRequestCache
	openAIRequestCache = newParsedRequestCache(2)
	t.Cleanup(func() { openAIRequestCache = previous })

	raw := []byte(`{"reasoning_effort":"high","messages":[{"role":"user","content":"hello"}]}`)
	PrimeOpenAIRequest(raw)

	req, ok := cachedOpenAIRequest(raw)
	if !ok {
		t.Fatal("expected primed request to be cached")
	}
	if req.ReasoningEffort != "high" {
		t.Fatalf("expected cached reasoning_effort=high, got %q", req.ReasoningEffort)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		t.Fatalf("unexpected cached messages: %+v", req.Messages)
	}
}
