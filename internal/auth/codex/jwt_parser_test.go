package codex

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestAccountIDFromIDToken(t *testing.T) {
	t.Run("extracts account id", func(t *testing.T) {
		token := buildTestIDToken(t, "acc-123")
		if got := AccountIDFromIDToken(token); got != "acc-123" {
			t.Fatalf("AccountIDFromIDToken() = %q, want %q", got, "acc-123")
		}
	})

	t.Run("invalid token returns empty", func(t *testing.T) {
		if got := AccountIDFromIDToken("bad-token"); got != "" {
			t.Fatalf("AccountIDFromIDToken() = %q, want empty", got)
		}
	})
}

func buildTestIDToken(t *testing.T, accountID string) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := map[string]any{
		"email": "test@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(body) + "."
}
