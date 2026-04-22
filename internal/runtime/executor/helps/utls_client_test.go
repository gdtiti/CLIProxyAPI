package helps

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestNewUtlsHTTPClientIgnoreAuthFileProxyURLFallsBackToGlobalProxy(t *testing.T) {
	t.Parallel()

	client := NewUtlsHTTPClient(&sdkconfig.Config{SDKConfig: sdkconfig.SDKConfig{
		ProxyURL:               "http://global-proxy.example.com:8080",
		IgnoreAuthFileProxyURL: true,
	}}, &cliproxyauth.Auth{
		FileName: "claude-auth.json",
		ProxyURL: "http://127.0.0.1:1",
	}, 0)

	fallback, ok := client.Transport.(*fallbackRoundTripper)
	if !ok {
		t.Fatalf("client.Transport type = %T, want *fallbackRoundTripper", client.Transport)
	}
	httpTransport, ok := fallback.fallback.(*http.Transport)
	if !ok {
		t.Fatalf("fallback transport type = %T, want *http.Transport", fallback.fallback)
	}
	if httpTransport.Proxy == nil {
		t.Fatal("expected fallback transport proxy function to be configured")
	}

	req := httptest.NewRequest(http.MethodGet, "https://api.anthropic.com/v1/messages", nil)
	proxyURL, err := httpTransport.Proxy(req)
	if err != nil {
		t.Fatalf("httpTransport.Proxy() error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://global-proxy.example.com:8080" {
		t.Fatalf("proxy URL = %v, want http://global-proxy.example.com:8080", proxyURL)
	}
}
