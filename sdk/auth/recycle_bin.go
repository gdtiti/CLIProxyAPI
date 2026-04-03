package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const TrashDirName = ".trash"

func NormalizeRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("auth recycle bin: relative path is empty")
	}
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("auth recycle bin: absolute path is not allowed")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	switch clean {
	case "", ".":
		return "", fmt.Errorf("auth recycle bin: relative path is empty")
	case "..":
		return "", fmt.Errorf("auth recycle bin: relative path escapes auth dir")
	}
	if strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("auth recycle bin: relative path escapes auth dir")
	}
	return clean, nil
}

func RelativePath(baseDir, path string) (string, error) {
	root, err := normalizeBaseDir(baseDir)
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("auth recycle bin: path is empty")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	clean := filepath.Clean(path)
	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return "", fmt.Errorf("auth recycle bin: compute relative path: %w", err)
	}
	return NormalizeRelativePath(rel)
}

func IsTrashRelative(rel string) bool {
	clean, err := NormalizeRelativePath(rel)
	if err != nil {
		return false
	}
	return clean == TrashDirName || strings.HasPrefix(clean, TrashDirName+"/")
}

func IsTrashPath(baseDir, path string) bool {
	rel, err := RelativePath(baseDir, path)
	if err != nil {
		return false
	}
	return IsTrashRelative(rel)
}

func TrashRoot(baseDir string) string {
	root, err := normalizeBaseDir(baseDir)
	if err != nil {
		return ""
	}
	return filepath.Join(root, TrashDirName)
}

func TrashPathForRelative(baseDir, activeRel string) (trashPath, trashRel string, err error) {
	root, err := normalizeBaseDir(baseDir)
	if err != nil {
		return "", "", err
	}
	activeRel, err = NormalizeRelativePath(activeRel)
	if err != nil {
		return "", "", err
	}
	if IsTrashRelative(activeRel) {
		return "", "", fmt.Errorf("auth recycle bin: active path already inside trash")
	}
	trashRel = filepath.ToSlash(filepath.Join(TrashDirName, filepath.FromSlash(activeRel)))
	trashPath = filepath.Join(root, filepath.FromSlash(trashRel))
	return trashPath, trashRel, nil
}

func RestorePathFor(baseDir, trashPathOrRel string) (trashPath, targetPath, targetRel string, err error) {
	root, err := normalizeBaseDir(baseDir)
	if err != nil {
		return "", "", "", err
	}
	rel, err := RelativePath(root, trashPathOrRel)
	if err != nil {
		return "", "", "", err
	}
	if !IsTrashRelative(rel) {
		return "", "", "", fmt.Errorf("auth recycle bin: path is not inside trash")
	}
	targetRel = strings.TrimPrefix(rel, TrashDirName+"/")
	targetRel, err = NormalizeRelativePath(targetRel)
	if err != nil {
		return "", "", "", err
	}
	trashPath = filepath.Join(root, filepath.FromSlash(rel))
	targetPath = filepath.Join(root, filepath.FromSlash(targetRel))
	return trashPath, targetPath, targetRel, nil
}

func PruneEmptyParentDirs(root, dir string) {
	root = filepath.Clean(strings.TrimSpace(root))
	dir = filepath.Clean(strings.TrimSpace(dir))
	if root == "" || dir == "" {
		return
	}
	for dir != root && strings.HasPrefix(dir, root) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func normalizeBaseDir(baseDir string) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return "", fmt.Errorf("auth recycle bin: auth dir is empty")
	}
	if !filepath.IsAbs(baseDir) {
		abs, err := filepath.Abs(baseDir)
		if err != nil {
			return "", fmt.Errorf("auth recycle bin: resolve auth dir: %w", err)
		}
		baseDir = abs
	}
	return filepath.Clean(baseDir), nil
}
