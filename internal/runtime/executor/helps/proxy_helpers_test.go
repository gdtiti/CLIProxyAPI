package helps

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestNewProxyAwareHTTPClientDirectBypassesGlobalProxy(t *testing.T) {
	t.Parallel()

	client := NewProxyAwareHTTPClient(
		context.Background(),
		&config.Config{SDKConfig: sdkconfig.SDKConfig{ProxyURL: "http://global-proxy.example.com:8080"}},
		&cliproxyauth.Auth{ProxyURL: "direct"},
		0,
	)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("expected direct transport to disable proxy function")
	}
}

func TestResolveAuthProxyURLIgnoresFileBackedProxyWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SDKConfig: sdkconfig.SDKConfig{
			IgnoreAuthFileProxyURL: true,
		},
	}
	auth := &cliproxyauth.Auth{
		FileName: "auths/codex.json",
		ProxyURL: "http://file-proxy.example.com:8080",
	}

	if got := ResolveAuthProxyURL(cfg, auth); got != "" {
		t.Fatalf("ResolveAuthProxyURL() = %q, want empty", got)
	}
}

func TestResolveAuthProxyURLIgnoresPathBackedProxyWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SDKConfig: sdkconfig.SDKConfig{
			IgnoreAuthFileProxyURL: true,
		},
	}
	auth := &cliproxyauth.Auth{
		ProxyURL: "http://file-proxy.example.com:8080",
		Attributes: map[string]string{
			"path": "auths/codex.json",
		},
	}

	if got := ResolveAuthProxyURL(cfg, auth); got != "" {
		t.Fatalf("ResolveAuthProxyURL() = %q, want empty", got)
	}
}

func TestResolveAuthProxyURLKeepsConfigBackedProxyWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SDKConfig: sdkconfig.SDKConfig{
			IgnoreAuthFileProxyURL: true,
		},
	}
	auth := &cliproxyauth.Auth{
		ProxyURL: "http://config-proxy.example.com:8080",
	}

	if got := ResolveAuthProxyURL(cfg, auth); got != "http://config-proxy.example.com:8080" {
		t.Fatalf("ResolveAuthProxyURL() = %q, want %q", got, "http://config-proxy.example.com:8080")
	}
}

func TestResolveProxyURLFallsBackToGlobalWhenFileProxyIgnored(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SDKConfig: sdkconfig.SDKConfig{
			ProxyURL:               "http://global-proxy.example.com:8080",
			IgnoreAuthFileProxyURL: true,
		},
	}
	auth := &cliproxyauth.Auth{
		FileName: "auths/codex.json",
		ProxyURL: "http://file-proxy.example.com:8080",
	}

	if got := ResolveProxyURL(cfg, auth); got != "http://global-proxy.example.com:8080" {
		t.Fatalf("ResolveProxyURL() = %q, want %q", got, "http://global-proxy.example.com:8080")
	}
}

func TestNewProxyAwareHTTPClientIgnoreAuthFileProxyURLFallsBackToGlobalProxy(t *testing.T) {
	t.Parallel()

	client := NewProxyAwareHTTPClient(
		context.Background(),
		&config.Config{SDKConfig: sdkconfig.SDKConfig{
			ProxyURL:               "http://global-proxy.example.com:8080",
			IgnoreAuthFileProxyURL: true,
		}},
		&cliproxyauth.Auth{
			FileName: "auths/codex-auth.json",
			ProxyURL: "http://127.0.0.1:1",
		},
		0,
	)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}

	req, errRequest := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errRequest != nil {
		t.Fatalf("http.NewRequest returned error: %v", errRequest)
	}

	proxyURL, errProxy := transport.Proxy(req)
	if errProxy != nil {
		t.Fatalf("transport.Proxy returned error: %v", errProxy)
	}
	if proxyURL == nil || proxyURL.String() != "http://global-proxy.example.com:8080" {
		t.Fatalf("proxy URL = %v, want http://global-proxy.example.com:8080", proxyURL)
	}
}

func TestShouldIgnoreAuthFileProxyURLDoesNotAffectNonFileAuth(t *testing.T) {
	t.Parallel()

	client := NewProxyAwareHTTPClient(
		context.Background(),
		&config.Config{SDKConfig: sdkconfig.SDKConfig{
			ProxyURL:               "http://global-proxy.example.com:8080",
			IgnoreAuthFileProxyURL: true,
		}},
		&cliproxyauth.Auth{ProxyURL: "http://auth-proxy.example.com:8080"},
		0,
	)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}

	req, errRequest := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errRequest != nil {
		t.Fatalf("http.NewRequest returned error: %v", errRequest)
	}

	proxyURL, errProxy := transport.Proxy(req)
	if errProxy != nil {
		t.Fatalf("transport.Proxy returned error: %v", errProxy)
	}
	if proxyURL == nil || proxyURL.String() != "http://auth-proxy.example.com:8080" {
		t.Fatalf("proxy URL = %v, want http://auth-proxy.example.com:8080", proxyURL)
	}
}

func TestNewProxyAwareHTTPClientIgnoreFileProxySkipsContextRoundTripper(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		SDKConfig: sdkconfig.SDKConfig{
			IgnoreAuthFileProxyURL: true,
		},
	}
	auth := &cliproxyauth.Auth{
		FileName: "auths/codex.json",
		ProxyURL: "http://file-proxy.example.com:8080",
	}

	contextTransport := &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) {
			return url.Parse("http://context-proxy.example.com:8080")
		},
	}
	ctx := context.WithValue(context.Background(), "cliproxy.roundtripper", http.RoundTripper(contextTransport))

	client := NewProxyAwareHTTPClient(ctx, cfg, auth, 0)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy == nil {
		return
	}

	req, errRequest := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errRequest != nil {
		t.Fatalf("http.NewRequest returned error: %v", errRequest)
	}
	proxyURL, errProxy := transport.Proxy(req)
	if errProxy != nil {
		t.Fatalf("transport.Proxy returned error: %v", errProxy)
	}
	if proxyURL != nil {
		t.Fatalf("proxy URL = %v, want nil", proxyURL)
	}
}
