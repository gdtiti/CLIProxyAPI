package management

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type uploadPersistStore struct {
	persistErr   error
	persistCalls int
	lastMessage  string
	lastPaths    []string
}

func (s *uploadPersistStore) List(context.Context) ([]*coreauth.Auth, error) { return nil, nil }

func (s *uploadPersistStore) Save(context.Context, *coreauth.Auth) (string, error) {
	return "", nil
}

func (s *uploadPersistStore) Delete(context.Context, string) error { return nil }

func (s *uploadPersistStore) PersistAuthFiles(_ context.Context, message string, paths ...string) error {
	s.persistCalls++
	s.lastMessage = message
	s.lastPaths = append([]string(nil), paths...)
	return s.persistErr
}

func TestUploadAuthFileMultipart_PersistsStoreAndReloadsManager(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	store := &uploadPersistStore{}
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = store

	payload := `{"type":"codex","email":"upload@example.com","prefix":"team-a","proxy_url":"https://proxy.example.com","priority":7}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "codex-upload.json")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte(payload)); err != nil {
		t.Fatalf("write multipart payload: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("UploadAuthFile status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if store.persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1", store.persistCalls)
	}

	expectedPath := filepath.Join(authDir, "codex-upload.json")
	if len(store.lastPaths) != 1 || !sameAuthPath(store.lastPaths[0], expectedPath) {
		t.Fatalf("persist paths = %#v, want [%q]", store.lastPaths, expectedPath)
	}
	if store.lastMessage != "Upload auth codex-upload.json" {
		t.Fatalf("persist message = %q, want %q", store.lastMessage, "Upload auth codex-upload.json")
	}

	authID := h.authIDForPath(expectedPath)
	uploaded, ok := manager.GetByID(authID)
	if !ok || uploaded == nil {
		t.Fatalf("expected uploaded auth %q in manager", authID)
	}
	if uploaded.Prefix != "team-a" {
		t.Fatalf("manager prefix = %q, want %q", uploaded.Prefix, "team-a")
	}
	if uploaded.ProxyURL != "https://proxy.example.com" {
		t.Fatalf("manager proxy_url = %q, want %q", uploaded.ProxyURL, "https://proxy.example.com")
	}
	if got := uploaded.Attributes["priority"]; got != "7" {
		t.Fatalf("manager priority = %q, want %q", got, "7")
	}

	raw, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	var metadata map[string]any
	if err = json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("unmarshal uploaded file: %v", err)
	}
	if got := metadata["email"]; got != "upload@example.com" {
		t.Fatalf("file email = %v, want %q", got, "upload@example.com")
	}

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	h.ListAuthFiles(listCtx)

	if listRec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var listPayload struct {
		Files []map[string]any `json:"files"`
	}
	if err = json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(listPayload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(listPayload.Files))
	}
	entry := listPayload.Files[0]
	if got := entry["prefix"]; got != "team-a" {
		t.Fatalf("list prefix = %v, want %q", got, "team-a")
	}
	if got := entry["proxy_url"]; got != "https://proxy.example.com" {
		t.Fatalf("list proxy_url = %v, want %q", got, "https://proxy.example.com")
	}
	if got := entry["priority"]; got != float64(7) {
		t.Fatalf("list priority = %v, want %v", got, float64(7))
	}
}

func TestUploadAuthFileMultipart_EnrichesCodexAccountIDFromIDToken(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	store := &uploadPersistStore{}
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = store

	idToken := buildCodexUploadTestIDToken(t, "acc-upload")
	payload := `{"type":"codex","email":"upload@example.com","id_token":"` + idToken + `","access_token":"tok-upload"}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "codex-upload-account.json")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = part.Write([]byte(payload)); err != nil {
		t.Fatalf("write multipart payload: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("UploadAuthFile status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	authID := h.authIDForPath(filepath.Join(authDir, "codex-upload-account.json"))
	uploaded, ok := manager.GetByID(authID)
	if !ok || uploaded == nil {
		t.Fatalf("expected uploaded auth %q in manager", authID)
	}
	if got := uploaded.Metadata["account_id"]; got != "acc-upload" {
		t.Fatalf("manager account_id = %v, want %q", got, "acc-upload")
	}

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	h.ListAuthFiles(listCtx)

	if listRec.Code != http.StatusOK {
		t.Fatalf("ListAuthFiles status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var listPayload struct {
		Files []map[string]any `json:"files"`
	}
	if err = json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(listPayload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(listPayload.Files))
	}
	entry := listPayload.Files[0]
	if got := entry["account_id"]; got != "acc-upload" {
		t.Fatalf("list account_id = %v, want %q", got, "acc-upload")
	}
	if got := entry["chatgpt_account_id"]; got != "acc-upload" {
		t.Fatalf("list chatgpt_account_id = %v, want %q", got, "acc-upload")
	}
}

func TestUploadAuthFileRaw_PersistFailureReturnsErrorAndRestoresFile(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	store := &uploadPersistStore{persistErr: errors.New("persist failed")}
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = store

	fileName := "codex-upload-raw.json"
	path := filepath.Join(authDir, fileName)
	if err := os.WriteFile(path, []byte(`{"type":"codex","email":"old@example.com"}`), 0o600); err != nil {
		t.Fatalf("seed existing auth file: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(
		http.MethodPost,
		"/v0/management/auth-files?name="+fileName,
		strings.NewReader(`{"type":"codex","email":"new@example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("UploadAuthFile status = %d, want %d, body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "persist failed") {
		t.Fatalf("expected persist error in response, body=%s", recorder.Body.String())
	}
	if store.persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1", store.persistCalls)
	}
	if len(manager.List()) != 0 {
		t.Fatalf("manager should remain unchanged after persist failure")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored auth file: %v", err)
	}
	if string(raw) != `{"type":"codex","email":"old@example.com"}` {
		t.Fatalf("restored auth file = %s, want original content", string(raw))
	}
}

func buildCodexUploadTestIDToken(t *testing.T, accountID string) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := map[string]any{
		"email": "user@example.com",
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

func TestUploadAuthFilesBatchMultipart_PersistsAllFiles(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	store := &uploadPersistStore{}
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = store

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	parts := map[string]string{
		"codex-a.json": `{"type":"codex","email":"a@example.com"}`,
		"codex-b.json": `{"type":"codex","email":"b@example.com"}`,
	}
	for name, payload := range parts {
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatalf("create form file %s: %v", name, err)
		}
		if _, err := part.Write([]byte(payload)); err != nil {
			t.Fatalf("write form file %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/batch", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFilesBatch(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("UploadAuthFilesBatch status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if store.persistCalls != 2 {
		t.Fatalf("persist calls = %d, want 2", store.persistCalls)
	}
	for name := range parts {
		if _, err := os.Stat(filepath.Join(authDir, name)); err != nil {
			t.Fatalf("expected uploaded auth file %s: %v", name, err)
		}
	}
	if len(manager.List()) != 2 {
		t.Fatalf("manager auth count = %d, want 2", len(manager.List()))
	}
}

func TestUploadAuthFilesBatchZip_ExpandsJsonEntries(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	store := &uploadPersistStore{}
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = store

	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)
	entries := map[string]string{
		"nested/codex-a.json": `{"type":"codex","email":"a@example.com"}`,
		"codex-b.json":        `{"type":"codex","email":"b@example.com"}`,
	}
	for name, payload := range entries {
		entry, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(payload)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "auths.zip")
	if err != nil {
		t.Fatalf("create zip form file: %v", err)
	}
	if _, err := part.Write(zipBuffer.Bytes()); err != nil {
		t.Fatalf("write zip payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/batch", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFilesBatch(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("UploadAuthFilesBatch status = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if store.persistCalls != 2 {
		t.Fatalf("persist calls = %d, want 2", store.persistCalls)
	}
	for _, name := range []string{"codex-a.json", "codex-b.json"} {
		if _, err := os.Stat(filepath.Join(authDir, name)); err != nil {
			t.Fatalf("expected uploaded auth file %s: %v", name, err)
		}
	}
	if len(manager.List()) != 2 {
		t.Fatalf("manager auth count = %d, want 2", len(manager.List()))
	}
}
