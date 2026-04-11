package cliproxy

import (
	"net/http"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestRoundTripperForDirectBypassesProxy(t *testing.T) {
	t.Parallel()

	provider := newDefaultRoundTripperProvider(nil)
	rt := provider.RoundTripperFor(&coreauth.Auth{ProxyURL: "direct"})
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
	}
	if transport.Proxy != nil {
		t.Fatal("expected direct transport to disable proxy function")
	}
}

func TestRoundTripperForIgnoreAuthFileProxyFallsBackToGlobalProxy(t *testing.T) {
	t.Parallel()

	provider := newDefaultRoundTripperProvider(&internalconfig.Config{
		SDKConfig: internalconfig.SDKConfig{
			ProxyURL:               "http://global-proxy.example.com:8080",
			IgnoreAuthFileProxyURL: true,
		},
	})

	rt := provider.RoundTripperFor(&coreauth.Auth{
		FileName: "auths/codex.json",
		ProxyURL: "http://file-proxy.example.com:8080",
	})
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
	}
	if transport.Proxy == nil {
		t.Fatal("expected global proxy transport")
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("transport.Proxy() error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://global-proxy.example.com:8080" {
		t.Fatalf("proxy URL = %v, want http://global-proxy.example.com:8080", proxyURL)
	}
}

func TestRoundTripperForIgnoreAuthFileProxyUsesDirectWhenGlobalProxyMissing(t *testing.T) {
	t.Parallel()

	provider := newDefaultRoundTripperProvider(&internalconfig.Config{
		SDKConfig: internalconfig.SDKConfig{
			IgnoreAuthFileProxyURL: true,
		},
	})

	rt := provider.RoundTripperFor(&coreauth.Auth{
		FileName: "auths/codex.json",
		ProxyURL: "http://file-proxy.example.com:8080",
	})
	transport, ok := rt.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", rt)
	}
	if transport.Proxy != nil {
		t.Fatal("expected direct transport when auth-file proxy is ignored and no global proxy is configured")
	}
}
