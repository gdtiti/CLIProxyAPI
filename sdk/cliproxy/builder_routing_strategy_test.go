package cliproxy

import (
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestBuilderBuild_SelectsFeedbackRoutingStrategies(t *testing.T) {
	tests := []struct {
		name       string
		routing    internalconfig.RoutingConfig
		assertType func(t *testing.T, selector coreauth.Selector)
	}{
		{
			name: "success-rate",
			routing: internalconfig.RoutingConfig{
				Strategy: "success-rate",
				SuccessRate: internalconfig.RoutingSuccessRateConfig{
					HalfLifeSeconds: 900,
					ExploreRate:     0.15,
				},
			},
			assertType: func(t *testing.T, selector coreauth.Selector) {
				t.Helper()
				if _, ok := selector.(*coreauth.SuccessRateSelector); !ok {
					t.Fatalf("selector = %T, want *SuccessRateSelector", selector)
				}
			},
		},
		{
			name: "simhash",
			routing: internalconfig.RoutingConfig{
				Strategy: "simhash",
				SimHash: internalconfig.RoutingSimHashConfig{
					PoolSize:             3,
					AdmitCooldownSeconds: 9,
				},
			},
			assertType: func(t *testing.T, selector coreauth.Selector) {
				t.Helper()
				if _, ok := selector.(*coreauth.SimHashSelector); !ok {
					t.Fatalf("selector = %T, want *SimHashSelector", selector)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &sdkconfig.Config{
				AuthDir: t.TempDir(),
				Routing: tc.routing,
			}
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			svc, err := NewBuilder().
				WithConfig(cfg).
				WithConfigPath(configPath).
				Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if svc == nil || svc.coreManager == nil {
				t.Fatalf("Build() returned nil service/coreManager")
			}
			tc.assertType(t, svc.coreManager.Selector())
		})
	}
}

func TestBuilderBuild_WiresRequestAuditHookWhenEnabled(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	cfg := &sdkconfig.Config{
		AuthDir: t.TempDir(),
		SDKConfig: internalconfig.SDKConfig{
			RequestAudit: internalconfig.RequestAuditConfig{
				Enable:   true,
				Endpoint: server.URL,
			},
		},
	}

	svc, err := NewBuilder().
		WithConfig(cfg).
		WithConfigPath(filepath.Join(t.TempDir(), "config.yaml")).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if svc == nil || svc.coreManager == nil {
		t.Fatalf("Build() returned nil service/coreManager")
	}
	hook := svc.coreManager.Hook()
	if _, ok := hook.(*requestAuditHook); ok {
		return
	}
	if !strings.Contains(fmt.Sprintf("%#v", hook), "requestAuditHook") {
		t.Fatalf("coreManager.Hook() = %T, want hook chain containing *requestAuditHook", hook)
	}
}
