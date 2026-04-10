package helps

import (
	"context"
	"net/http"
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
