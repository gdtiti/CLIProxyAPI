package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_AuthMaintenanceDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8080\nauth-dir: auth\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if !cfg.AuthMaintenance.Enable {
		t.Fatalf("AuthMaintenance.Enable = false, want true")
	}
	if !cfg.AuthMaintenance.DisableCodexUsageLimitReached {
		t.Fatalf("DisableCodexUsageLimitReached = false, want true")
	}
	if len(cfg.AuthMaintenance.DisableStatusCodes) != 0 {
		t.Fatalf("DisableStatusCodes = %#v, want empty", cfg.AuthMaintenance.DisableStatusCodes)
	}
	if cfg.AuthMaintenance.CodexMaxRequestCount != 0 {
		t.Fatalf("CodexMaxRequestCount = %d, want 0", cfg.AuthMaintenance.CodexMaxRequestCount)
	}
	if cfg.AuthMaintenance.ScanIntervalSeconds != 0 {
		t.Fatalf("ScanIntervalSeconds = %d, want 0 before runtime normalization", cfg.AuthMaintenance.ScanIntervalSeconds)
	}
}

func TestLoadConfigOptional_AuthMaintenanceExplicitConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	data := []byte("auth-maintenance:\n  enable: false\n  scan-interval-seconds: 7\n  delete-interval-seconds: 3\n  delete-status-codes: [402, 429]\n  disable-status-codes: [403, 500]\n  delete-quota-exceeded: true\n  quota-strike-threshold: 5\n  disable-codex-usage-limit-reached: false\n  codex-max-request-count: 12\n")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if cfg.AuthMaintenance.Enable {
		t.Fatalf("AuthMaintenance.Enable = true, want false")
	}
	if cfg.AuthMaintenance.ScanIntervalSeconds != 7 {
		t.Fatalf("ScanIntervalSeconds = %d, want 7", cfg.AuthMaintenance.ScanIntervalSeconds)
	}
	if cfg.AuthMaintenance.DeleteIntervalSeconds != 3 {
		t.Fatalf("DeleteIntervalSeconds = %d, want 3", cfg.AuthMaintenance.DeleteIntervalSeconds)
	}
	if len(cfg.AuthMaintenance.DeleteStatusCodes) != 2 || cfg.AuthMaintenance.DeleteStatusCodes[0] != 402 || cfg.AuthMaintenance.DeleteStatusCodes[1] != 429 {
		t.Fatalf("DeleteStatusCodes = %#v, want [402 429]", cfg.AuthMaintenance.DeleteStatusCodes)
	}
	if len(cfg.AuthMaintenance.DisableStatusCodes) != 2 || cfg.AuthMaintenance.DisableStatusCodes[0] != 403 || cfg.AuthMaintenance.DisableStatusCodes[1] != 500 {
		t.Fatalf("DisableStatusCodes = %#v, want [403 500]", cfg.AuthMaintenance.DisableStatusCodes)
	}
	if !cfg.AuthMaintenance.DeleteQuotaExceeded {
		t.Fatalf("DeleteQuotaExceeded = false, want true")
	}
	if cfg.AuthMaintenance.QuotaStrikeThreshold != 5 {
		t.Fatalf("QuotaStrikeThreshold = %d, want 5", cfg.AuthMaintenance.QuotaStrikeThreshold)
	}
	if cfg.AuthMaintenance.DisableCodexUsageLimitReached {
		t.Fatalf("DisableCodexUsageLimitReached = true, want false")
	}
	if cfg.AuthMaintenance.CodexMaxRequestCount != 12 {
		t.Fatalf("CodexMaxRequestCount = %d, want 12", cfg.AuthMaintenance.CodexMaxRequestCount)
	}
}
