package management

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/authfs"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	defaultUnauthorizedDeleteThreshold = 3
	defaultUnauthorizedDeleteWindow    = 10 * time.Minute
	// Keep this probe UA aligned with the Codex executor.
	managedCodexUsageProbeUserAgent = "codex_cli_rs/0.101.0 (Mac OS 26.0.1; arm64) Apple_Terminal/464"
)

type authRuntimeMaintenanceHook struct {
	handler *Handler
}

func newAuthRuntimeMaintenanceHook(handler *Handler) *authRuntimeMaintenanceHook {
	return &authRuntimeMaintenanceHook{handler: handler}
}

func (h *authRuntimeMaintenanceHook) OnAuthRegistered(context.Context, *coreauth.Auth) {}

func (h *authRuntimeMaintenanceHook) OnAuthUpdated(context.Context, *coreauth.Auth) {}

func (h *authRuntimeMaintenanceHook) OnResult(ctx context.Context, result coreauth.Result) {
	if h == nil || h.handler == nil || result.AuthID == "" {
		return
	}
	auth, ok := h.handler.authManager.GetByID(result.AuthID)
	if !ok || auth == nil {
		return
	}
	if h.handler.handleCodexRequestMaintenance(ctx, auth, result) {
		return
	}
	if result.Success {
		return
	}
	if shouldDisableCodexUsageLimitReached(result) {
		if !shouldTrackUnauthorizedCleanup(auth) {
			return
		}
		if err := h.handler.handleManagedAuthDisable(ctx, auth, "disabled after usage_limit_reached"); err != nil {
			log.WithError(err).Warnf("management: usage limit cleanup failed for %s", auth.ID)
		}
		return
	}
	if reason, ok := managedProtectiveDisableReason(auth, result); ok {
		if !shouldTrackUnauthorizedCleanup(auth) {
			return
		}
		if err := h.handler.handleManagedAuthDisable(ctx, auth, reason); err != nil {
			log.WithError(err).Warnf("management: protective disable failed for %s", auth.ID)
		}
		return
	}
	if reason, ok := h.handler.disableStatusCodeReason(result); ok {
		if !shouldTrackUnauthorizedCleanup(auth) {
			return
		}
		if err := h.handler.handleManagedAuthDisable(ctx, auth, reason); err != nil {
			log.WithError(err).Warnf("management: configured status-code disable failed for %s", auth.ID)
		}
		return
	}
	if statusCode := unauthorizedCleanupStatusCode(result.Error); statusCode != 401 {
		return
	}

	if !shouldTrackUnauthorizedCleanup(auth) {
		return
	}

	now := time.Now()
	count := coreauth.RecordUnauthorizedFailure(auth, now, h.handler.unauthorizedDeleteWindow())
	if count < h.handler.unauthorizedDeleteThreshold() {
		if _, err := h.handler.authManager.Update(ctx, auth); err != nil {
			log.WithError(err).Warnf("management: persist unauthorized history failed for %s", auth.ID)
		}
		return
	}

	if err := h.handler.handleUnauthorizedAuthCleanup(ctx, auth); err != nil {
		log.WithError(err).Warnf("management: unauthorized cleanup failed for %s", auth.ID)
		if _, updateErr := h.handler.authManager.Update(ctx, auth); updateErr != nil {
			log.WithError(updateErr).Warnf("management: persist unauthorized history after cleanup failure failed for %s", auth.ID)
		}
	}
}

func (h *Handler) handleCodexRequestMaintenance(ctx context.Context, auth *coreauth.Auth, result coreauth.Result) bool {
	limit := h.codexMaxRequestCount()
	quotaInterval := h.codexQuotaCheckRequestInterval()
	if auth == nil || (limit <= 0 && quotaInterval <= 0) {
		return false
	}
	if !shouldTrackCodexRequestCountCleanup(auth, result.Provider) {
		return false
	}

	count := coreauth.RecordCompletedRequest(auth)
	if limit > 0 && count >= limit {
		if err := h.handleManagedAuthDisable(ctx, auth, "disabled after codex_max_request_count"); err != nil {
			log.WithError(err).Warnf("management: codex request-count disable failed for %s", auth.ID)
			if _, updateErr := h.authManager.Update(ctx, auth); updateErr != nil {
				log.WithError(updateErr).Warnf("management: persist codex request count after disable failure failed for %s", auth.ID)
			}
			return false
		}
		return true
	}

	if quotaInterval > 0 && count%quotaInterval == 0 {
		statusCode, err := h.probeManagedCodexUsageStatus(ctx, auth)
		if err != nil {
			log.WithError(err).Warnf("management: codex quota probe failed for %s", auth.ID)
		} else if statusCode == http.StatusUnauthorized {
			if err := h.handleManagedAuthDisable(ctx, auth, "disabled after codex_quota_probe_401"); err != nil {
				log.WithError(err).Warnf("management: codex quota 401 disable failed for %s", auth.ID)
				if _, updateErr := h.authManager.Update(ctx, auth); updateErr != nil {
					log.WithError(updateErr).Warnf("management: persist codex request count after quota disable failure failed for %s", auth.ID)
				}
				return false
			}
			return true
		}
	}

	if _, err := h.authManager.Update(ctx, auth); err != nil {
		log.WithError(err).Warnf("management: persist codex request count failed for %s", auth.ID)
	}
	return false
}

func unauthorizedCleanupStatusCode(err *coreauth.Error) int {
	if err == nil {
		return 0
	}
	return err.HTTPStatus
}

func shouldDisableCodexUsageLimitReached(result coreauth.Result) bool {
	if result.Success || result.Error == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(result.Provider), "codex") {
		return false
	}
	if result.Error.HTTPStatus != http.StatusTooManyRequests {
		return false
	}
	return containsUsageLimitReached(result.Error.Message)
}

func managedProtectiveDisableReason(auth *coreauth.Auth, result coreauth.Result) (string, bool) {
	if result.Success || auth == nil {
		return "", false
	}
	if statusCode := unauthorizedCleanupStatusCode(result.Error); statusCode == http.StatusTooManyRequests {
		return fmt.Sprintf("http_%d", statusCode), true
	}
	if auth.Quota.Exceeded {
		if auth.Quota.BackoffLevel > 0 {
			return fmt.Sprintf("quota_strikes_%d", auth.Quota.BackoffLevel), true
		}
		return "quota_exceeded", true
	}
	return "", false
}

func containsUsageLimitReached(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.Contains(strings.ToLower(raw), "usage_limit_reached") {
		return true
	}
	if !gjson.Valid(raw) {
		return false
	}
	root := gjson.Parse(raw)
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

func shouldTrackUnauthorizedCleanup(auth *coreauth.Auth) bool {
	if auth == nil {
		return false
	}
	if strings.TrimSpace(authAttribute(auth, "gemini_virtual_project")) != "" {
		return true
	}
	if strings.TrimSpace(authAttribute(auth, "path")) != "" {
		return true
	}
	fileName := strings.TrimSpace(auth.FileName)
	return strings.HasSuffix(strings.ToLower(fileName), ".json")
}

func shouldTrackCodexRequestCountCleanup(auth *coreauth.Auth, provider string) bool {
	if auth == nil {
		return false
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(auth.Provider))
	}
	if provider != "codex" || auth.Disabled {
		return false
	}
	return shouldTrackUnauthorizedCleanup(auth)
}

func (h *Handler) unauthorizedDeleteThreshold() int {
	if h != nil && h.cfg != nil && h.cfg.AuthRuntime.UnauthorizedDeleteThreshold > 0 {
		return h.cfg.AuthRuntime.UnauthorizedDeleteThreshold
	}
	return defaultUnauthorizedDeleteThreshold
}

func (h *Handler) unauthorizedDeleteWindow() time.Duration {
	if h != nil && h.cfg != nil && h.cfg.AuthRuntime.UnauthorizedDeleteWindowSeconds > 0 {
		return time.Duration(h.cfg.AuthRuntime.UnauthorizedDeleteWindowSeconds) * time.Second
	}
	return defaultUnauthorizedDeleteWindow
}

func (h *Handler) codexMaxRequestCount() int {
	if h != nil && h.cfg != nil && h.cfg.AuthMaintenance.CodexMaxRequestCount > 0 {
		return h.cfg.AuthMaintenance.CodexMaxRequestCount
	}
	return 0
}

func (h *Handler) codexQuotaCheckRequestInterval() int {
	if h != nil && h.cfg != nil && h.cfg.AuthMaintenance.CodexQuotaCheckRequestInterval > 0 {
		return h.cfg.AuthMaintenance.CodexQuotaCheckRequestInterval
	}
	return 0
}

func (h *Handler) probeManagedCodexUsageStatus(ctx context.Context, auth *coreauth.Auth) (int, error) {
	if auth == nil {
		return 0, nil
	}
	apiKey := strings.TrimSpace(authAttribute(auth, "api_key"))
	if apiKey == "" {
		apiKey = tokenValueFromMetadata(auth.Metadata)
	}
	accountID, _ := auth.Metadata["account_id"].(string)
	accountID = strings.TrimSpace(accountID)
	if apiKey == "" || accountID == "" {
		return 0, nil
	}

	baseURL := strings.TrimSpace(authAttribute(auth, "base_url"))
	if baseURL == "" && auth.Metadata != nil {
		if value, ok := auth.Metadata["base_url"].(string); ok {
			baseURL = strings.TrimSpace(value)
		}
	}

	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(requestCtx, 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, managedCodexUsageURL(baseURL), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", managedCodexUsageProbeUserAgent)
	req.Header.Set("Chatgpt-Account-Id", accountID)

	resp, err := executor.NewCodexExecutor(h.cfg).HttpRequest(probeCtx, auth, req)
	if err != nil {
		return 0, err
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.WithError(errClose).Warn("management: close codex quota probe response body failed")
		}
	}()
	return resp.StatusCode, nil
}

func managedCodexUsageURL(baseURL string) string {
	const fallback = "https://chatgpt.com/backend-api/wham/usage"
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return fallback
	}
	parsed, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil || parsed.URL == nil || parsed.URL.Scheme == "" || parsed.URL.Host == "" {
		return fallback
	}
	parsed.URL.Path = "/backend-api/wham/usage"
	parsed.URL.RawQuery = ""
	parsed.URL.Fragment = ""
	return parsed.URL.String()
}

func (h *Handler) disableStatusCodeReason(result coreauth.Result) (string, bool) {
	if result.Success || result.Error == nil {
		return "", false
	}
	if h == nil || h.cfg == nil {
		return "", false
	}
	statusCode := unauthorizedCleanupStatusCode(result.Error)
	if statusCode <= 0 || !containsManagedStatusCode(h.cfg.AuthMaintenance.DisableStatusCodes, statusCode) {
		return "", false
	}
	return fmt.Sprintf("http_%d", statusCode), true
}

func (h *Handler) handleUnauthorizedAuthCleanup(ctx context.Context, auth *coreauth.Auth) error {
	if auth == nil {
		return fmt.Errorf("auth is nil")
	}
	if projectID := strings.TrimSpace(authAttribute(auth, "gemini_virtual_project")); projectID != "" {
		return h.removeGeminiVirtualAuth(ctx, auth, projectID)
	}
	return h.handleManagedAuthDisable(ctx, auth, "disabled after unauthorized threshold")
}

func (h *Handler) handleManagedAuthDisable(ctx context.Context, auth *coreauth.Auth, reason string) error {
	if auth == nil {
		return fmt.Errorf("auth is nil")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "disabled via management API"
	}
	path := h.resolveManagedAuthPath(auth)
	if path == "" {
		h.disableAuthWithMessage(ctx, auth.ID, reason)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			h.disableAuthWithMessage(ctx, auth.ID, reason)
			return nil
		}
		return fmt.Errorf("read auth file: %w", err)
	}
	if len(data) == 0 {
		h.disableAuthWithMessage(ctx, auth.ID, reason)
		return nil
	}

	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("unmarshal auth file: %w", err)
	}
	metadata["disabled"] = true
	coreauth.MarkAuthMaintenanceAutoRecovery(metadata, reason, time.Now())
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal auth file: %w", err)
	}
	if err := h.writeValidatedAuthFile(ctx, path, raw, "Disable auth"); err != nil {
		return err
	}
	h.disableAuthWithMessage(ctx, auth.ID, reason)
	return nil
}

func (h *Handler) removeGeminiVirtualAuth(ctx context.Context, auth *coreauth.Auth, projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return h.handleManagedAuthDisable(ctx, auth, "disabled after unauthorized threshold")
	}

	target := auth
	if parentID := strings.TrimSpace(authAttribute(auth, "gemini_virtual_parent")); parentID != "" && h != nil && h.authManager != nil {
		if parent, ok := h.authManager.GetByID(parentID); ok && parent != nil {
			target = parent
		}
	}

	path, metadata, err := h.loadAuthFileMetadata(target)
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return h.handleManagedAuthDisable(ctx, target, "disabled after unauthorized threshold")
	}

	projects := splitManagedGeminiProjectIDs(metadata["project_id"])
	remaining := make([]string, 0, len(projects))
	for _, candidate := range projects {
		if candidate == projectID {
			continue
		}
		remaining = append(remaining, candidate)
	}
	if len(remaining) == len(projects) {
		return h.handleManagedAuthDisable(ctx, target, "disabled after unauthorized threshold")
	}
	if len(remaining) == 0 {
		return h.handleManagedAuthDisable(ctx, target, "disabled after unauthorized threshold")
	}

	metadata["project_id"] = strings.Join(remaining, ",")
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal updated gemini auth metadata: %w", err)
	}
	return h.writeValidatedAuthFile(ctx, path, raw, "Update auth")
}

func splitManagedGeminiProjectIDs(raw any) []string {
	value, _ := raw.(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(parts))
	projects := make([]string, 0, len(parts))
	for _, part := range parts {
		projectID := strings.TrimSpace(part)
		if projectID == "" {
			continue
		}
		if _, exists := seen[projectID]; exists {
			continue
		}
		seen[projectID] = struct{}{}
		projects = append(projects, projectID)
	}
	return projects
}

func (h *Handler) removeManagedAuthFile(ctx context.Context, auth *coreauth.Auth) error {
	if auth == nil {
		return fmt.Errorf("auth is nil")
	}
	path := h.resolveManagedAuthPath(auth)
	if path == "" {
		h.disableAuth(ctx, auth.ID)
		return nil
	}
	if h != nil && h.cfg != nil && authfs.IsTrashPath(h.cfg.AuthDir, path) {
		h.disableAuth(ctx, auth.ID)
		return nil
	}
	logicalRel, ok := h.authLogicalRelativePath(auth, filepath.Base(path))
	if !ok {
		logicalRel = filepath.Base(path)
	}
	if _, err := h.moveAuthFileToTrash(ctx, path, logicalRel, "Trash auth"); err != nil {
		if errors.Is(err, errAuthFileNotFound) {
			if errDelete := h.deleteTokenRecord(ctx, path); errDelete != nil {
				return errDelete
			}
			h.disableAuth(ctx, auth.ID)
			return nil
		}
		return err
	}
	h.disableAuth(ctx, auth.ID)
	return nil
}

func (h *Handler) resolveManagedAuthPath(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if path := strings.TrimSpace(authAttribute(auth, "path")); path != "" {
		if filepath.IsAbs(path) {
			return path
		}
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
		return path
	}
	fileName := strings.TrimSpace(auth.FileName)
	if fileName == "" {
		fileName = strings.TrimSpace(auth.ID)
	}
	if fileName == "" {
		return ""
	}
	if filepath.IsAbs(fileName) {
		return fileName
	}
	if h != nil && h.cfg != nil && strings.TrimSpace(h.cfg.AuthDir) != "" {
		path := filepath.Join(h.cfg.AuthDir, fileName)
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
		return path
	}
	return fileName
}

func (h *Handler) writeValidatedAuthFile(ctx context.Context, path string, data []byte, action string) error {
	if err := h.validateUploadedAuthFile(path, data); err != nil {
		return err
	}
	previous, hadPrevious, err := snapshotExistingAuthFile(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to prepare auth dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	if err := h.persistAuthFileToStore(ctx, action, path); err != nil {
		if restoreErr := restoreUploadedAuthFile(path, previous, hadPrevious); restoreErr != nil {
			log.WithError(restoreErr).Warnf("failed to restore auth file after store persist error: %s", filepath.Base(path))
		}
		return err
	}
	if err := h.reloadAuthFile(ctx, path, data); err != nil {
		return err
	}
	return nil
}

func containsManagedStatusCode(codes []int, want int) bool {
	if want == 0 {
		return false
	}
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}
