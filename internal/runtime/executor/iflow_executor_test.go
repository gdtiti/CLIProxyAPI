package executor

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
)

func TestIFlowExecutorParseSuffix(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantBase  string
		wantLevel string
	}{
		{"no suffix", "glm-4", "glm-4", ""},
		{"glm with suffix", "glm-4.1-flash(high)", "glm-4.1-flash", "high"},
		{"minimax no suffix", "minimax-m2", "minimax-m2", ""},
		{"minimax with suffix", "minimax-m2.1(medium)", "minimax-m2.1", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := thinking.ParseSuffix(tt.model)
			if result.ModelName != tt.wantBase {
				t.Errorf("ParseSuffix(%q).ModelName = %q, want %q", tt.model, result.ModelName, tt.wantBase)
			}
		})
	}
}

func TestApplyIFlowHeaders(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		stream      bool
		checkAccept bool // if true, check that Accept header is NOT set
		checkConvID bool // if true, check that conversation-id IS set
	}{
		{
			name:        "non-streaming request without Accept header",
			apiKey:      "test-api-key",
			stream:      false,
			checkAccept: true,
			checkConvID: true,
		},
		{
			name:        "streaming request without Accept header",
			apiKey:      "test-api-key",
			stream:      true,
			checkAccept: true,
			checkConvID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: make(http.Header),
			}
			applyIFlowHeaders(req, tt.apiKey, tt.stream)

			if tt.checkAccept {
				if req.Header.Get("Accept") != "" {
					t.Errorf("Accept header should not be set, got: %s", req.Header.Get("Accept"))
				}
			}

			if tt.checkConvID {
				convID := req.Header.Get("conversation-id")
				if convID == "" {
					t.Error("conversation-id header should be set")
				}
				// Verify it's a valid UUID format
				if _, err := uuid.Parse(convID); err != nil {
					t.Errorf("conversation-id should be a valid UUID, got: %s", convID)
				}
			}

			// Verify other required headers are still set
			if req.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type should be application/json, got: %s", req.Header.Get("Content-Type"))
			}

			if req.Header.Get("Authorization") != "Bearer "+tt.apiKey {
				t.Errorf("Authorization header incorrect, got: %s", req.Header.Get("Authorization"))
			}

			if req.Header.Get("session-id") == "" {
				t.Error("session-id header should be set")
			}
		})
	}
}

func TestPreserveReasoningContentInMessages(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte // nil means output should equal input
	}{
		{
			"non-glm model passthrough",
			[]byte(`{"model":"gpt-4","messages":[]}`),
			nil,
		},
		{
			"glm model with empty messages",
			[]byte(`{"model":"glm-4","messages":[]}`),
			nil,
		},
		{
			"glm model preserves existing reasoning_content",
			[]byte(`{"model":"glm-4","messages":[{"role":"assistant","content":"hi","reasoning_content":"thinking..."}]}`),
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preserveReasoningContentInMessages(tt.input)
			want := tt.want
			if want == nil {
				want = tt.input
			}
			if string(got) != string(want) {
				t.Errorf("preserveReasoningContentInMessages() = %s, want %s", got, want)
			}
		})
	}
}
