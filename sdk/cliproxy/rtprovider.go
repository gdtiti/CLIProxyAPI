package cliproxy

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

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
	cache map[string]http.RoundTripper
	cfg   atomic.Value
}

func newDefaultRoundTripperProvider() *defaultRoundTripperProvider {
	provider := &defaultRoundTripperProvider{cache: make(map[string]http.RoundTripper)}
	provider.cfg.Store(&internalconfig.Config{})
	return provider
}

func (p *defaultRoundTripperProvider) SetConfig(cfg *internalconfig.Config) {
	if p == nil {
		return
	}
	if cfg == nil {
		cfg = &internalconfig.Config{}
	}
	p.cfg.Store(cfg)
}

// RoundTripperFor implements coreauth.RoundTripperProvider.
func (p *defaultRoundTripperProvider) RoundTripperFor(auth *coreauth.Auth) http.RoundTripper {
	if auth == nil {
		return nil
	}
	cfg, _ := p.cfg.Load().(*internalconfig.Config)
	if cfg == nil {
		cfg = &internalconfig.Config{}
	}
	proxyStr := ""
	if !helps.ShouldIgnoreAuthFileProxyURL(cfg, auth) {
		proxyStr = strings.TrimSpace(auth.ProxyURL)
	}
	if proxyStr == "" {
		proxyStr = strings.TrimSpace(cfg.ProxyURL)
	}
	if proxyStr == "" {
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
