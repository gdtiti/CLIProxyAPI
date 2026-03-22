package auth

import (
	"strings"
	"time"
)

const (
	temporaryDisableUntilMetadataKey  = "temporary_disable_until"
	temporaryDisableReasonMetadataKey = "temporary_disable_reason"

	temporaryDisableReasonCodex429 = "codex_429"

	codex429TemporaryDisableDuration = 5 * time.Hour
)

func setTemporaryDisable(auth *Auth, until time.Time, reason, message string) {
	if auth == nil || until.IsZero() {
		return
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata[temporaryDisableUntilMetadataKey] = until.UTC().Format(time.RFC3339Nano)
	auth.Metadata[temporaryDisableReasonMetadataKey] = strings.TrimSpace(reason)
	auth.Status = StatusDisabled
	if strings.TrimSpace(message) != "" {
		auth.StatusMessage = strings.TrimSpace(message)
	}
	auth.Unavailable = true
	auth.NextRetryAfter = until
}

func temporaryDisableUntil(auth *Auth) time.Time {
	if auth == nil || auth.Metadata == nil {
		return time.Time{}
	}
	raw, _ := auth.Metadata[temporaryDisableUntilMetadataKey].(string)
	return parsePersistedTime(raw)
}

func temporaryDisableActive(auth *Auth, now time.Time) (time.Time, bool) {
	until := temporaryDisableUntil(auth)
	if until.IsZero() || !until.After(now) {
		return time.Time{}, false
	}
	return until, true
}

func temporaryDisableExpired(auth *Auth, now time.Time) bool {
	until := temporaryDisableUntil(auth)
	return !until.IsZero() && !until.After(now)
}

func normalizeTemporaryDisableSnapshot(auth *Auth, now time.Time) {
	if auth == nil || auth.Disabled || !temporaryDisableExpired(auth, now) {
		return
	}
	delete(auth.Metadata, temporaryDisableUntilMetadataKey)
	delete(auth.Metadata, temporaryDisableReasonMetadataKey)
	if auth.Status == StatusDisabled {
		auth.Status = StatusActive
	}
	auth.StatusMessage = ""
	if !auth.NextRetryAfter.IsZero() && !auth.NextRetryAfter.After(now) {
		auth.NextRetryAfter = time.Time{}
	}
	auth.Unavailable = false
	auth.Quota = QuotaState{}
	updateAggregatedAvailability(auth, now)
}
