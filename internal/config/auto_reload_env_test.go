package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigOptional_AutoReloadIntervalEnvOverride(t *testing.T) {
	authDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "auth-dir: " + strings.ReplaceAll(authDir, "\\", "/") + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	tests := []struct {
		name      string
		envValue  string
		wantValue int
		wantErr   string
	}{
		{name: "unset keeps default", envValue: "", wantValue: 0},
		{name: "zero disables auto reload", envValue: "0", wantValue: 0},
		{name: "negative disables auto reload", envValue: "-3", wantValue: -3},
		{name: "positive enables auto reload", envValue: "15", wantValue: 15},
		{name: "invalid value fails", envValue: "abc", wantErr: EnvAutoReloadIntervalSeconds},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue == "" {
				t.Setenv(EnvAutoReloadIntervalSeconds, "")
			} else {
				t.Setenv(EnvAutoReloadIntervalSeconds, tc.envValue)
			}

			cfg, err := LoadConfigOptional(configPath, false)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfigOptional() error = %v", err)
			}
			if cfg.AutoReloadIntervalSeconds != tc.wantValue {
				t.Fatalf("AutoReloadIntervalSeconds = %d, want %d", cfg.AutoReloadIntervalSeconds, tc.wantValue)
			}
		})
	}
}
