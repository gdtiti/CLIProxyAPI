package codexquota

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
)

const (
	dataDirName    = ".codex-quota"
	snapshotsFile  = "snapshots.json"
	rollupsFile    = "rollups.json"
	eventsFile     = "events.json"
	filePermission = 0o644
)

type store struct {
	dir string
}

func newStore(authDir string) (*store, error) {
	resolved, err := util.ResolveAuthDir(authDir)
	if err != nil {
		return nil, err
	}
	if resolved == "" {
		return nil, fmt.Errorf("codex quota store: auth directory is empty")
	}
	dir := filepath.Join(resolved, dataDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("codex quota store: create %s: %w", dir, err)
	}
	return &store{dir: dir}, nil
}

func (s *store) loadState() (map[string]Snapshot, map[string]UsageRollup, []Event, error) {
	snapshots := make([]Snapshot, 0)
	rollups := make([]UsageRollup, 0)
	events := make([]Event, 0)
	if err := s.loadJSON(snapshotsFile, &snapshots); err != nil {
		return nil, nil, nil, err
	}
	if err := s.loadJSON(rollupsFile, &rollups); err != nil {
		return nil, nil, nil, err
	}
	if err := s.loadJSON(eventsFile, &events); err != nil {
		return nil, nil, nil, err
	}
	snapshotMap := make(map[string]Snapshot, len(snapshots))
	for _, item := range snapshots {
		snapshotMap[stateKey(item.AuthID, item.AuthIndex)] = item
	}
	rollupMap := make(map[string]UsageRollup, len(rollups))
	for _, item := range rollups {
		rollupMap[stateKey(item.AuthID, item.AuthIndex)] = item
	}
	return snapshotMap, rollupMap, events, nil
}

func (s *store) saveState(snapshots map[string]Snapshot, rollups map[string]UsageRollup, events []Event) error {
	snapshotItems := make([]Snapshot, 0, len(snapshots))
	for _, item := range snapshots {
		snapshotItems = append(snapshotItems, item)
	}
	sort.Slice(snapshotItems, func(i, j int) bool {
		if snapshotItems[i].AuthIndex == snapshotItems[j].AuthIndex {
			return snapshotItems[i].AuthID < snapshotItems[j].AuthID
		}
		return snapshotItems[i].AuthIndex < snapshotItems[j].AuthIndex
	})

	rollupItems := make([]UsageRollup, 0, len(rollups))
	for _, item := range rollups {
		rollupItems = append(rollupItems, item)
	}
	sort.Slice(rollupItems, func(i, j int) bool {
		if rollupItems[i].AuthIndex == rollupItems[j].AuthIndex {
			return rollupItems[i].AuthID < rollupItems[j].AuthID
		}
		return rollupItems[i].AuthIndex < rollupItems[j].AuthIndex
	})

	eventItems := append([]Event(nil), events...)
	sort.Slice(eventItems, func(i, j int) bool {
		return eventItems[i].CreatedAt.Before(eventItems[j].CreatedAt)
	})

	if err := s.saveJSON(snapshotsFile, snapshotItems); err != nil {
		return err
	}
	if err := s.saveJSON(rollupsFile, rollupItems); err != nil {
		return err
	}
	if err := s.saveJSON(eventsFile, eventItems); err != nil {
		return err
	}
	return nil
}

func (s *store) loadJSON(name string, dst any) error {
	path := filepath.Join(s.dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("codex quota store: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("codex quota store: decode %s: %w", path, err)
	}
	return nil
}

func (s *store) saveJSON(name string, value any) error {
	path := filepath.Join(s.dir, name)
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("codex quota store: encode %s: %w", path, err)
	}
	tmp := fmt.Sprintf("%s.%d.tmp", path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, filePermission); err != nil {
		return fmt.Errorf("codex quota store: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("codex quota store: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
