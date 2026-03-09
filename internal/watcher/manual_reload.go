package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// ReloadConfigNow forces the watcher to reload config from disk and refresh runtime state.
func (w *Watcher) ReloadConfigNow() error {
	if w == nil {
		return fmt.Errorf("watcher: not initialized")
	}
	if _, err := os.ReadFile(w.configPath); err != nil {
		return err
	}
	if _, err := config.LoadConfig(w.configPath); err != nil {
		return err
	}
	if ok := w.reloadConfig(); !ok {
		return fmt.Errorf("watcher: config reload failed")
	}
	if data, err := os.ReadFile(w.configPath); err == nil && len(data) > 0 {
		sum := sha256.Sum256(data)
		w.clientsMutex.Lock()
		w.lastConfigHash = hex.EncodeToString(sum[:])
		w.clientsMutex.Unlock()
	}
	return nil
}

// ReloadAuthFilesNow forces a full auth directory rescan and runtime refresh.
func (w *Watcher) ReloadAuthFilesNow(forceAuthRefresh bool) error {
	if w == nil {
		return fmt.Errorf("watcher: not initialized")
	}
	w.clientsMutex.RLock()
	cfg := w.config
	w.clientsMutex.RUnlock()
	if cfg == nil {
		return fmt.Errorf("watcher: config is nil")
	}
	w.reloadClients(true, nil, forceAuthRefresh)
	return nil
}
