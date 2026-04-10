package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_RoutingDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 8080\nauth-dir: auth\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if cfg.Routing.SuccessRate.HalfLifeSeconds != 1800 {
		t.Fatalf("HalfLifeSeconds = %d, want 1800", cfg.Routing.SuccessRate.HalfLifeSeconds)
	}
	if cfg.Routing.SuccessRate.ExploreRate != 0.02 {
		t.Fatalf("ExploreRate = %v, want 0.02", cfg.Routing.SuccessRate.ExploreRate)
	}
	if cfg.Routing.SimHash.PoolSize != 10 {
		t.Fatalf("PoolSize = %d, want 10", cfg.Routing.SimHash.PoolSize)
	}
	if cfg.Routing.SimHash.AdmitCooldownSeconds != 1 {
		t.Fatalf("AdmitCooldownSeconds = %d, want 1", cfg.Routing.SimHash.AdmitCooldownSeconds)
	}
}

func TestLoadConfigOptional_RoutingExplicitConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	data := []byte("" +
		"routing:\n" +
		"  strategy: simhash\n" +
		"  success-rate:\n" +
		"    half-life-seconds: 7200\n" +
		"    explore-rate: 0.3\n" +
		"  simhash:\n" +
		"    pool-size: 6\n" +
		"    admit-cooldown-seconds: 4\n")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if cfg.Routing.Strategy != "simhash" {
		t.Fatalf("Strategy = %q, want simhash", cfg.Routing.Strategy)
	}
	if cfg.Routing.SuccessRate.HalfLifeSeconds != 7200 {
		t.Fatalf("HalfLifeSeconds = %d, want 7200", cfg.Routing.SuccessRate.HalfLifeSeconds)
	}
	if cfg.Routing.SuccessRate.ExploreRate != 0.3 {
		t.Fatalf("ExploreRate = %v, want 0.3", cfg.Routing.SuccessRate.ExploreRate)
	}
	if cfg.Routing.SimHash.PoolSize != 6 {
		t.Fatalf("PoolSize = %d, want 6", cfg.Routing.SimHash.PoolSize)
	}
	if cfg.Routing.SimHash.AdmitCooldownSeconds != 4 {
		t.Fatalf("AdmitCooldownSeconds = %d, want 4", cfg.Routing.SimHash.AdmitCooldownSeconds)
	}
}
