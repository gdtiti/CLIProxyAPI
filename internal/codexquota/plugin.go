package codexquota

import (
	"context"
	"sync"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

var (
	defaultServiceMu sync.RWMutex
	defaultService   *Service
	registerOnce     sync.Once
)

// SetDefaultService updates the process-wide Codex quota service.
func SetDefaultService(service *Service) {
	defaultServiceMu.Lock()
	defaultService = service
	defaultServiceMu.Unlock()
}

// DefaultService returns the process-wide Codex quota service.
func DefaultService() *Service {
	defaultServiceMu.RLock()
	defer defaultServiceMu.RUnlock()
	return defaultService
}

// RegisterDefaultUsagePlugin installs the global usage plugin exactly once.
func RegisterDefaultUsagePlugin() {
	registerOnce.Do(func() {
		coreusage.RegisterPlugin(defaultUsagePlugin{})
	})
}

type defaultUsagePlugin struct{}

func (defaultUsagePlugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if service := DefaultService(); service != nil {
		service.ApplyUsage(record)
	}
}
