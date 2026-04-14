package auth

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

const (
	persistedRuntimeStateKey       = "cliproxy_runtime_state"
	persistedUnauthorizedRetention = 24 * time.Hour
	persistedRefreshHistoryLimit   = 100
)

// PersistedRuntimeStateMetadataKey is the metadata key used to store auth runtime
// cooldown and unauthorized history across restarts.
const PersistedRuntimeStateMetadataKey = persistedRuntimeStateKey

type persistedRuntimeState struct {
	Auths map[string]persistedAuthRuntime `json:"auths,omitempty"`
}

type persistedAuthRuntime struct {
	RequestCount         int                            `json:"request_count,omitempty"`
	UnauthorizedFailures []string                       `json:"unauthorized_failures,omitempty"`
	RefreshHistory       []persistedRefreshHistoryEntry `json:"refresh_history,omitempty"`
	Status               Status                         `json:"status,omitempty"`
	StatusMessage        string                         `json:"status_message,omitempty"`
	Unavailable          bool                           `json:"unavailable,omitempty"`
	NextRetryAfter       string                         `json:"next_retry_after,omitempty"`
	Quota                *persistedQuotaState           `json:"quota,omitempty"`
	ModelStates          map[string]persistedModelState `json:"model_states,omitempty"`
}

// RefreshHistoryEntry describes one persisted automatic refresh attempt.
type RefreshHistoryEntry struct {
	At        time.Time `json:"at"`
	Trigger   string    `json:"trigger,omitempty"`
	Result    string    `json:"result,omitempty"`
	Message   string    `json:"message,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type persistedRefreshHistoryEntry struct {
	At        string `json:"at,omitempty"`
	Trigger   string `json:"trigger,omitempty"`
	Result    string `json:"result,omitempty"`
	Message   string `json:"message,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type persistedModelState struct {
	Status         Status               `json:"status,omitempty"`
	StatusMessage  string               `json:"status_message,omitempty"`
	Unavailable    bool                 `json:"unavailable,omitempty"`
	NextRetryAfter string               `json:"next_retry_after,omitempty"`
	Quota          *persistedQuotaState `json:"quota,omitempty"`
}

type persistedQuotaState struct {
	Exceeded      bool   `json:"exceeded,omitempty"`
	Reason        string `json:"reason,omitempty"`
	NextRecoverAt string `json:"next_recover_at,omitempty"`
	BackoffLevel  int    `json:"backoff_level,omitempty"`
}

// ApplyPersistedRuntimeState restores persisted runtime cooldown state for the auth entry.
func ApplyPersistedRuntimeState(auth *Auth, now time.Time) {
	if auth == nil || auth.Metadata == nil {
		return
	}
	state, ok := loadPersistedRuntimeState(auth.Metadata)
	if !ok {
		return
	}
	entry, ok := state.Auths[strings.TrimSpace(auth.ID)]
	if !ok {
		return
	}

	if restored := restorePersistedAuthRuntime(auth, entry, now); restored {
		if auth.Status == "" {
			auth.Status = StatusError
		}
	}
}

// RecordUnauthorizedFailure adds a new 401 timestamp for the auth and returns the
// number of timestamps remaining within the configured window.
func RecordUnauthorizedFailure(auth *Auth, now time.Time, window time.Duration) int {
	if auth == nil {
		return 0
	}
	state, _ := loadPersistedRuntimeState(auth.Metadata)
	entry := state.entryFor(auth.ID)
	entry.UnauthorizedFailures = append(trimUnauthorizedFailures(entry.UnauthorizedFailures, now, window), now.UTC().Format(time.RFC3339Nano))
	state.setEntry(auth.ID, entry)
	storePersistedRuntimeState(auth, state)
	return len(entry.UnauthorizedFailures)
}

// RecordCompletedRequest increments the persisted cumulative completed-request count
// for the auth and returns the updated total.
func RecordCompletedRequest(auth *Auth) int {
	if auth == nil {
		return 0
	}
	state, _ := loadPersistedRuntimeState(auth.Metadata)
	entry := state.entryFor(auth.ID)
	entry.RequestCount++
	state.setEntry(auth.ID, entry)
	storePersistedRuntimeState(auth, state)
	return entry.RequestCount
}

// RecordRefreshHistory appends one refresh-history record for the auth.
func RecordRefreshHistory(auth *Auth, now time.Time, trigger, result, message string, expiresAt time.Time) int {
	if auth == nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	state, _ := loadPersistedRuntimeState(auth.Metadata)
	entry := state.entryFor(auth.ID)
	entry.RefreshHistory = append(entry.RefreshHistory, newPersistedRefreshHistoryEntry(now, trigger, result, message, expiresAt))
	entry.RefreshHistory = trimPersistedRefreshHistory(entry.RefreshHistory)
	state.setEntry(auth.ID, entry)
	storePersistedRuntimeState(auth, state)
	return len(entry.RefreshHistory)
}

// ListRefreshHistory returns the persisted refresh history for the auth.
func ListRefreshHistory(auth *Auth) []RefreshHistoryEntry {
	if auth == nil {
		return nil
	}
	state, ok := loadPersistedRuntimeState(auth.Metadata)
	if !ok {
		return nil
	}
	entry, ok := state.Auths[strings.TrimSpace(auth.ID)]
	if !ok || len(entry.RefreshHistory) == 0 {
		return nil
	}
	out := make([]RefreshHistoryEntry, 0, len(entry.RefreshHistory))
	for _, persisted := range entry.RefreshHistory {
		item, ok := parsePersistedRefreshHistoryEntry(persisted)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ClearUnauthorizedFailureHistory removes persisted 401 timestamps for the auth.
func ClearUnauthorizedFailureHistory(auth *Auth) bool {
	if auth == nil || auth.Metadata == nil {
		return false
	}
	state, ok := loadPersistedRuntimeState(auth.Metadata)
	if !ok {
		return false
	}
	entry, exists := state.Auths[strings.TrimSpace(auth.ID)]
	if !exists || len(entry.UnauthorizedFailures) == 0 {
		return false
	}
	entry.UnauthorizedFailures = nil
	state.setEntry(auth.ID, entry)
	return storePersistedRuntimeState(auth, state)
}

// SyncRuntimePersistence stores the auth runtime cooldown state back into metadata.
// It only persists transient cooldown fields and 401 history needed across restarts.
func SyncRuntimePersistence(auth *Auth, now time.Time) bool {
	if auth == nil {
		return false
	}
	state, _ := loadPersistedRuntimeState(auth.Metadata)
	entry := state.entryFor(auth.ID)
	entry.UnauthorizedFailures = trimUnauthorizedFailures(entry.UnauthorizedFailures, now, persistedUnauthorizedRetention)
	populatePersistedAuthRuntime(&entry, auth, now)
	state.setEntry(auth.ID, entry)
	return storePersistedRuntimeState(auth, state)
}

func populatePersistedAuthRuntime(entry *persistedAuthRuntime, auth *Auth, now time.Time) {
	if entry == nil || auth == nil {
		return
	}

	// Keep only the fields required to restore cooldown state after restart.
	entry.Status = ""
	entry.StatusMessage = ""
	entry.Unavailable = false
	entry.NextRetryAfter = ""
	entry.Quota = nil
	entry.ModelStates = nil

	if next := persistedNextRetry(auth.NextRetryAfter, auth.Quota.NextRecoverAt, now); !next.IsZero() {
		entry.Status = auth.Status
		entry.StatusMessage = strings.TrimSpace(auth.StatusMessage)
		entry.Unavailable = auth.Unavailable || auth.Quota.Exceeded
		entry.NextRetryAfter = next.UTC().Format(time.RFC3339Nano)
		if quota := toPersistedQuotaState(auth.Quota, now); quota != nil {
			entry.Quota = quota
		}
	}

	for model, modelState := range auth.ModelStates {
		if modelState == nil {
			continue
		}
		next := persistedNextRetry(modelState.NextRetryAfter, modelState.Quota.NextRecoverAt, now)
		if next.IsZero() && modelState.Status != StatusDisabled {
			continue
		}
		if entry.ModelStates == nil {
			entry.ModelStates = make(map[string]persistedModelState)
		}
		persisted := persistedModelState{
			Status:         modelState.Status,
			StatusMessage:  strings.TrimSpace(modelState.StatusMessage),
			Unavailable:    modelState.Unavailable || modelState.Quota.Exceeded,
			NextRetryAfter: "",
			Quota:          toPersistedQuotaState(modelState.Quota, now),
		}
		if !next.IsZero() {
			persisted.NextRetryAfter = next.UTC().Format(time.RFC3339Nano)
		}
		if !persistedModelRuntimeEmpty(persisted) {
			entry.ModelStates[model] = persisted
		}
	}
}

func restorePersistedAuthRuntime(auth *Auth, entry persistedAuthRuntime, now time.Time) bool {
	restored := false
	if next := parsePersistedTime(entry.NextRetryAfter); next.After(now) {
		auth.Status = normalizePersistedStatus(entry.Status, auth.Status)
		auth.StatusMessage = pickPersistedMessage(entry.StatusMessage, auth.StatusMessage)
		auth.Unavailable = entry.Unavailable || auth.Unavailable
		auth.NextRetryAfter = next
		if quota := fromPersistedQuotaState(entry.Quota); quota != nil {
			auth.Quota = *quota
		}
		restored = true
	}

	if len(entry.ModelStates) > 0 {
		if auth.ModelStates == nil {
			auth.ModelStates = make(map[string]*ModelState, len(entry.ModelStates))
		}
		for model, persisted := range entry.ModelStates {
			next := parsePersistedTime(persisted.NextRetryAfter)
			quota := fromPersistedQuotaState(persisted.Quota)
			if !next.After(now) && persisted.Status != StatusDisabled && (quota == nil || quota.NextRecoverAt.IsZero() || !quota.NextRecoverAt.After(now)) {
				continue
			}
			modelState := ensureModelState(auth, model)
			modelState.Status = normalizePersistedStatus(persisted.Status, modelState.Status)
			modelState.StatusMessage = pickPersistedMessage(persisted.StatusMessage, modelState.StatusMessage)
			modelState.Unavailable = persisted.Unavailable || (quota != nil && quota.Exceeded)
			modelState.NextRetryAfter = next
			if quota != nil {
				modelState.Quota = *quota
			}
			restored = true
		}
		updateAggregatedAvailability(auth, now)
	}

	return restored
}

func persistedNextRetry(primary, quotaRecover, now time.Time) time.Time {
	next := primary
	if !quotaRecover.IsZero() && (next.IsZero() || quotaRecover.After(next)) {
		next = quotaRecover
	}
	if next.IsZero() || !next.After(now) {
		return time.Time{}
	}
	return next
}

func trimUnauthorizedFailures(values []string, now time.Time, window time.Duration) []string {
	if len(values) == 0 {
		return nil
	}
	if window <= 0 {
		window = persistedUnauthorizedRetention
	}
	cutoff := now.Add(-window)
	out := make([]string, 0, len(values))
	for _, raw := range values {
		ts := parsePersistedTime(raw)
		if ts.IsZero() || ts.Before(cutoff) {
			continue
		}
		out = append(out, ts.UTC().Format(time.RFC3339Nano))
	}
	return out
}

func loadPersistedRuntimeState(metadata map[string]any) (persistedRuntimeState, bool) {
	if len(metadata) == 0 {
		return persistedRuntimeState{}, false
	}
	raw, ok := metadata[persistedRuntimeStateKey]
	if !ok || raw == nil {
		return persistedRuntimeState{}, false
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return persistedRuntimeState{}, false
	}
	var state persistedRuntimeState
	if err := json.Unmarshal(payload, &state); err != nil {
		return persistedRuntimeState{}, false
	}
	if len(state.Auths) == 0 {
		return persistedRuntimeState{}, false
	}
	return state, true
}

func storePersistedRuntimeState(auth *Auth, state persistedRuntimeState) bool {
	if auth == nil {
		return false
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	before, hasBefore := auth.Metadata[persistedRuntimeStateKey]
	if len(state.Auths) == 0 {
		if !hasBefore {
			return false
		}
		delete(auth.Metadata, persistedRuntimeStateKey)
		return true
	}

	normalized, ok := normalizePersistedRuntimeState(state)
	if !ok {
		if !hasBefore {
			return false
		}
		delete(auth.Metadata, persistedRuntimeStateKey)
		return true
	}
	if hasBefore && reflect.DeepEqual(before, normalized) {
		return false
	}
	auth.Metadata[persistedRuntimeStateKey] = normalized
	return true
}

func normalizePersistedRuntimeState(state persistedRuntimeState) (map[string]any, bool) {
	payload, err := json.Marshal(state)
	if err != nil {
		return nil, false
	}
	normalized := make(map[string]any)
	if err := json.Unmarshal(payload, &normalized); err != nil {
		return nil, false
	}
	authsRaw, ok := normalized["auths"].(map[string]any)
	if !ok || len(authsRaw) == 0 {
		return nil, false
	}
	return normalized, true
}

func (s *persistedRuntimeState) entryFor(authID string) persistedAuthRuntime {
	if s.Auths == nil {
		s.Auths = make(map[string]persistedAuthRuntime)
	}
	authID = strings.TrimSpace(authID)
	if authID == "" {
		return persistedAuthRuntime{}
	}
	return s.Auths[authID]
}

func (s *persistedRuntimeState) setEntry(authID string, entry persistedAuthRuntime) {
	if s == nil {
		return
	}
	authID = strings.TrimSpace(authID)
	if authID == "" {
		return
	}
	if entry.empty() {
		if s.Auths != nil {
			delete(s.Auths, authID)
		}
		return
	}
	if s.Auths == nil {
		s.Auths = make(map[string]persistedAuthRuntime)
	}
	s.Auths[authID] = entry
}

func (e persistedAuthRuntime) empty() bool {
	return e.RequestCount == 0 &&
		len(e.UnauthorizedFailures) == 0 &&
		len(e.RefreshHistory) == 0 &&
		e.Status == "" &&
		strings.TrimSpace(e.StatusMessage) == "" &&
		!e.Unavailable &&
		strings.TrimSpace(e.NextRetryAfter) == "" &&
		e.Quota == nil &&
		len(e.ModelStates) == 0
}

func newPersistedRefreshHistoryEntry(now time.Time, trigger, result, message string, expiresAt time.Time) persistedRefreshHistoryEntry {
	entry := persistedRefreshHistoryEntry{
		At:      now.UTC().Format(time.RFC3339Nano),
		Trigger: strings.TrimSpace(trigger),
		Result:  strings.TrimSpace(result),
		Message: strings.TrimSpace(message),
	}
	if !expiresAt.IsZero() {
		entry.ExpiresAt = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	return entry
}

func trimPersistedRefreshHistory(values []persistedRefreshHistoryEntry) []persistedRefreshHistoryEntry {
	if len(values) == 0 {
		return nil
	}
	filtered := make([]persistedRefreshHistoryEntry, 0, len(values))
	for _, item := range values {
		if strings.TrimSpace(item.At) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) <= persistedRefreshHistoryLimit {
		return filtered
	}
	out := make([]persistedRefreshHistoryEntry, persistedRefreshHistoryLimit)
	copy(out, filtered[len(filtered)-persistedRefreshHistoryLimit:])
	return out
}

func parsePersistedRefreshHistoryEntry(entry persistedRefreshHistoryEntry) (RefreshHistoryEntry, bool) {
	at := parsePersistedTime(entry.At)
	if at.IsZero() {
		return RefreshHistoryEntry{}, false
	}
	item := RefreshHistoryEntry{
		At:      at,
		Trigger: strings.TrimSpace(entry.Trigger),
		Result:  strings.TrimSpace(entry.Result),
		Message: strings.TrimSpace(entry.Message),
	}
	if expiresAt := parsePersistedTime(entry.ExpiresAt); !expiresAt.IsZero() {
		item.ExpiresAt = expiresAt
	}
	return item, true
}

func persistedModelRuntimeEmpty(e persistedModelState) bool {
	return e.Status == "" &&
		strings.TrimSpace(e.StatusMessage) == "" &&
		!e.Unavailable &&
		strings.TrimSpace(e.NextRetryAfter) == "" &&
		e.Quota == nil
}

func toPersistedQuotaState(quota QuotaState, now time.Time) *persistedQuotaState {
	if !quota.Exceeded && quota.BackoffLevel == 0 && strings.TrimSpace(quota.Reason) == "" && quota.NextRecoverAt.IsZero() {
		return nil
	}
	state := &persistedQuotaState{
		Exceeded:     quota.Exceeded,
		Reason:       strings.TrimSpace(quota.Reason),
		BackoffLevel: quota.BackoffLevel,
	}
	if quota.NextRecoverAt.After(now) {
		state.NextRecoverAt = quota.NextRecoverAt.UTC().Format(time.RFC3339Nano)
	}
	if !state.Exceeded && state.Reason == "" && state.BackoffLevel == 0 && state.NextRecoverAt == "" {
		return nil
	}
	return state
}

func fromPersistedQuotaState(state *persistedQuotaState) *QuotaState {
	if state == nil {
		return nil
	}
	quota := &QuotaState{
		Exceeded:     state.Exceeded,
		Reason:       strings.TrimSpace(state.Reason),
		BackoffLevel: state.BackoffLevel,
	}
	if next := parsePersistedTime(state.NextRecoverAt); !next.IsZero() {
		quota.NextRecoverAt = next
	}
	if !quota.Exceeded && quota.Reason == "" && quota.BackoffLevel == 0 && quota.NextRecoverAt.IsZero() {
		return nil
	}
	return quota
}

func parsePersistedTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed.UTC()
	}
	parsed, err = time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func normalizePersistedStatus(next, fallback Status) Status {
	if strings.TrimSpace(string(next)) != "" {
		return next
	}
	if strings.TrimSpace(string(fallback)) != "" {
		return fallback
	}
	return StatusError
}

func pickPersistedMessage(next, fallback string) string {
	next = strings.TrimSpace(next)
	if next != "" {
		return next
	}
	return strings.TrimSpace(fallback)
}
