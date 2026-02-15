package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestPatchAuthFilesExcludedModelsBatch_AddAndNormalize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authDir := t.TempDir()

	writeJSONFile(t, filepath.Join(authDir, "a.json"), map[string]any{
		"type":            "claude",
		"excluded_models": []string{"Model-A"},
	})
	writeJSONFile(t, filepath.Join(authDir, "b.json"), map[string]any{
		"type":            "claude",
		"excluded-models": []string{"legacy-x", "model-a"},
	})

	h := &Handler{
		cfg:         &config.Config{AuthDir: authDir},
		authManager: coreauth.NewManager(nil, nil, nil),
	}

	body := map[string]any{
		"names":     []string{"a.json", "b.json"},
		"operation": "add",
		"models":    []string{"model-b", "MODEL-A"},
	}

	w := performBatchExcludedModelsPatch(t, h, body)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}

	var resp patchAuthFilesExcludedModelsBatchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if resp.Summary.Updated != 2 || resp.Summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", resp.Summary)
	}

	gotA := readExcludedModelsFromFile(t, filepath.Join(authDir, "a.json"))
	assertStringSlice(t, gotA, []string{"model-a", "model-b"})

	gotB := readExcludedModelsFromFile(t, filepath.Join(authDir, "b.json"))
	assertStringSlice(t, gotB, []string{"legacy-x", "model-a", "model-b"})

	var bMap map[string]any
	readJSONFile(t, filepath.Join(authDir, "b.json"), &bMap)
	if _, ok := bMap["excluded-models"]; ok {
		t.Fatal("expected legacy key excluded-models to be removed")
	}
}

func TestPatchAuthFilesExcludedModelsBatch_DryRunDoesNotWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authDir := t.TempDir()
	path := filepath.Join(authDir, "dry.json")
	writeJSONFile(t, path, map[string]any{
		"type":            "claude",
		"excluded_models": []string{"old-a"},
	})

	h := &Handler{
		cfg:         &config.Config{AuthDir: authDir},
		authManager: coreauth.NewManager(nil, nil, nil),
	}

	body := map[string]any{
		"names":     []string{"dry.json"},
		"operation": "set",
		"models":    []string{"new-a"},
		"dry_run":   true,
	}

	w := performBatchExcludedModelsPatch(t, h, body)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}

	var resp patchAuthFilesExcludedModelsBatchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "dry_run" {
		t.Fatalf("expected dry_run status, got %q", resp.Status)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "would_update" {
		t.Fatalf("unexpected results: %+v", resp.Results)
	}

	got := readExcludedModelsFromFile(t, path)
	assertStringSlice(t, got, []string{"old-a"})
}

func TestPatchAuthFilesExcludedModelsBatch_StopOnError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authDir := t.TempDir()
	writeJSONFile(t, filepath.Join(authDir, "ok.json"), map[string]any{
		"type":            "claude",
		"excluded_models": []string{"a"},
	})

	h := &Handler{
		cfg:         &config.Config{AuthDir: authDir},
		authManager: coreauth.NewManager(nil, nil, nil),
	}

	body := map[string]any{
		"names":         []string{"missing.json", "ok.json"},
		"operation":     "set",
		"models":        []string{"b"},
		"stop_on_error": true,
	}

	w := performBatchExcludedModelsPatch(t, h, body)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}

	var resp patchAuthFilesExcludedModelsBatchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "partial" {
		t.Fatalf("expected partial status, got %q", resp.Status)
	}
	if resp.Summary.Failed != 1 || resp.Summary.Skipped != 1 {
		t.Fatalf("unexpected summary: %+v", resp.Summary)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.Results[1].Status != "skipped" {
		t.Fatalf("expected second result skipped, got %q", resp.Results[1].Status)
	}

	got := readExcludedModelsFromFile(t, filepath.Join(authDir, "ok.json"))
	assertStringSlice(t, got, []string{"a"})
}

func performBatchExcludedModelsPatch(t *testing.T, h *Handler, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/excluded-models/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.PatchAuthFilesExcludedModelsBatch(c)
	return w
}

func writeJSONFile(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
}

func readExcludedModelsFromFile(t *testing.T, path string) []string {
	t.Helper()
	var payload map[string]any
	readJSONFile(t, path, &payload)
	return readExcludedModelsFromMetadata(payload)
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice len mismatch, got=%v want=%v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice mismatch, got=%v want=%v", got, want)
		}
	}
}
