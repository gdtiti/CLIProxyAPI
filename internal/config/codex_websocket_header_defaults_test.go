package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_CodexHeaderDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
codex-header-defaults:
  user-agent: "  my-codex-client/1.0  "
  beta-features: "  feature-a,feature-b  "
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if got := cfg.CodexHeaderDefaults.UserAgent; got != "my-codex-client/1.0" {
		t.Fatalf("UserAgent = %q, want %q", got, "my-codex-client/1.0")
	}
	if got := cfg.CodexHeaderDefaults.BetaFeatures; got != "feature-a,feature-b" {
		t.Fatalf("BetaFeatures = %q, want %q", got, "feature-a,feature-b")
	}
}

func TestLoadConfigOptional_CodexMimicMode(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
codex-mimic:
  mode: "  SAFE  "
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if got := cfg.CodexMimicMode(); got != CodexMimicModeSafe {
		t.Fatalf("CodexMimicMode() = %q, want %q", got, CodexMimicModeSafe)
	}
}

func TestLoadConfigOptional_CodexMimicStrictFlags(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
codex-mimic:
  mode: strict
  strict:
    force-turn-metadata: true
    force-turn-state: true
    include-timing-metrics: true
    stable-request-id: true
    stable-turn-id: true
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if !cfg.CodexMimic.Strict.ForceTurnMetadata {
		t.Fatal("ForceTurnMetadata = false, want true")
	}
	if !cfg.CodexMimic.Strict.ForceTurnState {
		t.Fatal("ForceTurnState = false, want true")
	}
	if !cfg.CodexMimic.Strict.IncludeTimingMetrics {
		t.Fatal("IncludeTimingMetrics = false, want true")
	}
	if !cfg.CodexMimic.Strict.StableRequestID {
		t.Fatal("StableRequestID = false, want true")
	}
	if !cfg.CodexMimic.Strict.StableTurnID {
		t.Fatal("StableTurnID = false, want true")
	}
}
