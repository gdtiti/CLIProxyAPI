package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrConfigRecordNotFound = errors.New("postgres store: config record not found")

type ConfigReloadResult struct {
	Store   string `json:"store"`
	Changed bool   `json:"changed"`
}

type AuthReloadResult struct {
	Store     string `json:"store"`
	Total     int    `json:"total"`
	Written   int    `json:"written"`
	Removed   int    `json:"removed"`
	Unchanged int    `json:"unchanged"`
}

type authStoreRecord struct {
	id      string
	content string
}

// ReloadConfigFromStore reloads the mirrored config file strictly from PostgreSQL.
// Unlike bootstrap, it does not seed from local files or templates when the record is absent.
func (s *PostgresStore) ReloadConfigFromStore(ctx context.Context) (*ConfigReloadResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres store: not initialized")
	}

	query := fmt.Sprintf("SELECT content FROM %s WHERE id = $1", s.fullTableName(s.cfg.ConfigTable))
	var content string
	if err := s.db.QueryRowContext(ctx, query, defaultConfigKey).Scan(&content); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConfigRecordNotFound
		}
		return nil, fmt.Errorf("postgres store: load config from database: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o700); err != nil {
		return nil, fmt.Errorf("postgres store: prepare config directory: %w", err)
	}

	changed, err := writeFileIfChanged(s.configPath, []byte(normalizeLineEndings(content)), 0o600)
	if err != nil {
		return nil, fmt.Errorf("postgres store: write config to spool: %w", err)
	}

	return &ConfigReloadResult{
		Store:   "postgres",
		Changed: changed,
	}, nil
}

// ReloadAuthFilesFromStore incrementally syncs the mirrored auth directory from PostgreSQL.
// It keeps the auth directory itself intact so the running watcher stays attached.
func (s *PostgresStore) ReloadAuthFilesFromStore(ctx context.Context) (*AuthReloadResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres store: not initialized")
	}

	records, err := s.loadAuthStoreRecords(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.authDir, 0o700); err != nil {
		return nil, fmt.Errorf("postgres store: prepare auth directory: %w", err)
	}

	existing, err := s.listLocalAuthFiles()
	if err != nil {
		return nil, err
	}

	result := &AuthReloadResult{
		Store: "postgres",
		Total: len(records),
	}
	seen := make(map[string]struct{}, len(records))

	for _, record := range records {
		path, errPath := s.absoluteAuthPath(record.id)
		if errPath != nil {
			return nil, errPath
		}
		if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("postgres store: create auth subdir: %w", err)
		}

		changed, errWrite := writeFileIfChanged(path, []byte(record.content), 0o600)
		if errWrite != nil {
			return nil, fmt.Errorf("postgres store: write auth file: %w", errWrite)
		}
		if changed {
			result.Written++
		} else {
			result.Unchanged++
		}
		seen[record.id] = struct{}{}
		delete(existing, record.id)
	}

	for relID, path := range existing {
		if _, ok := seen[relID]; ok {
			continue
		}
		if err = os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("postgres store: remove stale auth file: %w", err)
		}
		result.Removed++
		pruneEmptyParentDirs(s.authDir, filepath.Dir(path))
	}

	return result, nil
}

func (s *PostgresStore) loadAuthStoreRecords(ctx context.Context) ([]authStoreRecord, error) {
	query := fmt.Sprintf("SELECT id, content FROM %s ORDER BY id", s.fullTableName(s.cfg.AuthTable))
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres store: load auth from database: %w", err)
	}
	defer rows.Close()

	records := make([]authStoreRecord, 0, 32)
	for rows.Next() {
		var record authStoreRecord
		if err = rows.Scan(&record.id, &record.content); err != nil {
			return nil, fmt.Errorf("postgres store: scan auth row: %w", err)
		}
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres store: iterate auth rows: %w", err)
	}
	return records, nil
}

func (s *PostgresStore) listLocalAuthFiles() (map[string]string, error) {
	existing := make(map[string]string)
	err := filepath.Walk(s.authDir, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			return nil
		}
		relID, err := s.relativeAuthID(path)
		if err != nil {
			return err
		}
		existing[relID] = path
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("postgres store: list local auth files: %w", err)
	}
	return existing, nil
}

func writeFileIfChanged(path string, data []byte, perm fs.FileMode) (bool, error) {
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, data) {
			return false, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}

func pruneEmptyParentDirs(root, dir string) {
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
