package cliproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	internallogging "github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor/helps"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	log "github.com/sirupsen/logrus"
)

const (
	authMaintenanceRecoveryRecentLimit      = 128
	authMaintenanceRecoveryLogFileName      = "auth-maintenance-recovery.jsonl"
	authMaintenanceCodexUsageProbeUserAgent = "codex_cli_rs/0.101.0 (Mac OS 26.0.1; arm64) Apple_Terminal/464"
)

type authMaintenanceRecoveryLogRecord struct {
	Timestamp         string `json:"timestamp"`
	Event             string `json:"event"`
	AuthID            string `json:"auth_id,omitempty"`
	Path              string `json:"path,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Message           string `json:"message,omitempty"`
	Window            string `json:"window,omitempty"`
	StatusCode        int    `json:"status_code,omitempty"`
	RetryAfterSeconds int64  `json:"retry_after_seconds,omitempty"`
	Error             string `json:"error,omitempty"`
}

type authMaintenanceQuotaProbeResult struct {
	StatusCode int
	RetryAfter *time.Duration
	Window     string
}

func (s *Service) authMaintenanceRecoveryRecentSnapshot() []authMaintenanceRecoveryLogRecord {
	if s == nil {
		return nil
	}
	s.maintenanceRecoveryLogMu.Lock()
	defer s.maintenanceRecoveryLogMu.Unlock()
	out := make([]authMaintenanceRecoveryLogRecord, len(s.maintenanceRecoveryRecent))
	copy(out, s.maintenanceRecoveryRecent)
	return out
}

func (s *Service) authMaintenanceRecoveryLogPath() string {
	cfg := (*config.Config)(nil)
	if s != nil {
		s.cfgMu.RLock()
		cfg = s.cfg
		s.cfgMu.RUnlock()
	}
	logDir := internallogging.ResolveLogDirectory(cfg)
	if strings.TrimSpace(logDir) == "" {
		return ""
	}
	return filepath.Join(logDir, authMaintenanceRecoveryLogFileName)
}

func (s *Service) recordAuthMaintenanceRecovery(record authMaintenanceRecoveryLogRecord) {
	if s == nil {
		return
	}
	if strings.TrimSpace(record.Timestamp) == "" {
		record.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	s.maintenanceRecoveryLogMu.Lock()
	s.maintenanceRecoveryRecent = append(s.maintenanceRecoveryRecent, record)
	if overflow := len(s.maintenanceRecoveryRecent) - authMaintenanceRecoveryRecentLimit; overflow > 0 {
		s.maintenanceRecoveryRecent = append([]authMaintenanceRecoveryLogRecord(nil), s.maintenanceRecoveryRecent[overflow:]...)
	}
	s.maintenanceRecoveryLogMu.Unlock()

	path := s.authMaintenanceRecoveryLogPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.WithError(err).Warn("auth maintenance recovery: create log directory failed")
		return
	}
	raw, err := json.Marshal(record)
	if err != nil {
		log.WithError(err).Warn("auth maintenance recovery: marshal log record failed")
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.WithError(err).Warn("auth maintenance recovery: open log file failed")
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.WithError(closeErr).Warn("auth maintenance recovery: close log file failed")
		}
	}()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		log.WithError(err).Warn("auth maintenance recovery: append log record failed")
	}
}

func (s *Service) authMaintenanceRecoveryCandidateForAuth(auth *coreauth.Auth, authDir string, now time.Time) (authMaintenanceCandidate, bool) {
	if auth == nil {
		return authMaintenanceCandidate{}, false
	}
	if !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		return authMaintenanceCandidate{}, false
	}
	if !(auth.Disabled || auth.Status == coreauth.StatusDisabled) {
		return authMaintenanceCandidate{}, false
	}
	if !coreauth.IsAuthMaintenanceAutoRecoverable(auth) {
		return authMaintenanceCandidate{}, false
	}
	if authMaintenancePendingAction(auth, authMaintenanceActionDelete) || authMaintenancePendingAction(auth, authMaintenanceActionDisable) {
		return authMaintenanceCandidate{}, false
	}
	if nextCheckAt, ok := coreauth.AuthMaintenanceAutoRecoveryNextCheckAt(auth); ok && nextCheckAt.After(now) {
		return authMaintenanceCandidate{}, false
	}
	path := resolveRecoverableAuthFilePath(auth, authDir)
	if path == "" {
		return authMaintenanceCandidate{}, false
	}
	reason := coreauth.AuthMaintenanceAutoRecoveryReason(auth)
	if reason == "" {
		reason = strings.TrimSpace(auth.StatusMessage)
	}
	candidate := authMaintenanceCandidate{
		Key:    authMaintenanceCandidateKey(authMaintenanceActionRecover, path),
		Path:   path,
		Reason: reason,
		Action: authMaintenanceActionRecover,
	}
	candidate.IDs = append(candidate.IDs, s.authMaintenanceIDsForPath(path, authDir)...)
	if len(candidate.IDs) == 0 {
		id := strings.TrimSpace(auth.ID)
		if id != "" {
			candidate.IDs = append(candidate.IDs, id)
		}
	}
	if candidate.Key == "" || candidate.Path == "" || len(candidate.IDs) == 0 {
		return authMaintenanceCandidate{}, false
	}
	return candidate, true
}

func (s *Service) recoverAuthMaintenanceCandidate(ctx context.Context, candidate authMaintenanceCandidate) error {
	auth := s.authMaintenanceRecoveryAuth(candidate)
	if auth == nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "skip",
			AuthID:  firstMaintenanceAuthID(candidate.IDs),
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: "candidate auth not found",
		})
		return nil
	}
	now := time.Now()
	if nextCheckAt, ok := coreauth.AuthMaintenanceAutoRecoveryNextCheckAt(auth); ok && nextCheckAt.After(now) {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:             "skip",
			AuthID:            auth.ID,
			Path:              strings.TrimSpace(candidate.Path),
			Reason:            strings.TrimSpace(candidate.Reason),
			Message:           "waiting for next recovery check window",
			RetryAfterSeconds: int64(time.Until(nextCheckAt).Seconds()),
		})
		return nil
	}

	s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
		Event:  "candidate",
		AuthID: auth.ID,
		Path:   strings.TrimSpace(candidate.Path),
		Reason: strings.TrimSpace(candidate.Reason),
	})

	refreshAttempted := false
	if shouldRefreshRecoverableCodexAuth(auth, now) {
		refreshAttempted = true
		var terminal bool
		var err error
		auth, terminal, err = s.refreshRecoverableCodexAuth(ctx, auth, candidate)
		if err != nil {
			return err
		}
		if terminal || auth == nil {
			return nil
		}
	}
	if authMaintenanceTokenValue(auth) == "" || authMaintenanceAccountID(auth) == "" {
		message := "missing codex probe credentials"
		if refreshAttempted {
			message = "missing codex probe credentials after refresh"
		}
		return s.persistTerminalRecoveryDisable(ctx, auth, candidate, message)
	}

	probe, err := s.probeRecoverableCodexQuota(ctx, auth, now)
	if err != nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "probe_error",
			AuthID:  auth.ID,
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: "quota probe failed",
			Error:   err.Error(),
		})
		return err
	}

	if probe.StatusCode == http.StatusUnauthorized && !refreshAttempted && authHasRefreshToken(auth) {
		refreshAttempted = true
		var terminal bool
		auth, terminal, err = s.refreshRecoverableCodexAuth(ctx, auth, candidate)
		if err != nil {
			return err
		}
		if terminal || auth == nil {
			return nil
		}
		probe, err = s.probeRecoverableCodexQuota(ctx, auth, time.Now())
		if err != nil {
			s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
				Event:   "probe_error",
				AuthID:  auth.ID,
				Path:    strings.TrimSpace(candidate.Path),
				Reason:  strings.TrimSpace(candidate.Reason),
				Message: "quota probe after refresh failed",
				Error:   err.Error(),
			})
			return err
		}
	}

	if probe.RetryAfter == nil && probe.StatusCode >= http.StatusOK && probe.StatusCode < http.StatusMultipleChoices {
		if err := s.unlockRecoveredAuth(ctx, auth, candidate, probe); err != nil {
			return err
		}
		return nil
	}

	if probe.StatusCode == http.StatusUnauthorized && !authHasRefreshToken(auth) {
		return s.persistTerminalRecoveryDisable(ctx, auth, candidate, "quota probe unauthorized and no refresh token")
	}
	if probe.StatusCode == http.StatusUnauthorized && refreshAttempted {
		return s.persistTerminalRecoveryDisable(ctx, auth, candidate, "quota probe unauthorized after refresh")
	}
	return s.persistDeferredRecoveryState(ctx, auth, candidate, probe)
}

func (s *Service) authMaintenanceRecoveryAuth(candidate authMaintenanceCandidate) *coreauth.Auth {
	if s == nil || s.coreManager == nil {
		return nil
	}
	for _, id := range candidate.IDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		auth, ok := s.coreManager.GetByID(id)
		if !ok || auth == nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
			continue
		}
		return auth.Clone()
	}
	return nil
}

func (s *Service) refreshRecoverableCodexAuth(ctx context.Context, auth *coreauth.Auth, candidate authMaintenanceCandidate) (*coreauth.Auth, bool, error) {
	if auth == nil {
		return nil, false, nil
	}
	exec, err := s.codexRecoveryExecutor(auth)
	if err != nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "refresh_error",
			AuthID:  auth.ID,
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: "resolve codex executor failed",
			Error:   err.Error(),
		})
		return nil, false, err
	}
	refreshed, err := exec.Refresh(ctx, auth.Clone())
	if err != nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "refresh_error",
			AuthID:  auth.ID,
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: "refresh failed",
			Error:   err.Error(),
		})
		return nil, false, err
	}
	if refreshed == nil {
		refreshed = auth.Clone()
	}
	refreshed.Disabled = auth.Disabled
	if refreshed.Status == "" {
		refreshed.Status = coreauth.StatusDisabled
	}
	if refreshed.Metadata == nil {
		refreshed.Metadata = make(map[string]any)
	}
	if coreauth.IsAuthMaintenanceAutoRecoverable(auth) {
		refreshed.Metadata = coreauth.MarkAuthMaintenanceAutoRecovery(refreshed.Metadata, recoveryReason(candidate, auth), time.Time{})
	}
	delete(refreshed.Metadata, coreauth.AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey)
	if refreshed.Disabled && strings.EqualFold(strings.TrimSpace(coreauth.AuthMaintenanceAutoRecoveryReason(refreshed)), "") {
		refreshed.Metadata = coreauth.MarkAuthMaintenanceAutoRecovery(refreshed.Metadata, recoveryReason(candidate, auth), time.Time{})
	}
	if reason, _ := refreshed.Metadata["refresh_disabled_reason"].(string); strings.EqualFold(strings.TrimSpace(reason), "refresh_token_reused") {
		if err := s.persistTerminalRecoveryDisable(ctx, refreshed, candidate, "refresh token reused"); err != nil {
			return nil, true, err
		}
		return nil, true, nil
	}
	if _, err := s.coreManager.Update(ctx, refreshed); err != nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "refresh_error",
			AuthID:  auth.ID,
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: "persist refreshed disabled auth failed",
			Error:   err.Error(),
		})
		return nil, false, err
	}
	s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
		Event:   "refresh",
		AuthID:  auth.ID,
		Path:    strings.TrimSpace(candidate.Path),
		Reason:  strings.TrimSpace(candidate.Reason),
		Message: "disabled auth refreshed",
	})
	return refreshed, false, nil
}

func (s *Service) unlockRecoveredAuth(ctx context.Context, auth *coreauth.Auth, candidate authMaintenanceCandidate, probe authMaintenanceQuotaProbeResult) error {
	if auth == nil {
		return nil
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	coreauth.ClearAuthMaintenanceAutoRecovery(auth.Metadata)
	delete(auth.Metadata, authMaintenancePendingDisableMetadataKey)
	delete(auth.Metadata, authMaintenancePendingDeleteMetadataKey)
	delete(auth.Metadata, authMaintenanceReasonMetadataKey)
	delete(auth.Metadata, "refresh_disabled_reason")
	auth.Disabled = false
	auth.Status = coreauth.StatusActive
	auth.StatusMessage = ""
	auth.Unavailable = false
	auth.LastError = nil
	auth.NextRetryAfter = time.Time{}
	auth.Quota = coreauth.QuotaState{}
	if _, err := s.coreManager.Update(ctx, auth); err != nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "unlock_error",
			AuthID:  auth.ID,
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: "persist unlock failed",
			Error:   err.Error(),
		})
		return err
	}
	s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
		Event:      "unlock",
		AuthID:     auth.ID,
		Path:       strings.TrimSpace(candidate.Path),
		Reason:     strings.TrimSpace(candidate.Reason),
		Message:    "disabled auth unlocked after quota recovery",
		StatusCode: probe.StatusCode,
		Window:     probe.Window,
	})
	return nil
}

func (s *Service) persistDeferredRecoveryState(ctx context.Context, auth *coreauth.Auth, candidate authMaintenanceCandidate, probe authMaintenanceQuotaProbeResult) error {
	if auth == nil {
		return nil
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	if probe.RetryAfter != nil {
		nextCheckAt := time.Now().Add(*probe.RetryAfter)
		coreauth.SetAuthMaintenanceAutoRecoveryNextCheckAt(auth.Metadata, nextCheckAt)
		auth.Quota.Exceeded = true
		auth.Quota.NextRecoverAt = nextCheckAt
		auth.Quota.Reason = probe.Window
	} else {
		coreauth.SetAuthMaintenanceAutoRecoveryNextCheckAt(auth.Metadata, time.Time{})
	}
	auth.Disabled = true
	auth.Status = coreauth.StatusDisabled
	if strings.TrimSpace(auth.StatusMessage) == "" {
		auth.StatusMessage = maintenanceStatusMessage(recoveryReason(candidate, auth), "disabled via auth maintenance")
	}
	if _, err := s.coreManager.Update(ctx, auth); err != nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "keep_disabled_error",
			AuthID:  auth.ID,
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: "persist deferred recovery state failed",
			Error:   err.Error(),
		})
		return err
	}
	record := authMaintenanceRecoveryLogRecord{
		Event:      "keep_disabled",
		AuthID:     auth.ID,
		Path:       strings.TrimSpace(candidate.Path),
		Reason:     strings.TrimSpace(candidate.Reason),
		Message:    "quota not recovered yet",
		StatusCode: probe.StatusCode,
		Window:     probe.Window,
	}
	if probe.RetryAfter != nil {
		record.RetryAfterSeconds = int64(probe.RetryAfter.Seconds())
	}
	s.recordAuthMaintenanceRecovery(record)
	return nil
}

func (s *Service) persistTerminalRecoveryDisable(ctx context.Context, auth *coreauth.Auth, candidate authMaintenanceCandidate, message string) error {
	if auth == nil {
		return nil
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	coreauth.ClearAuthMaintenanceAutoRecovery(auth.Metadata)
	auth.Disabled = true
	auth.Status = coreauth.StatusDisabled
	if strings.TrimSpace(message) != "" {
		auth.StatusMessage = message
	}
	if _, err := s.coreManager.Update(ctx, auth); err != nil {
		s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
			Event:   "terminal_disable_error",
			AuthID:  auth.ID,
			Path:    strings.TrimSpace(candidate.Path),
			Reason:  strings.TrimSpace(candidate.Reason),
			Message: message,
			Error:   err.Error(),
		})
		return err
	}
	s.recordAuthMaintenanceRecovery(authMaintenanceRecoveryLogRecord{
		Event:   "terminal_disable",
		AuthID:  auth.ID,
		Path:    strings.TrimSpace(candidate.Path),
		Reason:  strings.TrimSpace(candidate.Reason),
		Message: message,
	})
	return nil
}

func (s *Service) probeRecoverableCodexQuota(ctx context.Context, auth *coreauth.Auth, now time.Time) (authMaintenanceQuotaProbeResult, error) {
	if auth == nil {
		return authMaintenanceQuotaProbeResult{}, nil
	}
	exec, err := s.codexRecoveryExecutor(auth)
	if err != nil {
		return authMaintenanceQuotaProbeResult{}, err
	}
	apiKey := authMaintenanceTokenValue(auth)
	accountID := authMaintenanceAccountID(auth)
	if apiKey == "" || accountID == "" {
		return authMaintenanceQuotaProbeResult{}, fmt.Errorf("missing codex probe credentials")
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(requestCtx, 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, authMaintenanceCodexUsageURL(authMaintenanceBaseURL(auth)), nil)
	if err != nil {
		return authMaintenanceQuotaProbeResult{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", authMaintenanceCodexUsageProbeUserAgent)
	req.Header.Set("Chatgpt-Account-Id", accountID)

	resp, err := exec.HttpRequest(probeCtx, auth, req)
	if err != nil {
		return authMaintenanceQuotaProbeResult{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.WithError(closeErr).Warn("auth maintenance recovery: close quota probe response body failed")
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return authMaintenanceQuotaProbeResult{}, err
	}
	decision := helps.ParseCodexQuotaRetryDecision(body, now)
	record := authMaintenanceRecoveryLogRecord{
		Event:      "probe",
		AuthID:     auth.ID,
		Path:       strings.TrimSpace(authMaintenanceAuthPath(auth)),
		StatusCode: resp.StatusCode,
		Window:     decision.Window,
	}
	if decision.RetryAfter != nil {
		record.RetryAfterSeconds = int64(decision.RetryAfter.Seconds())
	}
	s.recordAuthMaintenanceRecovery(record)
	return authMaintenanceQuotaProbeResult{
		StatusCode: resp.StatusCode,
		RetryAfter: decision.RetryAfter,
		Window:     decision.Window,
	}, nil
}

func (s *Service) codexRecoveryExecutor(auth *coreauth.Auth) (coreauth.ProviderExecutor, error) {
	if s == nil || s.coreManager == nil {
		return nil, fmt.Errorf("core auth manager is unavailable")
	}
	if exec, ok := s.coreManager.Executor("codex"); ok && exec != nil {
		return exec, nil
	}
	s.ensureExecutorsForAuth(auth)
	exec, ok := s.coreManager.Executor("codex")
	if !ok || exec == nil {
		return nil, fmt.Errorf("codex executor is unavailable")
	}
	return exec, nil
}

func shouldRefreshRecoverableCodexAuth(auth *coreauth.Auth, now time.Time) bool {
	if auth == nil || !authHasRefreshToken(auth) {
		return false
	}
	if authMaintenanceTokenValue(auth) == "" || authMaintenanceAccountID(auth) == "" {
		return true
	}
	expiry, hasExpiry := auth.ExpirationTime()
	if !hasExpiry || expiry.IsZero() {
		return false
	}
	if !expiry.After(now) {
		return true
	}
	if lead := coreauth.ProviderRefreshLead(auth.Provider, auth.Runtime); lead != nil && *lead > 0 {
		return time.Until(expiry) <= *lead
	}
	return false
}

func authHasRefreshToken(auth *coreauth.Auth) bool {
	if auth == nil || auth.Metadata == nil {
		return false
	}
	value, _ := auth.Metadata["refresh_token"].(string)
	return strings.TrimSpace(value) != ""
}

func authMaintenanceTokenValue(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if value := strings.TrimSpace(auth.Attributes["api_key"]); value != "" {
			return value
		}
	}
	if auth.Metadata == nil {
		return ""
	}
	for _, key := range [...]string{"access_token", "token", "api_key"} {
		if value, ok := auth.Metadata[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func authMaintenanceAccountID(auth *coreauth.Auth) string {
	if auth == nil || auth.Metadata == nil {
		return ""
	}
	value, _ := auth.Metadata["account_id"].(string)
	return strings.TrimSpace(value)
}

func authMaintenanceBaseURL(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if value := strings.TrimSpace(auth.Attributes["base_url"]); value != "" {
			return value
		}
	}
	if auth.Metadata == nil {
		return ""
	}
	value, _ := auth.Metadata["base_url"].(string)
	return strings.TrimSpace(value)
}

func authMaintenanceCodexUsageURL(baseURL string) string {
	const fallback = "https://chatgpt.com/backend-api/wham/usage"
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return fallback
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fallback
	}
	parsed.Path = "/backend-api/wham/usage"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func authMaintenanceAuthPath(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if path := strings.TrimSpace(auth.Attributes["path"]); path != "" {
			return path
		}
	}
	return strings.TrimSpace(auth.FileName)
}

func firstMaintenanceAuthID(ids []string) string {
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			return id
		}
	}
	return ""
}

func recoveryReason(candidate authMaintenanceCandidate, auth *coreauth.Auth) string {
	if reason := strings.TrimSpace(candidate.Reason); reason != "" {
		return reason
	}
	if reason := coreauth.AuthMaintenanceAutoRecoveryReason(auth); reason != "" {
		return reason
	}
	if auth != nil {
		return strings.TrimSpace(auth.StatusMessage)
	}
	return ""
}
