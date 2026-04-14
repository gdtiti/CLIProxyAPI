package auth

import (
	"net/http"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

type testConfigAwareRoundTripperProvider struct {
	lastCfg *internalconfig.Config
	calls   int
}

func (p *testConfigAwareRoundTripperProvider) RoundTripperFor(*Auth) http.RoundTripper {
	return nil
}

func (p *testConfigAwareRoundTripperProvider) SetConfig(cfg *internalconfig.Config) {
	p.lastCfg = cfg
	p.calls++
}

func TestManagerSetRoundTripperProviderReceivesCurrentConfig(t *testing.T) {
	t.Parallel()

	manager := NewManager(nil, nil, nil)
	manager.SetConfig(&internalconfig.Config{
		SDKConfig: internalconfig.SDKConfig{
			ProxyURL: "direct",
		},
	})

	provider := &testConfigAwareRoundTripperProvider{}
	manager.SetRoundTripperProvider(provider)

	if provider.calls != 1 {
		t.Fatalf("SetConfig calls = %d, want 1", provider.calls)
	}
	if provider.lastCfg == nil || provider.lastCfg.ProxyURL != "direct" {
		t.Fatalf("provider last config = %#v, want ProxyURL=direct", provider.lastCfg)
	}
}

func TestManagerSetConfigPropagatesToRoundTripperProvider(t *testing.T) {
	t.Parallel()

	manager := NewManager(nil, nil, nil)
	provider := &testConfigAwareRoundTripperProvider{}
	manager.SetRoundTripperProvider(provider)

	cfg := &internalconfig.Config{
		SDKConfig: internalconfig.SDKConfig{
			IgnoreAuthFileProxyURL: true,
			ProxyURL:               "direct",
		},
	}
	manager.SetConfig(cfg)

	if provider.calls != 2 {
		t.Fatalf("SetConfig calls = %d, want 2", provider.calls)
	}
	if provider.lastCfg != cfg {
		t.Fatal("expected provider to receive the latest config pointer from manager")
	}
}
