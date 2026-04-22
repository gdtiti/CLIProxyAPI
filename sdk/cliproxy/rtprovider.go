package cliproxy

import (
	"net/http"
	"strings"
	"sync"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor/helps"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/proxyutil"
	log "github.com/sirupsen/logrus"
)

// defaultRoundTripperProvider returns a per-auth HTTP RoundTripper based on
// the Auth.ProxyURL value. It caches transports per proxy URL string.
type defaultRoundTripperProvider struct {
	mu    sync.RWMutex
	cfg   *internalconfig.Config
	cache map[string]http.RoundTripper
}

func newDefaultRoundTripperProvider(cfg *internalconfig.Config) *defaultRoundTripperProvider {
	return &defaultRoundTripperProvider{
		cfg:   cfg,
		cache: make(map[string]http.RoundTripper),
	}
}

func (p *defaultRoundTripperProvider) SetConfig(cfg *internalconfig.Config) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = cfg
}

func (p *defaultRoundTripperProvider) resolveProxy(auth *coreauth.Auth) (string, bool) {
	if auth == nil {
		return "", false
	}
	p.mu.RLock()
	cfg := p.cfg
	p.mu.RUnlock()
	if helps.ShouldIgnoreAuthFileProxyURL(cfg, auth) {
		if cfg == nil {
			return "", true
		}
		return strings.TrimSpace(cfg.ProxyURL), true
	}
	if proxyStr := strings.TrimSpace(auth.ProxyURL); proxyStr != "" {
		return proxyStr, false
	}
	if cfg != nil {
		return strings.TrimSpace(cfg.ProxyURL), false
	}
	return "", false
}

// RoundTripperFor implements coreauth.RoundTripperProvider.
func (p *defaultRoundTripperProvider) RoundTripperFor(auth *coreauth.Auth) http.RoundTripper {
	if auth == nil {
		return nil
	}
	proxyStr, forceDirect := p.resolveProxy(auth)
	if proxyStr == "" {
		if forceDirect {
			return proxyutil.NewDirectTransport()
		}
		return nil
	}
	p.mu.RLock()
	rt := p.cache[proxyStr]
	p.mu.RUnlock()
	if rt != nil {
		return rt
	}
	transport, _, errBuild := proxyutil.BuildHTTPTransport(proxyStr)
	if errBuild != nil {
		log.Errorf("%v", errBuild)
		return nil
	}
	if transport == nil {
		return nil
	}
	p.mu.Lock()
	p.cache[proxyStr] = transport
	p.mu.Unlock()
	return transport
}
