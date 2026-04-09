package auth

import (
	"strings"
	"time"
)

const (
	AuthMaintenanceAutoRecoveryMetadataKey            = "auth_maintenance_auto_recover"
	AuthMaintenanceAutoRecoveryReasonMetadataKey      = "auth_maintenance_auto_recover_reason"
	AuthMaintenanceAutoRecoveryDisabledAtMetadataKey  = "auth_maintenance_auto_recover_disabled_at"
	AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey = "auth_maintenance_auto_recover_next_check_at"
)

func MarkAuthMaintenanceAutoRecovery(metadata map[string]any, reason string, now time.Time) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata[AuthMaintenanceAutoRecoveryMetadataKey] = true
	reason = strings.TrimSpace(reason)
	if reason != "" {
		metadata[AuthMaintenanceAutoRecoveryReasonMetadataKey] = reason
	} else {
		delete(metadata, AuthMaintenanceAutoRecoveryReasonMetadataKey)
	}
	if !now.IsZero() {
		metadata[AuthMaintenanceAutoRecoveryDisabledAtMetadataKey] = now.UTC().Format(time.RFC3339Nano)
	}
	delete(metadata, AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey)
	return metadata
}

func ClearAuthMaintenanceAutoRecovery(metadata map[string]any) {
	if metadata == nil {
		return
	}
	delete(metadata, AuthMaintenanceAutoRecoveryMetadataKey)
	delete(metadata, AuthMaintenanceAutoRecoveryReasonMetadataKey)
	delete(metadata, AuthMaintenanceAutoRecoveryDisabledAtMetadataKey)
	delete(metadata, AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey)
}

func SetAuthMaintenanceAutoRecoveryNextCheckAt(metadata map[string]any, next time.Time) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if next.IsZero() {
		delete(metadata, AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey)
		return metadata
	}
	metadata[AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey] = next.UTC().Format(time.RFC3339Nano)
	return metadata
}

func IsAuthMaintenanceAutoRecoverable(auth *Auth) bool {
	if auth == nil || auth.Metadata == nil {
		return false
	}
	return authMaintenanceMetadataBool(auth.Metadata[AuthMaintenanceAutoRecoveryMetadataKey])
}

func AuthMaintenanceAutoRecoveryReason(auth *Auth) string {
	if auth == nil || auth.Metadata == nil {
		return ""
	}
	value, _ := auth.Metadata[AuthMaintenanceAutoRecoveryReasonMetadataKey].(string)
	return strings.TrimSpace(value)
}

func AuthMaintenanceAutoRecoveryNextCheckAt(auth *Auth) (time.Time, bool) {
	if auth == nil || auth.Metadata == nil {
		return time.Time{}, false
	}
	if next, ok := parseTimeValue(auth.Metadata[AuthMaintenanceAutoRecoveryNextCheckAtMetadataKey]); ok {
		return next.UTC(), true
	}
	return time.Time{}, false
}

func authMaintenanceMetadataBool(raw any) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}
