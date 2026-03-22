package watcher

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

func (w *Watcher) autoReloadInterval() time.Duration {
	w.clientsMutex.RLock()
	defer w.clientsMutex.RUnlock()
	if w.config == nil || w.config.AutoReloadIntervalSeconds <= 0 {
		return 0
	}
	return time.Duration(w.config.AutoReloadIntervalSeconds) * time.Second
}

func (w *Watcher) startAutoReload(ctx context.Context) {
	interval := w.autoReloadInterval()
	if interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Infof("watcher auto reload enabled: every %s", interval)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.handleAutoReloadTick()
			}
		}
	}()
}

func (w *Watcher) handleAutoReloadTick() {
	newHash, err := w.readConfigHash()
	if err != nil {
		log.WithError(err).Warn("watcher auto reload: failed to read config hash")
		return
	}
	if newHash == "" {
		log.Debug("watcher auto reload: skipping empty config file")
		return
	}

	w.clientsMutex.RLock()
	currentHash := w.lastConfigHash
	w.clientsMutex.RUnlock()

	if currentHash == "" || currentHash != newHash {
		log.Debug("watcher auto reload: config hash changed, reloading config")
		w.reloadConfigIfChanged()
		return
	}

	log.Debug("watcher auto reload: config unchanged, rescanning auth files")
	w.reloadClients(true, nil, false)
}
