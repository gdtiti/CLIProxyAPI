package codexquota

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
)

const codexProvider = "codex"

const (
	eventTypeRegistered     = "registered"
	eventTypeDisabled       = "disabled"
	eventTypeEnabled        = "enabled"
	eventTypeRefreshSuccess = "refresh_success"
	eventTypeRefreshFailed  = "refresh_failed"
	eventTypeQuotaExceeded  = "quota_exceeded"
	eventTypeQuotaRecovered = "quota_recovered"
	eventTypeUnavailable    = "unavailable"
)

type Service struct {
	store *store

	mu        sync.RWMutex
	auths     *coreauth.Manager
	snapshots map[string]Snapshot
	rollups   map[string]UsageRollup
	events    []Event
}

type serviceHook struct {
	service *Service
}

func NewService(authDir string) (*Service, error) {
	backingStore, err := newStore(authDir)
	if err != nil {
		return nil, err
	}
	snapshots, rollups, events, err := backingStore.loadState()
	if err != nil {
		return nil, err
	}
	return &Service{
		store:     backingStore,
		snapshots: snapshots,
		rollups:   rollups,
		events:    events,
	}, nil
}

func (s *Service) Hook() coreauth.Hook {
	return serviceHook{service: s}
}

func (s *Service) SetAuthManager(manager *coreauth.Manager) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.auths = manager
	s.mu.Unlock()
}

func (s *Service) SyncAuths(auths []*coreauth.Auth) {
	if s == nil || len(auths) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for _, auth := range auths {
		if !isCodexAuth(auth) {
			continue
		}
		snapshot := snapshotFromAuth(auth)
		if snapshot.AuthID == "" && snapshot.AuthIndex == "" {
			continue
		}
		prev, existed := s.replaceSnapshotLocked(snapshot)
		s.syncRollupLocked(snapshot)
		if !existed || !snapshotsEqual(prev, snapshot) {
			changed = true
		}
	}
	if changed {
		s.persistLocked()
	}
}

func (s *Service) HandleAuthRegistered(auth *coreauth.Auth) {
	if s == nil || !isCodexAuth(auth) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := snapshotFromAuth(auth)
	prev, existed := s.replaceSnapshotLocked(snapshot)
	s.syncRollupLocked(snapshot)
	if !existed || !snapshotsEqual(prev, snapshot) {
		if !existed {
			s.events = append(s.events, newEvent(eventTypeRegistered, snapshot, s.rollupForSnapshotLocked(snapshot), nil, time.Now().UTC()))
		}
		s.persistLocked()
	}
}

func (s *Service) HandleAuthUpdated(auth *coreauth.Auth) {
	if s == nil || !isCodexAuth(auth) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	snapshot := snapshotFromAuth(auth)
	prev, existed := s.replaceSnapshotLocked(snapshot)
	s.syncRollupLocked(snapshot)
	if !existed {
		s.events = append(s.events, newEvent(eventTypeRegistered, snapshot, s.rollupForSnapshotLocked(snapshot), nil, now))
		s.persistLocked()
		return
	}

	changed := !snapshotsEqual(prev, snapshot)
	s.appendAuthTransitionEventsLocked(prev, snapshot, now)
	if changed {
		s.persistLocked()
	}
}

func (s *Service) HandleResult(result coreauth.Result) {
	if s == nil || result.AuthID == "" {
		return
	}

	manager := s.authManager()
	if manager == nil {
		return
	}
	auth, ok := manager.GetByID(result.AuthID)
	if !ok || !isCodexAuth(auth) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	snapshot := snapshotFromAuth(auth)
	prev, existed := s.replaceSnapshotLocked(snapshot)
	s.syncRollupLocked(snapshot)
	rollup := s.rollupForSnapshotLocked(snapshot)

	changed := !existed || !snapshotsEqual(prev, snapshot)
	if result.Success {
		if existed && prev.QuotaExceeded && !snapshot.QuotaExceeded {
			s.events = append(s.events, newEvent(eventTypeQuotaRecovered, snapshot, rollup, nil, now))
			changed = true
		}
		if existed && prev.Unavailable && !snapshot.Unavailable && !snapshot.QuotaExceeded {
			s.events = append(s.events, newEvent(eventTypeEnabled, snapshot, rollup, nil, now))
			changed = true
		}
		if changed {
			s.persistLocked()
		}
		return
	}

	if snapshot.QuotaExceeded {
		if !existed || !prev.QuotaExceeded || !prev.NextRecoverAt.Equal(snapshot.NextRecoverAt) || prev.QuotaModel != snapshot.QuotaModel || prev.LastErrorMessage != snapshot.LastErrorMessage {
			s.events = append(s.events, newEvent(eventTypeQuotaExceeded, snapshot, rollup, &result, now))
			changed = true
		}
	} else if snapshot.Unavailable {
		if !existed || !prev.Unavailable || !prev.NextRetryAfter.Equal(snapshot.NextRetryAfter) || prev.LastErrorMessage != snapshot.LastErrorMessage {
			s.events = append(s.events, newEvent(eventTypeUnavailable, snapshot, rollup, &result, now))
			changed = true
		}
	}

	if changed {
		s.persistLocked()
	}
}

func (s *Service) ApplyUsage(record coreusage.Record) {
	if s == nil || !strings.EqualFold(strings.TrimSpace(record.Provider), codexProvider) {
		return
	}
	key := stateKey(record.AuthID, record.AuthIndex)
	if key == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rollup := s.rollups[key]
	rollup.AuthID = firstNonEmpty(record.AuthID, rollup.AuthID)
	rollup.AuthIndex = firstNonEmpty(record.AuthIndex, rollup.AuthIndex)
	rollup.Provider = firstNonEmpty(strings.TrimSpace(record.Provider), rollup.Provider, codexProvider)
	if rollup.Account == "" {
		if snapshot, ok := s.snapshotByKeyLocked(key); ok {
			rollup.Account = snapshot.Account
			if rollup.AuthIndex == "" {
				rollup.AuthIndex = snapshot.AuthIndex
			}
			if rollup.AuthID == "" {
				rollup.AuthID = snapshot.AuthID
			}
		}
	}
	rollup.RequestCount++
	rollup.InputTokens += record.Detail.InputTokens
	rollup.OutputTokens += record.Detail.OutputTokens
	rollup.CachedTokens += record.Detail.CachedTokens
	rollup.ReasoningTokens += record.Detail.ReasoningTokens
	rollup.TotalTokens += record.Detail.TotalTokens
	if record.RequestedAt.After(rollup.LastRequestedAt) {
		rollup.LastRequestedAt = record.RequestedAt
	}
	rollup.UpdatedAt = time.Now().UTC()
	updateRollupAverages(&rollup)
	s.rollups[key] = rollup
	s.persistLocked()
}

func (s *Service) ListSnapshots() []SnapshotView {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]SnapshotView, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		items = append(items, SnapshotView{
			Snapshot: snapshot,
			Usage:    s.rollupForSnapshotLocked(snapshot),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AuthIndex == items[j].AuthIndex {
			return items[i].AuthID < items[j].AuthID
		}
		return items[i].AuthIndex < items[j].AuthIndex
	})
	return items
}

func (s *Service) GetSnapshot(authKey string) (SnapshotView, bool) {
	if s == nil {
		return SnapshotView{}, false
	}
	authKey = strings.TrimSpace(authKey)
	if authKey == "" {
		return SnapshotView{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if snapshot, ok := s.findSnapshotLocked(authKey); ok {
		return SnapshotView{
			Snapshot: snapshot,
			Usage:    s.rollupForSnapshotLocked(snapshot),
		}, true
	}
	return SnapshotView{}, false
}

func (s *Service) ListEvents(authKey string, limit int) []Event {
	if s == nil {
		return nil
	}
	authKey = strings.TrimSpace(authKey)

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		if authKey == "" || event.AuthID == authKey || event.AuthIndex == authKey {
			items = append(items, event)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *Service) ListRollups() []UsageRollup {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]UsageRollup, 0, len(s.rollups))
	for _, rollup := range s.rollups {
		items = append(items, rollup)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].AuthIndex == items[j].AuthIndex {
			return items[i].AuthID < items[j].AuthID
		}
		return items[i].AuthIndex < items[j].AuthIndex
	})
	return items
}

func (h serviceHook) OnAuthRegistered(ctx context.Context, auth *coreauth.Auth) {
	_ = ctx
	if auth == nil || h.service == nil {
		return
	}
	h.service.HandleAuthRegistered(auth)
}

func (h serviceHook) OnAuthUpdated(ctx context.Context, auth *coreauth.Auth) {
	_ = ctx
	if auth == nil || h.service == nil {
		return
	}
	h.service.HandleAuthUpdated(auth)
}

func (h serviceHook) OnResult(ctx context.Context, result coreauth.Result) {
	_ = ctx
	if h.service == nil {
		return
	}
	h.service.HandleResult(result)
}

func (s *Service) authManager() *coreauth.Manager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.auths
}

func (s *Service) appendAuthTransitionEventsLocked(prev, next Snapshot, now time.Time) {
	rollup := s.rollupForSnapshotLocked(next)

	if !prev.Disabled && next.Disabled {
		s.events = append(s.events, newEvent(eventTypeDisabled, next, rollup, nil, now))
	}
	if prev.Disabled && !next.Disabled {
		s.events = append(s.events, newEvent(eventTypeEnabled, next, rollup, nil, now))
	}
	if !prev.QuotaExceeded && next.QuotaExceeded {
		s.events = append(s.events, newEvent(eventTypeQuotaExceeded, next, rollup, nil, now))
	}
	if prev.QuotaExceeded && !next.QuotaExceeded {
		s.events = append(s.events, newEvent(eventTypeQuotaRecovered, next, rollup, nil, now))
	}
	if !next.LastRefreshedAt.IsZero() && !prev.LastRefreshedAt.Equal(next.LastRefreshedAt) && next.LastErrorMessage == "" {
		s.events = append(s.events, newEvent(eventTypeRefreshSuccess, next, rollup, nil, now))
	}
	if next.LastErrorMessage != "" && prev.LastErrorMessage != next.LastErrorMessage && !next.NextRefreshAfter.IsZero() && !prev.LastRefreshedAt.After(next.LastRefreshedAt) {
		s.events = append(s.events, newEvent(eventTypeRefreshFailed, next, rollup, nil, now))
	}
}

func (s *Service) replaceSnapshotLocked(snapshot Snapshot) (Snapshot, bool) {
	key := stateKey(snapshot.AuthID, snapshot.AuthIndex)
	if key == "" {
		return Snapshot{}, false
	}

	prev, existed := s.snapshotByKeyLocked(key)
	for existingKey, existing := range s.snapshots {
		if existingKey == key {
			continue
		}
		if existing.AuthID != "" && existing.AuthID == snapshot.AuthID {
			delete(s.snapshots, existingKey)
			if rollup, ok := s.rollups[existingKey]; ok {
				delete(s.rollups, existingKey)
				s.rollups[key] = rollup
			}
			if !existed {
				prev = existing
				existed = true
			}
		}
	}

	s.snapshots[key] = snapshot
	return prev, existed
}

func (s *Service) snapshotByKeyLocked(key string) (Snapshot, bool) {
	snapshot, ok := s.snapshots[key]
	return snapshot, ok
}

func (s *Service) findSnapshotLocked(authKey string) (Snapshot, bool) {
	for _, snapshot := range s.snapshots {
		if snapshot.AuthID == authKey || snapshot.AuthIndex == authKey {
			return snapshot, true
		}
	}
	return Snapshot{}, false
}

func (s *Service) rollupForSnapshotLocked(snapshot Snapshot) UsageRollup {
	key := stateKey(snapshot.AuthID, snapshot.AuthIndex)
	rollup := s.rollups[key]
	rollup.AuthID = firstNonEmpty(snapshot.AuthID, rollup.AuthID)
	rollup.AuthIndex = firstNonEmpty(snapshot.AuthIndex, rollup.AuthIndex)
	rollup.Provider = firstNonEmpty(snapshot.Provider, rollup.Provider, codexProvider)
	rollup.Account = firstNonEmpty(snapshot.Account, rollup.Account)
	return rollup
}

func (s *Service) syncRollupLocked(snapshot Snapshot) {
	key := stateKey(snapshot.AuthID, snapshot.AuthIndex)
	if key == "" {
		return
	}
	rollup := s.rollups[key]
	rollup.AuthID = firstNonEmpty(snapshot.AuthID, rollup.AuthID)
	rollup.AuthIndex = firstNonEmpty(snapshot.AuthIndex, rollup.AuthIndex)
	rollup.Provider = firstNonEmpty(snapshot.Provider, rollup.Provider, codexProvider)
	rollup.Account = firstNonEmpty(snapshot.Account, rollup.Account)
	s.rollups[key] = rollup
}

func (s *Service) persistLocked() {
	if s.store == nil {
		return
	}
	if err := s.store.saveState(s.snapshots, s.rollups, s.events); err != nil {
		log.WithError(err).Warn("codex quota store persistence failed")
	}
}

func newEvent(eventType string, snapshot Snapshot, rollup UsageRollup, result *coreauth.Result, now time.Time) Event {
	event := Event{
		ID:              uuid.NewString(),
		AuthID:          snapshot.AuthID,
		AuthIndex:       snapshot.AuthIndex,
		Provider:        snapshot.Provider,
		EventType:       eventType,
		Reason:          eventReason(snapshot, result),
		StatusMessage:   snapshot.StatusMessage,
		LastError:       snapshot.LastErrorMessage,
		Disabled:        snapshot.Disabled,
		Unavailable:     snapshot.Unavailable,
		QuotaExceeded:   snapshot.QuotaExceeded,
		QuotaReason:     snapshot.QuotaReason,
		QuotaModel:      snapshot.QuotaModel,
		RequestCount:    rollup.RequestCount,
		InputTokens:     rollup.InputTokens,
		OutputTokens:    rollup.OutputTokens,
		CachedTokens:    rollup.CachedTokens,
		ReasoningTokens: rollup.ReasoningTokens,
		TotalTokens:     rollup.TotalTokens,
		RecoveredTokens: rollup.RecoveredTokens,
		CreatedAt:       now,
	}
	if result != nil && result.Error != nil {
		event.HTTPStatus = result.Error.HTTPStatus
	}
	switch eventType {
	case eventTypeDisabled, eventTypeQuotaExceeded, eventTypeUnavailable:
		event.DisabledAt = timePointer(now)
	case eventTypeEnabled, eventTypeQuotaRecovered:
		event.EnabledAt = timePointer(now)
	}
	if !snapshot.NextRecoverAt.IsZero() {
		event.RecoverAt = timePointer(snapshot.NextRecoverAt)
	}
	return event
}

func snapshotFromAuth(auth *coreauth.Auth) Snapshot {
	if auth == nil {
		return Snapshot{}
	}
	index := auth.EnsureIndex()
	accountType, account := auth.AccountInfo()
	account = sanitizeAccountValue(accountType, account)
	expiresAt, _ := auth.ExpirationTime()
	return Snapshot{
		AuthID:           strings.TrimSpace(auth.ID),
		AuthIndex:        index,
		Provider:         strings.ToLower(strings.TrimSpace(auth.Provider)),
		FileName:         authFileName(auth.FileName),
		Label:            strings.TrimSpace(auth.Label),
		AccountType:      strings.TrimSpace(accountType),
		Account:          account,
		ExpiresAt:        expiresAt.UTC(),
		Status:           string(auth.Status),
		StatusMessage:    strings.TrimSpace(auth.StatusMessage),
		LastErrorMessage: lastErrorMessage(auth.LastError),
		Disabled:         auth.Disabled,
		Unavailable:      auth.Unavailable,
		QuotaExceeded:    auth.Quota.Exceeded,
		QuotaReason:      strings.TrimSpace(auth.Quota.Reason),
		QuotaModel:       quotaModelFromAuth(auth),
		NextRecoverAt:    auth.Quota.NextRecoverAt.UTC(),
		LastRefreshedAt:  auth.LastRefreshedAt.UTC(),
		NextRefreshAfter: auth.NextRefreshAfter.UTC(),
		NextRetryAfter:   auth.NextRetryAfter.UTC(),
		UpdatedAt:        auth.UpdatedAt.UTC(),
	}
}

func quotaModelFromAuth(auth *coreauth.Auth) string {
	if auth == nil || len(auth.ModelStates) == 0 {
		return ""
	}
	models := make([]string, 0, len(auth.ModelStates))
	for model, state := range auth.ModelStates {
		if state == nil || !state.Quota.Exceeded {
			continue
		}
		models = append(models, model)
	}
	if len(models) == 0 {
		return ""
	}
	sort.Strings(models)
	return models[0]
}

func eventReason(snapshot Snapshot, result *coreauth.Result) string {
	if snapshot.QuotaExceeded {
		if snapshot.QuotaReason != "" {
			return snapshot.QuotaReason
		}
		return "quota"
	}
	if result != nil && result.Error != nil {
		if result.Error.Code != "" {
			return result.Error.Code
		}
		if result.Error.Message != "" {
			return result.Error.Message
		}
	}
	if snapshot.LastErrorMessage != "" {
		return snapshot.LastErrorMessage
	}
	if snapshot.StatusMessage != "" {
		return snapshot.StatusMessage
	}
	if snapshot.Disabled {
		return "disabled"
	}
	if snapshot.Unavailable {
		return "unavailable"
	}
	return ""
}

func updateRollupAverages(rollup *UsageRollup) {
	if rollup == nil || rollup.RequestCount <= 0 {
		return
	}
	count := float64(rollup.RequestCount)
	rollup.AvgInputTokens = float64(rollup.InputTokens) / count
	rollup.AvgOutputTokens = float64(rollup.OutputTokens) / count
	rollup.AvgCachedTokens = float64(rollup.CachedTokens) / count
	rollup.AvgReasoning = float64(rollup.ReasoningTokens) / count
	rollup.AvgTotalTokens = float64(rollup.TotalTokens) / count
}

func stateKey(authID, authIndex string) string {
	authID = strings.TrimSpace(authID)
	if authID != "" {
		return "id:" + authID
	}
	authIndex = strings.TrimSpace(authIndex)
	if authIndex != "" {
		return "index:" + authIndex
	}
	return ""
}

func isCodexAuth(auth *coreauth.Auth) bool {
	return auth != nil && strings.EqualFold(strings.TrimSpace(auth.Provider), codexProvider)
}

func sanitizeAccountValue(accountType, account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "api_key", "personal_access_token":
		return maskSecret(account)
	default:
		if strings.HasPrefix(account, "sk-") {
			return maskSecret(account)
		}
		return account
	}
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

func authFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return filepath.Base(name)
}

func lastErrorMessage(err *coreauth.Error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Message)
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func snapshotsEqual(a, b Snapshot) bool {
	return a == b
}
