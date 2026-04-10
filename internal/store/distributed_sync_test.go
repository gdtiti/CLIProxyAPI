package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type captureNotifier struct {
	ch  chan DistributedChange
	err error
}

func (n *captureNotifier) PublishDistributedChange(_ context.Context, change DistributedChange) error {
	if n.ch != nil {
		n.ch <- change
	}
	return n.err
}

func TestPersistConfigSkipsDistributedChangeWhenContentUnchanged(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := "port: 8317\n"
	if err = writeTestFile(configPath, []byte(content)); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := &PostgresStore{
		db:         db,
		cfg:        PostgresStoreConfig{ConfigTable: defaultConfigTable, AuthTable: defaultAuthTable},
		configPath: configPath,
		authDir:    filepath.Join(dir, "auths"),
	}
	notifier := &captureNotifier{ch: make(chan DistributedChange, 1)}
	store.SetDistributedChangeNotifier(notifier)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT content FROM "config_store" WHERE id = \$1`).
		WithArgs(defaultConfigKey).
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow(content))
	mock.ExpectCommit()

	if err = store.PersistConfig(context.Background()); err != nil {
		t.Fatalf("PersistConfig: %v", err)
	}

	select {
	case change := <-notifier.ch:
		t.Fatalf("unexpected distributed change: %+v", change)
	case <-time.After(50 * time.Millisecond):
	}

	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPersistConfigPublishesDistributedChangeOnUpdate(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := "port: 8317\n"
	if err = writeTestFile(configPath, []byte(content)); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := &PostgresStore{
		db:         db,
		cfg:        PostgresStoreConfig{ConfigTable: defaultConfigTable, AuthTable: defaultAuthTable},
		configPath: configPath,
		authDir:    filepath.Join(dir, "auths"),
	}
	notifier := &captureNotifier{ch: make(chan DistributedChange, 1)}
	store.SetDistributedChangeNotifier(notifier)

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT content FROM "config_store" WHERE id = \$1`).
		WithArgs(defaultConfigKey).
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow("port: 8383\n"))
	mock.ExpectExec(`(?s)INSERT INTO "config_store".*DO UPDATE SET content = EXCLUDED\.content, updated_at = NOW\(\)`).
		WithArgs(defaultConfigKey, content).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`(?s)INSERT INTO "sync_outbox".*RETURNING id, version, created_at`).
		WithArgs(DistributedResourceConfig, DistributedOperationUpsert, "persist config", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version", "created_at"}).AddRow(int64(11), int64(3), now))
	mock.ExpectCommit()

	if err = store.PersistConfig(context.Background()); err != nil {
		t.Fatalf("PersistConfig: %v", err)
	}

	select {
	case change := <-notifier.ch:
		if change.Resource != DistributedResourceConfig {
			t.Fatalf("unexpected resource: %s", change.Resource)
		}
		if change.Version != 3 {
			t.Fatalf("unexpected version: %d", change.Version)
		}
		if change.Operation != DistributedOperationUpsert {
			t.Fatalf("unexpected operation: %s", change.Operation)
		}
		var payload map[string]string
		if err = json.Unmarshal(change.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload["id"] != defaultConfigKey {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for distributed change")
	}

	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPersistConfigIgnoresNotifierFailure(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := "port: 8317\n"
	if err = writeTestFile(configPath, []byte(content)); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := &PostgresStore{
		db:         db,
		cfg:        PostgresStoreConfig{ConfigTable: defaultConfigTable, AuthTable: defaultAuthTable},
		configPath: configPath,
		authDir:    filepath.Join(dir, "auths"),
	}
	store.SetDistributedChangeNotifier(&captureNotifier{err: errors.New("redis unavailable")})

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT content FROM "config_store" WHERE id = \$1`).
		WithArgs(defaultConfigKey).
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow("port: 8383\n"))
	mock.ExpectExec(`(?s)INSERT INTO "config_store".*DO UPDATE SET content = EXCLUDED\.content, updated_at = NOW\(\)`).
		WithArgs(defaultConfigKey, content).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`(?s)INSERT INTO "sync_outbox".*RETURNING id, version, created_at`).
		WithArgs(DistributedResourceConfig, DistributedOperationUpsert, "persist config", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version", "created_at"}).AddRow(int64(12), int64(4), now))
	mock.ExpectCommit()

	if err = store.PersistConfig(context.Background()); err != nil {
		t.Fatalf("PersistConfig: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestPersistAuthFilesBatchesChangedPathsIntoSingleDistributedEvent(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	authDir := filepath.Join(dir, "auths")
	fileA := filepath.Join(authDir, "a.json")
	if err = writeTestFile(fileA, []byte(`{"type":"gemini","email":"a@example.com"}`)); err != nil {
		t.Fatalf("write auth file: %v", err)
	}
	fileB := filepath.Join(authDir, "b.json")

	store := &PostgresStore{
		db:      db,
		cfg:     PostgresStoreConfig{ConfigTable: defaultConfigTable, AuthTable: defaultAuthTable},
		authDir: authDir,
	}
	notifier := &captureNotifier{ch: make(chan DistributedChange, 1)}
	store.SetDistributedChangeNotifier(notifier)

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT content::text FROM "auth_store" WHERE id = \$1`).
		WithArgs("a.json").
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow(`{"type":"old"}`))
	mock.ExpectExec(`(?s)INSERT INTO "auth_store".*DO UPDATE SET content = EXCLUDED\.content, updated_at = NOW\(\)`).
		WithArgs("a.json", json.RawMessage(`{"type":"gemini","email":"a@example.com"}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT content::text FROM "auth_store" WHERE id = \$1`).
		WithArgs("b.json").
		WillReturnRows(sqlmock.NewRows([]string{"content"}).AddRow(`{"type":"delete"}`))
	mock.ExpectExec(`DELETE FROM "auth_store" WHERE id = \$1`).
		WithArgs("b.json").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`(?s)INSERT INTO "sync_outbox".*RETURNING id, version, created_at`).
		WithArgs(DistributedResourceAuth, DistributedOperationSync, "bulk sync", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version", "created_at"}).AddRow(int64(21), int64(7), now))
	mock.ExpectCommit()

	if err = store.PersistAuthFiles(context.Background(), "bulk sync", fileA, fileB); err != nil {
		t.Fatalf("PersistAuthFiles: %v", err)
	}

	select {
	case change := <-notifier.ch:
		if change.Resource != DistributedResourceAuth {
			t.Fatalf("unexpected resource: %s", change.Resource)
		}
		if change.Version != 7 {
			t.Fatalf("unexpected version: %d", change.Version)
		}
		var payload authBatchPayload
		if err = json.Unmarshal(change.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if !reflect.DeepEqual(payload.Upserted, []string{"a.json"}) {
			t.Fatalf("unexpected upserted payload: %#v", payload.Upserted)
		}
		if !reflect.DeepEqual(payload.Deleted, []string{"b.json"}) {
			t.Fatalf("unexpected deleted payload: %#v", payload.Deleted)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for distributed change")
	}

	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func writeTestFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
