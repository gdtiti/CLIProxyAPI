package cliproxy

import (
	"net/http"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestRoundTripperForDirectBypassesProxy(t *testing.T) {
	t.Parallel()

	provider := newDefaultRoundTripperProvider()
	rt := provider.RoundTripperFor(&coreauth.Auth{ProxyURL: "direct"})
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
	}
	if transport.Proxy != nil {
		t.Fatal("expected direct transport to disable proxy function")
	}
}

func TestRoundTripperForIgnoreAuthFileProxyURLFallsBackToGlobalProxy(t *testing.T) {
	t.Parallel()

	provider := newDefaultRoundTripperProvider()
	provider.SetConfig(&internalconfig.Config{
		SDKConfig: internalconfig.SDKConfig{
			IgnoreAuthFileProxyURL: true,
			ProxyURL:               "direct",
		},
	})

	rt := provider.RoundTripperFor(&coreauth.Auth{
		FileName: "auths/codex.json",
		ProxyURL: "http://127.0.0.1:8080",
	})
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
	}
	if transport.Proxy != nil {
		t.Fatal("expected global direct proxy to win when auth-file proxy is ignored")
	}
}
