package cliproxy

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

	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	defaultMaintenanceScanIntervalSeconds   = 30
	defaultMaintenanceDeleteIntervalSeconds = 5
	defaultMaintenanceQuotaStrikeThreshold  = 6
	authMaintenanceMinSpacing               = 250 * time.Millisecond
	authMaintenanceSuppressWindow           = 2 * time.Second
)

const (
	authRuntimeDefaultUnauthorizedDeleteThreshold = 3
	authRuntimeDefaultUnauthorizedDeleteWindow    = 10 * time.Minute
)

const (
	authMaintenancePendingDeleteMetadataKey  = "auth_maintenance_pending_delete"
	authMaintenancePendingDisableMetadataKey = "auth_maintenance_pending_disable"
	authMaintenanceReasonMetadataKey         = "auth_maintenance_reason"
)

type authFilesStorePersister interface {
	PersistAuthFiles(ctx context.Context, message string, paths ...string) error
}

type authMaintenanceAction string

const (
	authMaintenanceActionDelete  authMaintenanceAction = "delete"
	authMaintenanceActionDisable authMaintenanceAction = "disable"
)

type authMaintenanceCandidate struct {
	Key    string
	Path   string
	IDs    []string
	Reason string
	Action authMaintenanceAction
}

type authMaintenanceHook struct {
	service *Service
}

func (h authMaintenanceHook) OnAuthRegistered(context.Context, *coreauth.Auth) {}

func (h authMaintenanceHook) OnAuthUpdated(context.Context, *coreauth.Auth) {}

func (h authMaintenanceHook) OnResult(ctx context.Context, result coreauth.Result) {
	if h.service != nil {
		h.service.handleAuthMaintenanceResult(ctx, result)
	}
}

func (s *Service) startAuthMaintenance(parent context.Context) {
	if s == nil || s.maintenanceCancel != nil {
		return
	}
	s.installAuthMaintenanceHook()
	s.ensureAuthMaintenanceQueue()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	s.maintenanceCancel = cancel
	go s.runAuthMaintenance(ctx)
}

func (s *Service) installAuthMaintenanceHook() {
	if s == nil || s.coreManager == nil {
		return
	}
	s.maintenanceHookOnce.Do(func() {
		s.coreManager.SetHook(coreauth.CombineHooks(
			s.coreManager.Hook(),
			authMaintenanceHook{service: s},
		))
	})
}

func (s *Service) runAuthMaintenance(ctx context.Context) {
	wake := s.authMaintenanceWakeChan()
	nextScan := time.Time{}
	nextAction := time.Time{}
	lastAction := time.Time{}
	var timer *time.Timer

	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
	}
	defer stopTimer()

	for {
		now := time.Now()
		cfg, authDir := s.snapshotAuthMaintenanceConfig()

		if !cfg.Enable {
			s.resetAuthMaintenanceQueue()
			nextAction = time.Time{}
			nextScan = now.Add(time.Duration(cfg.ScanIntervalSeconds) * time.Second)
		} else {
			if nextScan.IsZero() || !now.Before(nextScan) {
				candidates := s.scanAuthMaintenanceCandidates(cfg, authDir)
				for _, candidate := range candidates {
					s.enqueueAuthMaintenanceCandidate(candidate)
				}
				nextScan = now.Add(time.Duration(cfg.ScanIntervalSeconds) * time.Second)
			}

			if depth := s.authMaintenanceOutstandingLen(); depth > 0 {
				spacing := authMaintenanceActionSpacing(cfg, depth)
				if nextAction.IsZero() {
					nextAction = now
					minNext := lastAction.Add(spacing)
					if !lastAction.IsZero() && now.Before(minNext) {
						nextAction = minNext
					}
				}
				if !nextAction.IsZero() && !now.Before(nextAction) {
					candidate, remaining, ok := s.popAuthMaintenanceCandidate()
					if ok {
						err := s.applyAuthMaintenanceCandidate(ctx, candidate)
						s.finishAuthMaintenanceCandidate(candidate)
						if err != nil {
							log.WithError(err).Warnf("auth maintenance action failed for %s (%s)", candidate.Path, candidate.Action)
							if ctx.Err() == nil {
								s.enqueueAuthMaintenanceCandidate(candidate)
							}
						}
						lastAction = time.Now()
						if outstanding := remaining + s.authMaintenanceInFlightLen(); outstanding > 0 {
							nextAction = lastAction.Add(authMaintenanceActionSpacing(cfg, outstanding))
						} else {
							nextAction = time.Time{}
						}
						continue
					}
					nextAction = time.Time{}
				}
			} else {
				nextAction = time.Time{}
			}
		}

		waitUntil := nextScan
		if !nextAction.IsZero() && (waitUntil.IsZero() || nextAction.Before(waitUntil)) {
			waitUntil = nextAction
		}
		if waitUntil.IsZero() {
			waitUntil = now.Add(time.Duration(cfg.ScanIntervalSeconds) * time.Second)
		}
		wait := time.Until(waitUntil)
		if wait < 0 {
			wait = 0
		}
		if timer == nil {
			timer = time.NewTimer(wait)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(wait)
		}

		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-timer.C:
		}
	}
}

func authMaintenanceActionSpacing(cfg config.AuthMaintenanceConfig, backlog int) time.Duration {
	interval := time.Duration(cfg.DeleteIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Duration(defaultMaintenanceDeleteIntervalSeconds) * time.Second
	}
	divisor := 1
	switch {
	case backlog >= 64:
		divisor = 4
	case backlog >= 16:
		divisor = 2
	}
	spacing := interval / time.Duration(divisor)
	if spacing < authMaintenanceMinSpacing {
		return authMaintenanceMinSpacing
	}
	return spacing
}

func (s *Service) ensureAuthMaintenanceQueue() {
	if s == nil {
		return
	}
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	if s.maintenancePending == nil {
		s.maintenancePending = make(map[string]struct{})
	}
	if s.maintenanceInFlight == nil {
		s.maintenanceInFlight = make(map[string]struct{})
	}
	if s.maintenanceAuthIDsByPath == nil {
		s.maintenanceAuthIDsByPath = make(map[string]map[string]struct{})
	}
	if s.maintenanceAuthPathByID == nil {
		s.maintenanceAuthPathByID = make(map[string]string)
	}
	if s.maintenanceWake == nil {
		s.maintenanceWake = make(chan struct{}, 1)
	}
}

func (s *Service) authMaintenanceWakeChan() <-chan struct{} {
	if s == nil {
		return nil
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	return s.maintenanceWake
}

func (s *Service) enqueueAuthMaintenanceCandidate(candidate authMaintenanceCandidate) bool {
	if s == nil {
		return false
	}
	candidate.Key = strings.TrimSpace(candidate.Key)
	candidate.Path = strings.TrimSpace(candidate.Path)
	candidate.Reason = strings.TrimSpace(candidate.Reason)
	if candidate.Key == "" || candidate.Path == "" || candidate.Reason == "" || len(candidate.IDs) == 0 {
		return false
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	if _, ok := s.maintenancePending[candidate.Key]; ok {
		s.maintenanceMu.Unlock()
		return false
	}
	if _, ok := s.maintenanceInFlight[candidate.Key]; ok {
		s.maintenanceMu.Unlock()
		return false
	}
	s.maintenancePending[candidate.Key] = struct{}{}
	s.maintenanceQueue = append(s.maintenanceQueue, candidate)
	wake := s.maintenanceWake
	s.maintenanceMu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	return true
}

func (s *Service) popAuthMaintenanceCandidate() (authMaintenanceCandidate, int, bool) {
	if s == nil {
		return authMaintenanceCandidate{}, 0, false
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	if len(s.maintenanceQueue) == 0 {
		return authMaintenanceCandidate{}, 0, false
	}
	candidate := s.maintenanceQueue[0]
	s.maintenanceQueue = s.maintenanceQueue[1:]
	delete(s.maintenancePending, candidate.Key)
	s.maintenanceInFlight[candidate.Key] = struct{}{}
	return candidate, len(s.maintenanceQueue), true
}

func (s *Service) finishAuthMaintenanceCandidate(candidate authMaintenanceCandidate) {
	if s == nil {
		return
	}
	key := strings.TrimSpace(candidate.Key)
	if key == "" {
		return
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	delete(s.maintenanceInFlight, key)
	s.maintenanceMu.Unlock()
}

func (s *Service) resetAuthMaintenanceQueue() {
	if s == nil {
		return
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	s.maintenanceQueue = s.maintenanceQueue[:0]
	clear(s.maintenancePending)
	clear(s.maintenanceInFlight)
	clear(s.maintenanceAuthIDsByPath)
	clear(s.maintenanceAuthPathByID)
}

func (s *Service) authMaintenanceOutstandingLen() int {
	if s == nil {
		return 0
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	return len(s.maintenanceQueue) + len(s.maintenanceInFlight)
}

func (s *Service) authMaintenanceInFlightLen() int {
	if s == nil {
		return 0
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	return len(s.maintenanceInFlight)
}

func (s *Service) authMaintenanceIsTracked(key string) bool {
	if s == nil {
		return false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	if _, ok := s.maintenancePending[key]; ok {
		return true
	}
	_, ok := s.maintenanceInFlight[key]
	return ok
}

func (s *Service) snapshotAuthMaintenanceConfig() (config.AuthMaintenanceConfig, string) {
	defaultCfg := config.AuthMaintenanceConfig{
		Enable:                        true,
		ScanIntervalSeconds:           defaultMaintenanceScanIntervalSeconds,
		DeleteIntervalSeconds:         defaultMaintenanceDeleteIntervalSeconds,
		QuotaStrikeThreshold:          defaultMaintenanceQuotaStrikeThreshold,
		DisableCodexUsageLimitReached: true,
	}
	if s == nil {
		return defaultCfg, ""
	}
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()
	if cfg == nil {
		return defaultCfg, ""
	}
	maintenance := cfg.AuthMaintenance
	if isZeroAuthMaintenanceConfig(maintenance) {
		maintenance = defaultCfg
	}
	if maintenance.ScanIntervalSeconds <= 0 {
		maintenance.ScanIntervalSeconds = defaultMaintenanceScanIntervalSeconds
	}
	if maintenance.DeleteIntervalSeconds <= 0 {
		maintenance.DeleteIntervalSeconds = defaultMaintenanceDeleteIntervalSeconds
	}
	if maintenance.QuotaStrikeThreshold <= 0 {
		maintenance.QuotaStrikeThreshold = defaultMaintenanceQuotaStrikeThreshold
	}
	if !maintenance.DisableCodexUsageLimitReached && isZeroAuthMaintenanceConfig(cfg.AuthMaintenance) {
		maintenance.DisableCodexUsageLimitReached = true
	}
	return maintenance, strings.TrimSpace(cfg.AuthDir)
}

func isZeroAuthMaintenanceConfig(cfg config.AuthMaintenanceConfig) bool {
	return !cfg.Enable &&
		cfg.ScanIntervalSeconds == 0 &&
		cfg.DeleteIntervalSeconds == 0 &&
		len(cfg.DeleteStatusCodes) == 0 &&
		len(cfg.DisableStatusCodes) == 0 &&
		!cfg.DeleteQuotaExceeded &&
		cfg.QuotaStrikeThreshold == 0 &&
		!cfg.DisableCodexUsageLimitReached &&
		cfg.CodexMaxRequestCount == 0 &&
		cfg.CodexQuotaCheckRequestInterval == 0
}

func (s *Service) authMaintenanceUnauthorizedThreshold() int {
	if s == nil || s.cfg == nil || s.cfg.AuthRuntime.UnauthorizedDeleteThreshold <= 0 {
		return authRuntimeDefaultUnauthorizedDeleteThreshold
	}
	return s.cfg.AuthRuntime.UnauthorizedDeleteThreshold
}

func (s *Service) authMaintenanceUnauthorizedWindow() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.AuthRuntime.UnauthorizedDeleteWindowSeconds <= 0 {
		return authRuntimeDefaultUnauthorizedDeleteWindow
	}
	return time.Duration(s.cfg.AuthRuntime.UnauthorizedDeleteWindowSeconds) * time.Second
}

func (s *Service) handleAuthMaintenanceResult(ctx context.Context, result coreauth.Result) {
	if s == nil || s.coreManager == nil || result.Success {
		return
	}
	cfg, authDir := s.snapshotAuthMaintenanceConfig()
	if !cfg.Enable {
		return
	}
	authID := strings.TrimSpace(result.AuthID)
	if authID == "" {
		return
	}
	auth, ok := s.coreManager.GetByID(authID)
	if !ok || auth == nil || !shouldTrackServiceAuthMaintenance(auth) {
		return
	}

	if shouldDisableCodexUsageLimitReachedForMaintenance(result, cfg) {
		candidate, ok := s.authMaintenanceCandidateForAuth(auth, authDir, "disabled after usage_limit_reached", authMaintenanceActionDisable)
		if !ok {
			return
		}
		if authMaintenancePendingAction(auth, authMaintenanceActionDisable) && s.authMaintenanceIsTracked(candidate.Key) {
			return
		}
		s.markAuthMaintenanceCandidate(ctx, candidate, authID)
		s.enqueueAuthMaintenanceCandidate(candidate)
		return
	}

	if reason, ok := authEligibleForMaintenanceDisable(auth, &result, cfg); ok {
		candidate, ok := s.authMaintenanceCandidateForAuth(auth, authDir, reason, authMaintenanceActionDisable)
		if !ok {
			return
		}
		if authMaintenancePendingAction(auth, authMaintenanceActionDisable) && s.authMaintenanceIsTracked(candidate.Key) {
			return
		}
		s.markAuthMaintenanceCandidate(ctx, candidate, authID)
		s.enqueueAuthMaintenanceCandidate(candidate)
		return
	}

	if unauthorizedCleanupStatusCode(result.Error) == http.StatusUnauthorized {
		now := time.Now()
		count := coreauth.RecordUnauthorizedFailure(auth, now, s.authMaintenanceUnauthorizedWindow())
		if count < s.authMaintenanceUnauthorizedThreshold() {
			if _, err := s.coreManager.Update(ctx, auth); err != nil {
				log.WithError(err).Warnf("auth maintenance: persist unauthorized history failed for %s", auth.ID)
			}
			return
		}
		candidate, ok := s.authMaintenanceCandidateForAuth(auth, authDir, "disabled after unauthorized threshold", authMaintenanceActionDisable)
		if !ok {
			return
		}
		if authMaintenancePendingAction(auth, authMaintenanceActionDisable) && s.authMaintenanceIsTracked(candidate.Key) {
			return
		}
		s.markAuthMaintenanceCandidate(ctx, candidate, authID)
		s.enqueueAuthMaintenanceCandidate(candidate)
		return
	}

	if reason, ok := authEligibleForMaintenanceDelete(auth, &result, cfg); ok {
		candidate, ok := s.authMaintenanceCandidateForAuth(auth, authDir, reason, authMaintenanceActionDelete)
		if !ok {
			return
		}
		if authMaintenancePendingAction(auth, authMaintenanceActionDelete) && s.authMaintenanceIsTracked(candidate.Key) {
			return
		}
		s.markAuthMaintenanceCandidate(ctx, candidate, authID)
		s.enqueueAuthMaintenanceCandidate(candidate)
	}
}

func (s *Service) markAuthMaintenanceCandidate(ctx context.Context, candidate authMaintenanceCandidate, persistID string) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	persistID = strings.TrimSpace(persistID)
	persisted := false
	for _, id := range candidate.IDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		updateCtx := ctx
		if persisted || (persistID != "" && persistID != id) {
			updateCtx = coreauth.WithSkipPersist(updateCtx)
		} else {
			persisted = true
		}
		switch candidate.Action {
		case authMaintenanceActionDisable:
			s.applyCoreAuthDisableWithReason(updateCtx, id, candidate.Reason, true)
		default:
			s.applyCoreAuthDisableWithReason(updateCtx, id, candidate.Reason, true)
		}
	}
}

func (s *Service) authMaintenanceCandidateForAuth(auth *coreauth.Auth, authDir, reason string, action authMaintenanceAction) (authMaintenanceCandidate, bool) {
	if s == nil || auth == nil {
		return authMaintenanceCandidate{}, false
	}
	path := resolveAuthFilePath(auth, authDir)
	if path == "" {
		return authMaintenanceCandidate{}, false
	}
	candidate := authMaintenanceCandidate{
		Key:    authMaintenanceCandidateKey(action, path),
		Path:   path,
		Reason: strings.TrimSpace(reason),
		Action: action,
	}
	candidate.IDs = append(candidate.IDs, s.authMaintenanceIDsForPath(path, authDir)...)
	if len(candidate.IDs) == 0 {
		id := strings.TrimSpace(auth.ID)
		if id == "" {
			return authMaintenanceCandidate{}, false
		}
		candidate.IDs = []string{id}
	}
	if candidate.Reason == "" {
		candidate.Reason = "auth maintenance"
	}
	return candidate, true
}

func (s *Service) scanAuthMaintenanceCandidates(cfg config.AuthMaintenanceConfig, authDir string) []authMaintenanceCandidate {
	if s == nil || s.coreManager == nil || !cfg.Enable {
		return nil
	}
	snapshot := s.coreManager.List()
	grouped := make(map[string]authMaintenanceCandidate)
	idsByPath := make(map[string]map[string]struct{})
	pathByID := make(map[string]string)

	for _, auth := range snapshot {
		if auth == nil || !shouldTrackServiceAuthMaintenance(auth) {
			continue
		}
		path := resolveAuthFilePath(auth, authDir)
		if path == "" {
			continue
		}
		id := strings.TrimSpace(auth.ID)
		if id != "" {
			if idsByPath[path] == nil {
				idsByPath[path] = make(map[string]struct{})
			}
			idsByPath[path][id] = struct{}{}
			pathByID[id] = path
		}

		action := authMaintenancePendingCandidateAction(auth)
		reason, hasReason := authMaintenancePendingReason(auth)
		if action == authMaintenanceActionDelete {
			action = authMaintenanceActionDisable
		}
		if protectedReason, ok := authProtectedMaintenanceDisableReason(auth, nil); ok {
			action = authMaintenanceActionDisable
			reason = protectedReason
			hasReason = true
		} else if action == "" {
			if shouldDisableAuthAfterUsageLimitFromState(auth, cfg) {
				action = authMaintenanceActionDisable
				reason = "disabled after usage_limit_reached"
				hasReason = true
			} else if disableReason, ok := authEligibleForMaintenanceDisable(auth, nil, cfg); ok {
				action = authMaintenanceActionDisable
				reason = disableReason
				hasReason = true
			} else if deleteReason, ok := authEligibleForMaintenanceDelete(auth, nil, cfg); ok {
				action = authMaintenanceActionDelete
				reason = deleteReason
				hasReason = true
			}
		}
		if action == "" || !hasReason {
			continue
		}
		key := authMaintenanceCandidateKey(action, path)
		group := grouped[key]
		if group.Key == "" {
			group = authMaintenanceCandidate{
				Key:    key,
				Path:   path,
				Action: action,
				Reason: reason,
			}
		}
		if id != "" {
			group.IDs = append(group.IDs, id)
		}
		grouped[key] = group
	}

	s.replaceAuthMaintenanceIndex(idsByPath, pathByID)

	candidates := make([]authMaintenanceCandidate, 0, len(grouped))
	for _, candidate := range grouped {
		if candidate.Key == "" || candidate.Path == "" || len(candidate.IDs) == 0 {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func (s *Service) applyAuthMaintenanceCandidate(ctx context.Context, candidate authMaintenanceCandidate) error {
	switch candidate.Action {
	case authMaintenanceActionDisable:
		return s.disableAuthMaintenanceCandidate(ctx, candidate)
	default:
		return s.disableAuthMaintenanceCandidate(ctx, candidate)
	}
}

func (s *Service) disableAuthMaintenanceCandidate(ctx context.Context, candidate authMaintenanceCandidate) error {
	if s == nil {
		return nil
	}
	ctx = coreauth.WithSkipPersist(ctx)
	path := strings.TrimSpace(candidate.Path)
	if path != "" {
		if err := writeDisabledAuthFile(path); err != nil {
			return err
		}
	}
	for _, id := range candidate.IDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			s.applyCoreAuthDisableWithReason(ctx, trimmed, maintenanceStatusMessage(candidate.Reason, "disabled via auth maintenance"), false)
		}
	}
	return nil
}

func writeDisabledAuthFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read auth file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("unmarshal auth file: %w", err)
	}
	metadata["disabled"] = true
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal auth file: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write auth file: %w", err)
	}
	return nil
}

func (s *Service) deleteAuthMaintenanceCandidate(ctx context.Context, candidate authMaintenanceCandidate) error {
	if s == nil {
		return nil
	}
	ctx = coreauth.WithSkipPersist(ctx)
	path := strings.TrimSpace(candidate.Path)
	var cleanupErr error
	if path != "" {
		authDir := ""
		s.cfgMu.RLock()
		if s.cfg != nil {
			authDir = strings.TrimSpace(s.cfg.AuthDir)
		}
		s.cfgMu.RUnlock()
		if s.watcher != nil {
			s.watcher.SuppressAuthPath(path, authMaintenanceSuppressWindow)
		}
		if authDir == "" || sdkAuth.IsTrashPath(authDir, path) {
			if err := s.deleteAuthTokenRecord(ctx, path); err != nil {
				cleanupErr = fmt.Errorf("delete auth token record: %w", err)
			}
		} else {
			logicalRel, errRel := sdkAuth.RelativePath(authDir, path)
			if errRel != nil || sdkAuth.IsTrashRelative(logicalRel) {
				logicalRel = filepath.Base(path)
			}
			trashPath, _, errTrash := sdkAuth.TrashPathForRelative(authDir, logicalRel)
			if errTrash != nil {
				return fmt.Errorf("resolve auth recycle bin path: %w", errTrash)
			}
			if err := os.MkdirAll(filepath.Dir(trashPath), 0o700); err != nil {
				return fmt.Errorf("prepare auth recycle bin: %w", err)
			}
			if err := os.Remove(trashPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("clear existing recycle bin entry: %w", err)
			}
			if err := os.Rename(path, trashPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if errDelete := s.deleteAuthTokenRecord(ctx, path); errDelete != nil {
						cleanupErr = fmt.Errorf("delete auth token record: %w", errDelete)
					}
				} else {
					return fmt.Errorf("move auth file to recycle bin: %w", err)
				}
			} else {
				if err := s.persistAuthTrashMove(ctx, path, trashPath); err != nil {
					if rollbackErr := os.Rename(trashPath, path); rollbackErr != nil {
						log.WithError(rollbackErr).Warnf("failed to rollback auth recycle bin move for %s", logicalRel)
					}
					return err
				}
				sdkAuth.PruneEmptyParentDirs(authDir, filepath.Dir(path))
			}
		}
	}
	for _, id := range candidate.IDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			s.emitAuthUpdate(ctx, watcher.AuthUpdate{Action: watcher.AuthUpdateActionDelete, ID: trimmed})
		}
	}
	return cleanupErr
}

func (s *Service) persistAuthTrashMove(ctx context.Context, sourcePath, trashPath string) error {
	store := sdkAuth.GetTokenStore()
	if store == nil {
		return fmt.Errorf("token store unavailable")
	}
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()
	if cfg != nil {
		if dirSetter, ok := store.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(cfg.AuthDir)
		}
	}
	if persister, ok := store.(authFilesStorePersister); ok {
		if err := persister.PersistAuthFiles(ctx, fmt.Sprintf("Trash auth %s", filepath.Base(sourcePath)), sourcePath, trashPath); err != nil {
			return fmt.Errorf("persist auth recycle bin move: %w", err)
		}
	}
	return nil
}

func (s *Service) deleteAuthTokenRecord(ctx context.Context, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	store := sdkAuth.GetTokenStore()
	if store == nil {
		return fmt.Errorf("token store unavailable")
	}
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()
	if cfg != nil {
		if dirSetter, ok := store.(interface{ SetBaseDir(string) }); ok {
			dirSetter.SetBaseDir(cfg.AuthDir)
		}
	}
	return store.Delete(ctx, path)
}

func (s *Service) applyCoreAuthDisableWithReason(ctx context.Context, id, reason string, pendingDisable bool) {
	if s == nil || strings.TrimSpace(id) == "" || s.coreManager == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	GlobalModelRegistry().UnregisterClient(id)
	existing, ok := s.coreManager.GetByID(id)
	if !ok || existing == nil {
		s.removeAuthMaintenanceAuth(id)
		return
	}
	if existing.Metadata == nil {
		existing.Metadata = make(map[string]any)
	}
	existing.Disabled = true
	existing.Status = coreauth.StatusDisabled
	existing.StatusMessage = maintenanceStatusMessage(reason, "disabled after usage_limit_reached")
	if pendingDisable {
		existing.Metadata[authMaintenancePendingDisableMetadataKey] = true
	} else {
		delete(existing.Metadata, authMaintenancePendingDisableMetadataKey)
	}
	delete(existing.Metadata, authMaintenancePendingDeleteMetadataKey)
	if reason = strings.TrimSpace(reason); reason != "" {
		existing.Metadata[authMaintenanceReasonMetadataKey] = reason
	} else {
		delete(existing.Metadata, authMaintenanceReasonMetadataKey)
	}
	if _, err := s.coreManager.Update(ctx, existing); err != nil {
		log.Errorf("failed to disable auth %s: %v", id, err)
	}
	if pendingDisable {
		s.indexAuthMaintenanceAuth(existing)
	} else {
		s.removeAuthMaintenanceAuth(id)
	}
	if strings.EqualFold(strings.TrimSpace(existing.Provider), "codex") {
		s.ensureExecutorsForAuth(existing)
	}
}

func (s *Service) applyCoreAuthRemovalWithReason(ctx context.Context, id, reason string, pendingDelete bool) {
	if s == nil || strings.TrimSpace(id) == "" || s.coreManager == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	GlobalModelRegistry().UnregisterClient(id)
	existing, ok := s.coreManager.GetByID(id)
	if !ok || existing == nil {
		s.removeAuthMaintenanceAuth(id)
		return
	}
	if existing.Metadata == nil {
		existing.Metadata = make(map[string]any)
	}
	existing.Disabled = true
	existing.Status = coreauth.StatusDisabled
	existing.StatusMessage = maintenanceStatusMessage(reason, "removed via auth maintenance")
	if pendingDelete {
		existing.Metadata[authMaintenancePendingDeleteMetadataKey] = true
	} else {
		delete(existing.Metadata, authMaintenancePendingDeleteMetadataKey)
	}
	delete(existing.Metadata, authMaintenancePendingDisableMetadataKey)
	if reason = strings.TrimSpace(reason); reason != "" {
		existing.Metadata[authMaintenanceReasonMetadataKey] = reason
	} else {
		delete(existing.Metadata, authMaintenanceReasonMetadataKey)
	}
	if _, err := s.coreManager.Update(ctx, existing); err != nil {
		log.Errorf("failed to disable auth %s: %v", id, err)
	}
	if pendingDelete {
		s.indexAuthMaintenanceAuth(existing)
	} else {
		s.removeAuthMaintenanceAuth(id)
	}
	if strings.EqualFold(strings.TrimSpace(existing.Provider), "codex") {
		s.ensureExecutorsForAuth(existing)
	}
}

func maintenanceStatusMessage(reason, fallback string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	return fallback
}

func (s *Service) indexAuthMaintenanceAuth(auth *coreauth.Auth) {
	if s == nil {
		return
	}
	_, authDir := s.snapshotAuthMaintenanceConfig()
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	s.indexAuthMaintenanceAuthLocked(auth, authDir)
}

func (s *Service) indexAuthMaintenanceAuthLocked(auth *coreauth.Auth, authDir string) {
	if auth == nil {
		return
	}
	id := strings.TrimSpace(auth.ID)
	if id == "" {
		return
	}
	s.removeAuthMaintenanceAuthLocked(id)
	path := resolveAuthFilePath(auth, authDir)
	if path == "" {
		return
	}
	if s.maintenanceAuthIDsByPath == nil {
		s.maintenanceAuthIDsByPath = make(map[string]map[string]struct{})
	}
	if s.maintenanceAuthPathByID == nil {
		s.maintenanceAuthPathByID = make(map[string]string)
	}
	if s.maintenanceAuthIDsByPath[path] == nil {
		s.maintenanceAuthIDsByPath[path] = make(map[string]struct{})
	}
	s.maintenanceAuthIDsByPath[path][id] = struct{}{}
	s.maintenanceAuthPathByID[id] = path
}

func (s *Service) removeAuthMaintenanceAuth(id string) {
	if s == nil {
		return
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	s.removeAuthMaintenanceAuthLocked(id)
}

func (s *Service) removeAuthMaintenanceAuthLocked(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	path := strings.TrimSpace(s.maintenanceAuthPathByID[id])
	if path == "" {
		return
	}
	delete(s.maintenanceAuthPathByID, id)
	if ids := s.maintenanceAuthIDsByPath[path]; ids != nil {
		delete(ids, id)
		if len(ids) == 0 {
			delete(s.maintenanceAuthIDsByPath, path)
		}
	}
}

func (s *Service) authMaintenanceIDsForPath(path, authDir string) []string {
	if s == nil {
		return nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	if ids := s.maintenanceAuthIDsByPath[path]; len(ids) > 0 {
		out := make([]string, 0, len(ids))
		for id := range ids {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		s.maintenanceMu.Unlock()
		return out
	}
	s.maintenanceMu.Unlock()

	if s.coreManager == nil {
		return nil
	}
	s.rebuildAuthMaintenanceIndex(s.coreManager.List(), authDir)

	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	ids := s.maintenanceAuthIDsByPath[path]
	out := make([]string, 0, len(ids))
	for id := range ids {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (s *Service) rebuildAuthMaintenanceIndex(snapshot []*coreauth.Auth, authDir string) {
	if s == nil {
		return
	}
	idsByPath := make(map[string]map[string]struct{})
	pathByID := make(map[string]string)
	for _, auth := range snapshot {
		if auth == nil || !shouldTrackServiceAuthMaintenance(auth) {
			continue
		}
		id := strings.TrimSpace(auth.ID)
		if id == "" {
			continue
		}
		path := resolveAuthFilePath(auth, authDir)
		if path == "" {
			continue
		}
		if idsByPath[path] == nil {
			idsByPath[path] = make(map[string]struct{})
		}
		idsByPath[path][id] = struct{}{}
		pathByID[id] = path
	}
	s.replaceAuthMaintenanceIndex(idsByPath, pathByID)
}

func (s *Service) replaceAuthMaintenanceIndex(idsByPath map[string]map[string]struct{}, pathByID map[string]string) {
	if s == nil {
		return
	}
	s.ensureAuthMaintenanceQueue()
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	if idsByPath == nil {
		idsByPath = make(map[string]map[string]struct{})
	}
	if pathByID == nil {
		pathByID = make(map[string]string)
	}
	s.maintenanceAuthIDsByPath = idsByPath
	s.maintenanceAuthPathByID = pathByID
}

func shouldTrackServiceAuthMaintenance(auth *coreauth.Auth) bool {
	if auth == nil {
		return false
	}
	if auth.Attributes != nil {
		if strings.EqualFold(strings.TrimSpace(auth.Attributes["runtime_only"]), "true") {
			return false
		}
		if strings.TrimSpace(auth.Attributes["gemini_virtual_project"]) != "" {
			return false
		}
	}
	return true
}

func shouldDisableCodexUsageLimitReachedForMaintenance(result coreauth.Result, cfg config.AuthMaintenanceConfig) bool {
	if !cfg.DisableCodexUsageLimitReached || result.Success || result.Error == nil {
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

func shouldDisableAuthAfterUsageLimitFromState(auth *coreauth.Auth, cfg config.AuthMaintenanceConfig) bool {
	if auth == nil || !cfg.DisableCodexUsageLimitReached {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		return false
	}
	if auth.LastError != nil && auth.LastError.HTTPStatus == http.StatusTooManyRequests && containsUsageLimitReached(auth.LastError.Message) {
		return true
	}
	return containsUsageLimitReached(auth.StatusMessage)
}

func authEligibleForMaintenanceDisable(auth *coreauth.Auth, result *coreauth.Result, cfg config.AuthMaintenanceConfig) (string, bool) {
	if reason, ok := authProtectedMaintenanceDisableReason(auth, result); ok {
		return reason, true
	}
	if authMaintenancePendingAction(auth, authMaintenanceActionDelete) {
		return authMaintenancePendingReason(auth)
	}
	if authMaintenancePendingAction(auth, authMaintenanceActionDisable) {
		return authMaintenancePendingReason(auth)
	}
	if auth == nil {
		return "", false
	}
	if statusCode := authMaintenanceStatusCode(auth, result); statusCode > 0 {
		if containsStatusCode(cfg.DisableStatusCodes, statusCode) {
			return fmt.Sprintf("http_%d", statusCode), true
		}
		if statusCode != http.StatusUnauthorized && containsStatusCode(cfg.DeleteStatusCodes, statusCode) {
			return fmt.Sprintf("http_%d", statusCode), true
		}
	}
	if cfg.DeleteQuotaExceeded && auth.Quota.Exceeded && auth.Quota.BackoffLevel >= cfg.QuotaStrikeThreshold {
		return fmt.Sprintf("quota_strikes_%d", auth.Quota.BackoffLevel), true
	}
	return "", false
}

func authProtectedMaintenanceDisableReason(auth *coreauth.Auth, result *coreauth.Result) (string, bool) {
	if auth == nil {
		return "", false
	}
	if statusCode := authMaintenanceStatusCode(auth, result); statusCode == http.StatusTooManyRequests {
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

func unauthorizedCleanupStatusCode(err *coreauth.Error) int {
	if err == nil {
		return 0
	}
	return err.HTTPStatus
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

func authEligibleForMaintenanceDelete(auth *coreauth.Auth, result *coreauth.Result, cfg config.AuthMaintenanceConfig) (string, bool) {
	_ = auth
	_ = result
	_ = cfg
	return "", false
}

func authMaintenanceStatusCode(auth *coreauth.Auth, result *coreauth.Result) int {
	if result != nil && result.Error != nil && result.Error.HTTPStatus > 0 {
		return result.Error.HTTPStatus
	}
	if auth == nil {
		return 0
	}
	if auth.LastError != nil && auth.LastError.HTTPStatus > 0 {
		return auth.LastError.HTTPStatus
	}
	return authMaintenanceStatusCodeFromMessage(auth.StatusMessage)
}

func authMaintenanceStatusCodeFromMessage(message string) int {
	message = strings.TrimSpace(message)
	if message == "" {
		return 0
	}
	if statusCode := int(gjson.Get(message, "status").Int()); statusCode > 0 {
		return statusCode
	}
	if strings.EqualFold(strings.TrimSpace(gjson.Get(message, "error.type").String()), "usage_limit_reached") {
		return http.StatusTooManyRequests
	}
	switch strings.ToLower(strings.TrimSpace(gjson.Get(message, "error.code").String())) {
	case "token_invalidated", "token_revoked":
		return http.StatusUnauthorized
	default:
		return 0
	}
}

func containsStatusCode(codes []int, want int) bool {
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

func resolveAuthFilePath(auth *coreauth.Auth, authDir string) string {
	if auth == nil {
		return ""
	}
	if (auth.Disabled || auth.Status == coreauth.StatusDisabled) &&
		!authMaintenancePendingAction(auth, authMaintenanceActionDelete) &&
		!authMaintenancePendingAction(auth, authMaintenanceActionDisable) {
		return ""
	}
	if auth.Attributes != nil && strings.EqualFold(strings.TrimSpace(auth.Attributes["runtime_only"]), "true") {
		return ""
	}
	path := ""
	if auth.Attributes != nil {
		path = strings.TrimSpace(auth.Attributes["path"])
	}
	if path == "" {
		path = strings.TrimSpace(auth.FileName)
	}
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		if authDir == "" {
			return ""
		}
		path = filepath.Join(authDir, filepath.FromSlash(path))
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if authDir != "" && sdkAuth.IsTrashPath(authDir, path) {
		return ""
	}
	return path
}

func authMaintenanceCandidateKey(action authMaintenanceAction, path string) string {
	return string(action) + ":" + strings.TrimSpace(path)
}

func authMaintenancePendingAction(auth *coreauth.Auth, action authMaintenanceAction) bool {
	if auth == nil || auth.Metadata == nil {
		return false
	}
	var key string
	switch action {
	case authMaintenanceActionDisable:
		key = authMaintenancePendingDisableMetadataKey
	default:
		key = authMaintenancePendingDeleteMetadataKey
	}
	raw, ok := auth.Metadata[key]
	if !ok {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func authMaintenancePendingCandidateAction(auth *coreauth.Auth) authMaintenanceAction {
	switch {
	case authMaintenancePendingAction(auth, authMaintenanceActionDelete):
		return authMaintenanceActionDelete
	case authMaintenancePendingAction(auth, authMaintenanceActionDisable):
		return authMaintenanceActionDisable
	default:
		return ""
	}
}

func authMaintenancePendingReason(auth *coreauth.Auth) (string, bool) {
	if auth == nil || auth.Metadata == nil {
		return "", false
	}
	reason, _ := auth.Metadata[authMaintenanceReasonMetadataKey].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		action := authMaintenancePendingCandidateAction(auth)
		if action == "" {
			return "", false
		}
		switch action {
		case authMaintenanceActionDisable:
			return "disabled after usage_limit_reached", true
		default:
			return "disabled via auth maintenance", true
		}
	}
	return reason, true
}
