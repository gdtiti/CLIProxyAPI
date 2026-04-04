package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestCodexExecutorCompactMimicBypassesCompactEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantPath string
	}{
		{
			name:     "mimic off uses compact endpoint",
			cfg:      &config.Config{},
			wantPath: "/backend-api/codex/responses/compact",
		},
		{
			name: "mimic safe uses responses endpoint",
			cfg: &config.Config{
				SDKConfig: config.SDKConfig{
					CodexMimic: config.CodexMimicConfig{Mode: config.CodexMimicModeSafe},
				},
			},
			wantPath: "/backend-api/codex/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				if r.URL.Path == "/backend-api/codex/responses" {
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_123\",\"object\":\"response\",\"status\":\"completed\",\"output\":[]}}\n\n"))
					return
				}
				if r.URL.Path == "/backend-api/codex/responses/compact" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":"resp_compact","object":"response","status":"completed","output":[]}`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			auth := &cliproxyauth.Auth{
				ID:       "codex-mimic-test",
				Provider: "codex",
				Attributes: map[string]string{
					"api_key":  "test-key",
					"base_url": server.URL + "/backend-api/codex",
				},
			}

			exec := NewCodexExecutor(tt.cfg)
			_, err := exec.Execute(context.Background(), auth, cliproxyexecutor.Request{
				Model:   "gpt-5",
				Payload: []byte(`{"input":[{"type":"message","content":[{"type":"text","text":"hi"}]}]}`),
			}, cliproxyexecutor.Options{
				SourceFormat: "openai-response",
				Alt:          "responses/compact",
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if gotPath != tt.wantPath {
				t.Fatalf("upstream path = %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}
