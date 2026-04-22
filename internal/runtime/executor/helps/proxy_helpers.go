package helps

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/proxyutil"
	log "github.com/sirupsen/logrus"
)

// httpClientCache caches HTTP clients by proxy URL to enable connection reuse
var (
	httpClientCache      = make(map[string]*http.Client)
	httpClientCacheMutex sync.RWMutex
)

// ResolveAuthProxyURL returns the auth-level proxy override after applying config-level ignore rules.
func ResolveAuthProxyURL(cfg *config.Config, auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}
	if cfg != nil && cfg.IgnoreAuthFileProxyURL && isFileBackedAuth(auth) {
		return ""
	}
	return strings.TrimSpace(auth.ProxyURL)
}

// ResolveProxyURL returns the effective proxy URL using auth override first, then global config.
func ResolveProxyURL(cfg *config.Config, auth *cliproxyauth.Auth) string {
	if proxyURL := ResolveAuthProxyURL(cfg, auth); proxyURL != "" {
		return proxyURL
	}
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.ProxyURL)
}

func authFileProxyIgnored(cfg *config.Config, auth *cliproxyauth.Auth) bool {
	return cfg != nil && cfg.IgnoreAuthFileProxyURL && isFileBackedAuth(auth)
}

func isFileBackedAuth(auth *cliproxyauth.Auth) bool {
	if auth == nil {
		return false
	}
	if strings.TrimSpace(auth.FileName) != "" {
		return true
	}
	if auth.Attributes != nil {
		if isLikelyAuthFileReference(auth.Attributes["path"]) {
			return true
		}
		if isLikelyAuthFileReference(auth.Attributes["source"]) {
			return true
		}
	}
	return isLikelyAuthFileReference(auth.ID)
}

func isLikelyAuthFileReference(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
	return strings.HasSuffix(normalized, ".json")
}

// NewProxyAwareHTTPClient creates an HTTP client with proper proxy configuration priority:
//  1. Use auth.ProxyURL if configured (highest priority), unless ignore-auth-file-proxy-url
//     is enabled for a file-backed auth record
//  2. Use cfg.ProxyURL if auth proxy is not configured
//  3. Use RoundTripper from context if neither are configured and auth-file proxy ignore is not active
//
// This function caches HTTP clients by proxy URL to enable TCP/TLS connection reuse.
//
// Parameters:
//   - ctx: The context containing optional RoundTripper
//   - cfg: The application configuration
//   - auth: The authentication information
//   - timeout: The client timeout (0 means no timeout)
//
// Returns:
//   - *http.Client: An HTTP client with configured proxy or transport
func NewProxyAwareHTTPClient(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth, timeout time.Duration) *http.Client {
	proxyURL := ResolveProxyURL(cfg, auth)
	ignoredAuthFileProxy := ShouldIgnoreAuthFileProxyURL(cfg, auth)

	// If we have a proxy URL configured, try cache first to reuse TCP/TLS connections.
	if proxyURL != "" {
		httpClientCacheMutex.RLock()
		if cachedClient, ok := httpClientCache[proxyURL]; ok {
			httpClientCacheMutex.RUnlock()
			if timeout > 0 {
				return &http.Client{Transport: cachedClient.Transport, Timeout: timeout}
			}
			return cachedClient
		}
		httpClientCacheMutex.RUnlock()
	}

	// Create new client
	httpClient := &http.Client{}
	if timeout > 0 {
		httpClient.Timeout = timeout
	}

	// If we have a proxy URL configured, set up the transport
	if proxyURL != "" {
		transport := buildProxyTransport(proxyURL)
		if transport != nil {
			httpClient.Transport = transport
			// Cache the client
			httpClientCacheMutex.Lock()
			httpClientCache[proxyURL] = httpClient
			httpClientCacheMutex.Unlock()
			return httpClient
		}
		// If proxy setup failed, log and fall through to context RoundTripper
		log.Debugf("failed to setup proxy from URL: %s, falling back to context transport", proxyURL)
	}

	// When file-backed auth proxy overrides are ignored, never fall back to the context
	// round tripper because it may still have been constructed from auth.ProxyURL.
	if ignoredAuthFileProxy {
		transport, ok := http.DefaultTransport.(*http.Transport)
		if !ok || transport == nil {
			httpClient.Transport = &http.Transport{Proxy: nil}
			return httpClient
		}
		clone := transport.Clone()
		clone.Proxy = nil
		httpClient.Transport = clone
		return httpClient
	}

	// Priority 3: Use RoundTripper from context (typically from RoundTripperFor)
	if rt, ok := ctx.Value("cliproxy.roundtripper").(http.RoundTripper); ok && rt != nil {
		httpClient.Transport = rt
	}

	return httpClient
}

// ShouldIgnoreAuthFileProxyURL reports whether the auth-specific proxy_url should be ignored.
func ShouldIgnoreAuthFileProxyURL(cfg *config.Config, auth *cliproxyauth.Auth) bool {
	return authFileProxyIgnored(cfg, auth)
}

// buildProxyTransport creates an HTTP transport configured for the given proxy URL.
// It supports SOCKS5, HTTP, and HTTPS proxy protocols.
//
// Parameters:
//   - proxyURL: The proxy URL string (e.g., "socks5://user:pass@host:port", "http://host:port")
//
// Returns:
//   - *http.Transport: A configured transport, or nil if the proxy URL is invalid
func buildProxyTransport(proxyURL string) *http.Transport {
	transport, _, errBuild := proxyutil.BuildHTTPTransport(proxyURL)
	if errBuild != nil {
		log.Errorf("%v", errBuild)
		return nil
	}
	return transport
}
