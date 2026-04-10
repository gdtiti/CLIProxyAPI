package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

func TestNewHookDisabledCases(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		if hook := NewHook(nil); hook != nil {
			t.Fatalf("NewHook(nil) = %#v, want nil", hook)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		cfg := &internalconfig.RequestAuditConfig{Enable: false, Endpoint: "http://example.test/hook"}
		if hook := NewHook(cfg); hook != nil {
			t.Fatalf("NewHook(disabled) = %#v, want nil", hook)
		}
	})

	t.Run("empty endpoint", func(t *testing.T) {
		cfg := &internalconfig.RequestAuditConfig{Enable: true}
		if hook := NewHook(cfg); hook != nil {
			t.Fatalf("NewHook(empty endpoint) = %#v, want nil", hook)
		}
	})
}

func TestNewHookUnsupportedSchemeReturnsNil(t *testing.T) {
	cfg := &internalconfig.RequestAuditConfig{
		Enable:   true,
		Endpoint: "ftp://example.test/hook",
	}
	if hook := NewHook(cfg); hook != nil {
		t.Fatalf("NewHook(unsupported scheme) = %#v, want nil", hook)
	}
}

func TestHookEmitProviderFilterAndPayloadCapture(t *testing.T) {
	received := make(chan Event, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("decode event: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- event
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	hook := NewHook(&internalconfig.RequestAuditConfig{
		Enable:         true,
		Endpoint:       server.URL,
		Providers:      []string{"codex"},
		QueueSize:      4,
		TimeoutSeconds: 1,
		MaxBodyBytes:   4096,
	})
	if hook == nil {
		t.Fatal("NewHook() returned nil")
	}

	longText := strings.Repeat("a", 60) + strings.Repeat("b", 60)
	ctx := WithRequest(context.Background(), coreexecutor.Options{
		Stream:          false,
		Headers:         http.Header{"X-Test": []string{"1"}},
		OriginalRequest: []byte(`{"items":[1,2,3,4,5,6,7],"text":"` + longText + `"}`),
		SourceFormat:    sdktranslator.FromString("openai"),
	}, coreexecutor.Request{Model: "gpt-5"}, 4096)
	SetAttempt(ctx, "codex", "gpt-5-codex", "auth-1", "label-1", "auth.json", "/tmp/auth.json")
	SetClientResponse(ctx, []byte(`{"items":[1,2,3,4,5,6,7]}`))

	hook.Emit(ctx, ResultInfo{Provider: "claude", Model: "gpt-5", Success: true, AuthID: "auth-1"})
	select {
	case event := <-received:
		t.Fatalf("filtered provider unexpectedly emitted event: %+v", event)
	case <-time.After(150 * time.Millisecond):
	}

	hook.Emit(ctx, ResultInfo{Provider: "codex", Model: "gpt-5", Success: true, AuthID: "auth-1"})

	select {
	case event := <-received:
		if event.Provider != "codex" {
			t.Fatalf("event.Provider = %q, want codex", event.Provider)
		}
		if event.UpstreamModel != "gpt-5-codex" {
			t.Fatalf("event.UpstreamModel = %q, want gpt-5-codex", event.UpstreamModel)
		}
		if event.SourceFormat != "openai" {
			t.Fatalf("event.SourceFormat = %q, want openai", event.SourceFormat)
		}
		var requestBody map[string]any
		if err := json.Unmarshal(event.ClientRequest, &requestBody); err != nil {
			t.Fatalf("unmarshal ClientRequest: %v", err)
		}
		items := requestBody["items"].([]any)
		if len(items) != 6 {
			t.Fatalf("len(request items) = %d, want 6", len(items))
		}
		var responseBody map[string]any
		if err := json.Unmarshal(event.ClientResponse, &responseBody); err != nil {
			t.Fatalf("unmarshal ClientResponse: %v", err)
		}
		respItems := responseBody["items"].([]any)
		if len(respItems) != 6 {
			t.Fatalf("len(response items) = %d, want 6", len(respItems))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for emitted audit event")
	}
}

func TestHookEmitQueueFullDoesNotBlock(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	hook := NewHook(&internalconfig.RequestAuditConfig{
		Enable:         true,
		Endpoint:       server.URL,
		QueueSize:      1,
		TimeoutSeconds: 2,
	})
	if hook == nil {
		t.Fatal("NewHook() returned nil")
	}

	makeCtx := func(model string) context.Context {
		return WithRequest(context.Background(), coreexecutor.Options{
			OriginalRequest: []byte(`{"model":"` + model + `"}`),
			SourceFormat:    sdktranslator.FromString("openai"),
		}, coreexecutor.Request{Model: model}, 1024)
	}

	hook.Emit(makeCtx("gpt-5.1"), ResultInfo{Provider: "codex", Model: "gpt-5.1", Success: true})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first request did not reach server")
	}

	hook.Emit(makeCtx("gpt-5.2"), ResultInfo{Provider: "codex", Model: "gpt-5.2", Success: true})

	start := time.Now()
	hook.Emit(makeCtx("gpt-5.3"), ResultInfo{Provider: "codex", Model: "gpt-5.3", Success: true})
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Emit() blocked for %v while queue was full", elapsed)
	}

	close(release)
}

func TestHookEmitSendFailureDoesNotBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hook := NewHook(&internalconfig.RequestAuditConfig{
		Enable:         true,
		Endpoint:       server.URL,
		QueueSize:      2,
		TimeoutSeconds: 1,
	})
	if hook == nil {
		t.Fatal("NewHook() returned nil")
	}

	ctx := WithRequest(context.Background(), coreexecutor.Options{
		OriginalRequest: []byte(`{"model":"gpt-5"}`),
		SourceFormat:    sdktranslator.FromString("openai"),
	}, coreexecutor.Request{Model: "gpt-5"}, 1024)

	start := time.Now()
	hook.Emit(ctx, ResultInfo{Provider: "codex", Model: "gpt-5", Success: false, StatusCode: http.StatusInternalServerError, ErrorMessage: "boom"})
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Emit() blocked for %v on failing upstream hook", elapsed)
	}
}
