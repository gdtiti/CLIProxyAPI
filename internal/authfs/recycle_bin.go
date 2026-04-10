package authfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const TrashDirName = ".trash"

func TrashRoot(authDir string) string {
	root, err := normalizeAuthDir(authDir)
	if err != nil {
		return ""
	}
	return filepath.Join(root, TrashDirName)
}

func NormalizeRelative(rel string) (string, error) {
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

func RelativePath(authDir, path string) (string, error) {
	root, err := normalizeAuthDir(authDir)
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
	return NormalizeRelative(rel)
}

func IsTrashRelative(rel string) bool {
	clean, err := NormalizeRelative(rel)
	if err != nil {
		return false
	}
	return clean == TrashDirName || strings.HasPrefix(clean, TrashDirName+"/")
}

func IsTrashPath(authDir, path string) bool {
	rel, err := RelativePath(authDir, path)
	if err != nil {
		return false
	}
	return IsTrashRelative(rel)
}

func TrashPathFor(authDir, activePath string) (trashPath, trashRel, activeRel string, err error) {
	activeRel, err = RelativePath(authDir, activePath)
	if err != nil {
		return "", "", "", err
	}
	trashPath, trashRel, err = TrashPathForRelative(authDir, activeRel)
	if err != nil {
		return "", "", "", err
	}
	return trashPath, trashRel, activeRel, nil
}

func TrashPathForRelative(authDir, activeRel string) (trashPath, trashRel string, err error) {
	root, err := normalizeAuthDir(authDir)
	if err != nil {
		return "", "", err
	}
	activeRel, err = NormalizeRelative(activeRel)
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

func RestorePathFor(authDir, trashPathOrRel string) (trashPath, targetPath, targetRel string, err error) {
	root, err := normalizeAuthDir(authDir)
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
	targetRel, err = NormalizeRelative(targetRel)
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

func normalizeAuthDir(authDir string) (string, error) {
	authDir = strings.TrimSpace(authDir)
	if authDir == "" {
		return "", fmt.Errorf("auth recycle bin: auth dir is empty")
	}
	if !filepath.IsAbs(authDir) {
		abs, err := filepath.Abs(authDir)
		if err != nil {
			return "", fmt.Errorf("auth recycle bin: resolve auth dir: %w", err)
		}
		authDir = abs
	}
	return filepath.Clean(authDir), nil
}
