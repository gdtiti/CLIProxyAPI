package management

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/authfs"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

func (h *Handler) persistAuthFilesToStore(ctx context.Context, action string, paths ...string) error {
	store := h.tokenStoreWithBaseDir()
	if store == nil {
		return fmt.Errorf("token store unavailable")
	}
	persister, ok := store.(authFilesStorePersister)
	if !ok {
		return nil
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "Update auth"
	}
	filtered := make([]string, 0, len(paths))
	displayName := ""
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		filtered = append(filtered, path)
		if displayName == "" {
			displayName = filepath.Base(path)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if displayName == "" {
		displayName = "auth"
	}
	if err := persister.PersistAuthFiles(ctx, fmt.Sprintf("%s %s", action, displayName), filtered...); err != nil {
		return fmt.Errorf("failed to persist auth file to store: %w", err)
	}
	return nil
}

func (h *Handler) authLogicalRelativePath(auth *coreauth.Auth, fallback string) (string, bool) {
	if auth != nil {
		if id := strings.TrimSpace(auth.ID); id != "" && !filepath.IsAbs(id) {
			if rel, err := authfs.NormalizeRelative(id); err == nil && !authfs.IsTrashRelative(rel) {
				return rel, true
			}
		}
		if h != nil && h.cfg != nil {
			if rel, err := authfs.RelativePath(h.cfg.AuthDir, authAttribute(auth, "path")); err == nil && !authfs.IsTrashRelative(rel) {
				return rel, true
			}
		}
		if fileName := strings.TrimSpace(auth.FileName); fileName != "" && !filepath.IsAbs(fileName) {
			if rel, err := authfs.NormalizeRelative(fileName); err == nil && !authfs.IsTrashRelative(rel) {
				return rel, true
			}
		}
	}
	if rel, err := authfs.NormalizeRelative(fallback); err == nil && !authfs.IsTrashRelative(rel) {
		return rel, true
	}
	return "", false
}

func (h *Handler) moveAuthFileToTrash(ctx context.Context, sourcePath, logicalRel, action string) (string, error) {
	if h == nil || h.cfg == nil {
		return "", fmt.Errorf("auth recycle bin unavailable")
	}
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", errAuthFileNotFound
	}
	logicalRel, err := authfs.NormalizeRelative(logicalRel)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(sourcePath) {
		if abs, errAbs := filepath.Abs(sourcePath); errAbs == nil {
			sourcePath = abs
		}
	}
	if _, err := os.Stat(sourcePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errAuthFileNotFound
		}
		return "", fmt.Errorf("stat auth file: %w", err)
	}
	trashPath, _, err := authfs.TrashPathForRelative(h.cfg.AuthDir, logicalRel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(trashPath), 0o700); err != nil {
		return "", fmt.Errorf("prepare recycle bin: %w", err)
	}
	if err := os.Remove(trashPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("clear existing recycle bin entry: %w", err)
	}
	if err := os.Rename(sourcePath, trashPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errAuthFileNotFound
		}
		return "", fmt.Errorf("move auth file to recycle bin: %w", err)
	}
	if err := h.persistAuthFilesToStore(ctx, action, sourcePath, trashPath); err != nil {
		if rollbackErr := os.Rename(trashPath, sourcePath); rollbackErr != nil {
			log.WithError(rollbackErr).Warnf("failed to rollback auth recycle bin move for %s", logicalRel)
		}
		return "", err
	}
	authfs.PruneEmptyParentDirs(strings.TrimSpace(h.cfg.AuthDir), filepath.Dir(sourcePath))
	return trashPath, nil
}

func (h *Handler) listRecycleBinAuthPaths() ([]string, error) {
	if h == nil || h.cfg == nil || strings.TrimSpace(h.cfg.AuthDir) == "" {
		return nil, fmt.Errorf("auth directory is not configured")
	}
	trashRoot := authfs.TrashRoot(h.cfg.AuthDir)
	if strings.TrimSpace(trashRoot) == "" {
		return nil, fmt.Errorf("auth recycle bin is not configured")
	}
	if _, err := os.Stat(trashRoot); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	paths := make([]string, 0, 32)
	err := filepath.WalkDir(trashRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func (h *Handler) resolveTrashSelection(name string) (trashPath, targetPath, targetRel string, err error) {
	if h == nil || h.cfg == nil {
		return "", "", "", fmt.Errorf("auth recycle bin unavailable")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", "", fmt.Errorf("invalid name")
	}
	if !authfs.IsTrashRelative(name) {
		normalized, errNormalize := authfs.NormalizeRelative(name)
		if errNormalize != nil {
			return "", "", "", fmt.Errorf("invalid name")
		}
		name = filepath.ToSlash(filepath.Join(authfs.TrashDirName, filepath.FromSlash(normalized)))
	}
	return authfs.RestorePathFor(h.cfg.AuthDir, name)
}

func (h *Handler) restoreAuthFileFromTrash(ctx context.Context, name string) (string, int, error) {
	trashPath, targetPath, targetRel, err := h.resolveTrashSelection(name)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	data, err := os.ReadFile(trashPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return targetRel, http.StatusNotFound, errAuthFileNotFound
		}
		return targetRel, http.StatusInternalServerError, fmt.Errorf("read recycle bin auth file: %w", err)
	}
	if _, err := os.Stat(targetPath); err == nil {
		return targetRel, http.StatusConflict, fmt.Errorf("active auth already exists")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return targetRel, http.StatusInternalServerError, fmt.Errorf("inspect active auth file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return targetRel, http.StatusInternalServerError, fmt.Errorf("prepare auth directory: %w", err)
	}
	if err := os.Rename(trashPath, targetPath); err != nil {
		return targetRel, http.StatusInternalServerError, fmt.Errorf("restore auth file: %w", err)
	}
	if err := h.persistAuthFilesToStore(ctx, "Restore auth", trashPath, targetPath); err != nil {
		if rollbackErr := os.Rename(targetPath, trashPath); rollbackErr != nil {
			log.WithError(rollbackErr).Warnf("failed to rollback auth restore for %s", targetRel)
		}
		return targetRel, http.StatusInternalServerError, err
	}
	if err := h.reloadAuthFile(ctx, targetPath, data); err != nil {
		_ = os.Rename(targetPath, trashPath)
		_ = h.persistAuthFilesToStore(ctx, "Restore auth rollback", targetPath, trashPath)
		return targetRel, http.StatusInternalServerError, err
	}
	authfs.PruneEmptyParentDirs(authfs.TrashRoot(h.cfg.AuthDir), filepath.Dir(trashPath))
	return targetRel, http.StatusOK, nil
}

func (h *Handler) purgeAuthFileFromTrash(ctx context.Context, name string) (string, int, error) {
	trashPath, _, targetRel, err := h.resolveTrashSelection(name)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	if err := os.Remove(trashPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return targetRel, http.StatusNotFound, errAuthFileNotFound
		}
		return targetRel, http.StatusInternalServerError, fmt.Errorf("purge recycle bin auth file: %w", err)
	}
	if err := h.persistAuthFilesToStore(ctx, "Purge auth", trashPath); err != nil {
		return targetRel, http.StatusInternalServerError, err
	}
	authfs.PruneEmptyParentDirs(authfs.TrashRoot(h.cfg.AuthDir), filepath.Dir(trashPath))
	return targetRel, http.StatusOK, nil
}

func (h *Handler) ListAuthRecycleBin(c *gin.Context) {
	paths, err := h.listRecycleBinAuthPaths()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read recycle bin: %v", err)})
		return
	}
	files := make([]gin.H, 0, len(paths))
	for _, path := range paths {
		info, errInfo := os.Stat(path)
		if errInfo != nil {
			continue
		}
		trashRel, errRel := authfs.RelativePath(h.cfg.AuthDir, path)
		if errRel != nil {
			continue
		}
		_, _, restoreRel, errRestore := authfs.RestorePathFor(h.cfg.AuthDir, trashRel)
		if errRestore != nil {
			continue
		}
		entry := gin.H{
			"name":       restoreRel,
			"trash_name": trashRel,
			"size":       info.Size(),
			"modtime":    info.ModTime(),
			"status":     "trashed",
			"disabled":   true,
			"source":     "trash",
		}
		if data, errRead := os.ReadFile(path); errRead == nil {
			entry["type"] = gjson.GetBytes(data, "type").String()
			entry["email"] = gjson.GetBytes(data, "email").String()
			if prefixValue := normalizeAuthPrefixMetadata(gjson.GetBytes(data, "prefix").String()); prefixValue != "" {
				entry["prefix"] = prefixValue
			}
			if proxyURLValue := strings.TrimSpace(gjson.GetBytes(data, "proxy_url").String()); proxyURLValue != "" {
				entry["proxy_url"] = proxyURLValue
			}
			if priorityValue, ok := parsePriorityValue(gjson.GetBytes(data, "priority").Value()); ok {
				entry["priority"] = priorityValue
			}
			if nv := gjson.GetBytes(data, "note"); nv.Exists() && nv.Type == gjson.String {
				if trimmed := strings.TrimSpace(nv.String()); trimmed != "" {
					entry["note"] = trimmed
				}
			}
		}
		files = append(files, entry)
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (h *Handler) RestoreAuthFile(c *gin.Context) {
	ctx := c.Request.Context()
	if all := c.Query("all"); all == "true" || all == "1" || all == "*" {
		paths, err := h.listRecycleBinAuthPaths()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read recycle bin: %v", err)})
			return
		}
		restored := make([]string, 0, len(paths))
		for _, path := range paths {
			name, errRel := authfs.RelativePath(h.cfg.AuthDir, path)
			if errRel != nil {
				continue
			}
			restoredName, status, errRestore := h.restoreAuthFileFromTrash(ctx, name)
			if errRestore != nil {
				c.JSON(status, gin.H{"error": errRestore.Error()})
				return
			}
			restored = append(restored, restoredName)
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "restored": len(restored), "files": restored})
		return
	}

	names, errNames := requestedAuthFileNamesForDelete(c)
	if errNames != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errNames.Error()})
		return
	}
	if len(names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	if len(names) == 1 {
		restoredName, status, errRestore := h.restoreAuthFileFromTrash(ctx, names[0])
		if errRestore != nil {
			c.JSON(status, gin.H{"error": errRestore.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "name": restoredName})
		return
	}

	restoredFiles := make([]string, 0, len(names))
	failed := make([]gin.H, 0)
	for _, name := range names {
		restoredName, _, errRestore := h.restoreAuthFileFromTrash(ctx, name)
		if errRestore != nil {
			failed = append(failed, gin.H{"name": name, "error": errRestore.Error()})
			continue
		}
		restoredFiles = append(restoredFiles, restoredName)
	}
	if len(failed) > 0 {
		c.JSON(http.StatusMultiStatus, gin.H{
			"status":   "partial",
			"restored": len(restoredFiles),
			"files":    restoredFiles,
			"failed":   failed,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "restored": len(restoredFiles), "files": restoredFiles})
}

func (h *Handler) PurgeAuthRecycleBin(c *gin.Context) {
	ctx := c.Request.Context()
	if all := c.Query("all"); all == "true" || all == "1" || all == "*" {
		paths, err := h.listRecycleBinAuthPaths()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read recycle bin: %v", err)})
			return
		}
		purged := make([]string, 0, len(paths))
		for _, path := range paths {
			name, errRel := authfs.RelativePath(h.cfg.AuthDir, path)
			if errRel != nil {
				continue
			}
			purgedName, status, errPurge := h.purgeAuthFileFromTrash(ctx, name)
			if errPurge != nil {
				c.JSON(status, gin.H{"error": errPurge.Error()})
				return
			}
			purged = append(purged, purgedName)
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "purged": len(purged), "files": purged})
		return
	}

	names, errNames := requestedAuthFileNamesForDelete(c)
	if errNames != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errNames.Error()})
		return
	}
	if len(names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	if len(names) == 1 {
		purgedName, status, errPurge := h.purgeAuthFileFromTrash(ctx, names[0])
		if errPurge != nil {
			c.JSON(status, gin.H{"error": errPurge.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "name": purgedName})
		return
	}

	purgedFiles := make([]string, 0, len(names))
	failed := make([]gin.H, 0)
	for _, name := range names {
		purgedName, _, errPurge := h.purgeAuthFileFromTrash(ctx, name)
		if errPurge != nil {
			failed = append(failed, gin.H{"name": name, "error": errPurge.Error()})
			continue
		}
		purgedFiles = append(purgedFiles, purgedName)
	}
	if len(failed) > 0 {
		c.JSON(http.StatusMultiStatus, gin.H{
			"status": "partial",
			"purged": len(purgedFiles),
			"files":  purgedFiles,
			"failed": failed,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "purged": len(purgedFiles), "files": purgedFiles})
}
