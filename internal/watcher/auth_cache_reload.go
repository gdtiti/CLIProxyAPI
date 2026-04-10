package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher/synthesizer"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

const authReloadBatchSize = 32

type authFileCacheEntry struct {
	normalizedPath string
	hash           string
	parsed         *coreauth.Auth
	generated      map[string]*coreauth.Auth
	readable       bool
}

func (w *Watcher) rebuildFileAuthCaches(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}

	authDir, err := util.ResolveAuthDir(cfg.AuthDir)
	if err != nil {
		log.Errorf("failed to resolve auth directory for hash cache: %v", err)
		return 0
	}
	if authDir == "" {
		w.clientsMutex.Lock()
		w.lastAuthHashes = make(map[string]string)
		w.lastAuthContents = make(map[string]*coreauth.Auth)
		w.fileAuthsByPath = make(map[string]map[string]*coreauth.Auth)
		w.fileAuthCacheReady = true
		w.clientsMutex.Unlock()
		return 0
	}

	paths := make([]string, 0, 64)
	_ = filepath.Walk(authDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	sort.Strings(paths)

	hashes := make(map[string]string, len(paths))
	contents := make(map[string]*coreauth.Auth, len(paths))
	authsByPath := make(map[string]map[string]*coreauth.Auth, len(paths))
	readableCount := 0

	for batchStart := 0; batchStart < len(paths); batchStart += authReloadBatchSize {
		batchEnd := batchStart + authReloadBatchSize
		if batchEnd > len(paths) {
			batchEnd = len(paths)
		}
		entries := w.buildAuthCacheBatch(cfg, authDir, paths[batchStart:batchEnd])
		for _, entry := range entries {
			if !entry.readable {
				continue
			}
			readableCount++
			hashes[entry.normalizedPath] = entry.hash
			if entry.parsed != nil {
				contents[entry.normalizedPath] = entry.parsed
			}
			if len(entry.generated) > 0 {
				authsByPath[entry.normalizedPath] = entry.generated
			}
		}
	}

	w.clientsMutex.Lock()
	w.lastAuthHashes = hashes
	w.lastAuthContents = contents
	w.fileAuthsByPath = authsByPath
	w.fileAuthCacheReady = true
	w.clientsMutex.Unlock()

	log.Debugf("auth directory scan complete - found %d .json files, %d readable", len(paths), readableCount)
	return len(paths)
}

func (w *Watcher) buildAuthCacheBatch(cfg *config.Config, authDir string, paths []string) []authFileCacheEntry {
	entries := make([]authFileCacheEntry, len(paths))
	var wg sync.WaitGroup

	for idx, path := range paths {
		wg.Add(1)
		go func(index int, filePath string) {
			defer wg.Done()
			entries[index] = w.buildAuthCacheEntry(cfg, authDir, filePath)
		}(idx, path)
	}

	wg.Wait()
	return entries
}

func (w *Watcher) buildAuthCacheEntry(cfg *config.Config, authDir, path string) authFileCacheEntry {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return authFileCacheEntry{}
	}

	sum := sha256.Sum256(data)
	entry := authFileCacheEntry{
		normalizedPath: w.normalizeAuthPath(path),
		hash:           hex.EncodeToString(sum[:]),
		readable:       true,
	}

	if parsed := parseCachedAuthFile(data); parsed != nil {
		entry.parsed = parsed
	}

	ctx := &synthesizer.SynthesisContext{
		Config:      cfg,
		AuthDir:     authDir,
		Now:         time.Now(),
		IDGenerator: synthesizer.NewStableIDGenerator(),
	}
	entry.generated = authSliceToMap(synthesizer.SynthesizeAuthFile(ctx, path, data))
	return entry
}

func parseCachedAuthFile(data []byte) *coreauth.Auth {
	var parsed coreauth.Auth
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	return &parsed
}
