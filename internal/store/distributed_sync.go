package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	defaultVersionTable = "sync_versions"
	defaultOutboxTable  = "sync_outbox"

	DistributedResourceConfig = "config"
	DistributedResourceAuth   = "auth"

	DistributedOperationDelete = "delete"
	DistributedOperationSync   = "sync"
	DistributedOperationUpsert = "upsert"
)

// DistributedChange describes a committed config/auth change that should be
// propagated to other nodes.
type DistributedChange struct {
	EventID   int64           `json:"event_id"`
	Resource  string          `json:"resource"`
	Version   int64           `json:"version"`
	Operation string          `json:"operation"`
	Message   string          `json:"message,omitempty"`
	NodeID    string          `json:"node_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// DistributedChangeNotifier publishes committed changes to an external bus.
type DistributedChangeNotifier interface {
	PublishDistributedChange(ctx context.Context, change DistributedChange) error
}

type authBatchPayload struct {
	Upserted []string `json:"upserted,omitempty"`
	Deleted  []string `json:"deleted,omitempty"`
}

func (s *PostgresStore) distributedVersionTable() string {
	return s.fullTableName(defaultVersionTable)
}

func (s *PostgresStore) distributedOutboxTable() string {
	return s.fullTableName(defaultOutboxTable)
}

// SetDistributedChangeNotifier configures the optional notifier used after
// successful commits. Publish failures never roll back committed data.
func (s *PostgresStore) SetDistributedChangeNotifier(notifier DistributedChangeNotifier) {
	if s == nil {
		return
	}
	s.notifierMu.Lock()
	s.changeNotifier = notifier
	s.notifierMu.Unlock()
}

func (s *PostgresStore) currentDistributedNotifier() DistributedChangeNotifier {
	if s == nil {
		return nil
	}
	s.notifierMu.RLock()
	defer s.notifierMu.RUnlock()
	return s.changeNotifier
}

// EnsureDistributedSchema creates the version and outbox tables required by
// distributed sync.
func (s *PostgresStore) EnsureDistributedSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres store: not initialized")
	}

	versionTable := s.distributedVersionTable()
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			resource TEXT PRIMARY KEY,
			version BIGINT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, versionTable)); err != nil {
		return fmt.Errorf("postgres store: create distributed version table: %w", err)
	}

	outboxTable := s.distributedOutboxTable()
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			resource TEXT NOT NULL,
			version BIGINT NOT NULL,
			operation TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, outboxTable)); err != nil {
		return fmt.Errorf("postgres store: create distributed outbox table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s
		ON %s (resource, version DESC)
	`, quoteIdentifier(defaultOutboxTable+"_resource_version_idx"), outboxTable)); err != nil {
		return fmt.Errorf("postgres store: create distributed outbox index: %w", err)
	}

	return nil
}

// GetDistributedVersions returns the current committed resource versions.
func (s *PostgresStore) GetDistributedVersions(ctx context.Context) (map[string]int64, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres store: not initialized")
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		"SELECT resource, version FROM %s",
		s.distributedVersionTable(),
	))
	if err != nil {
		return nil, fmt.Errorf("postgres store: query distributed versions: %w", err)
	}
	defer rows.Close()

	versions := map[string]int64{
		DistributedResourceConfig: 0,
		DistributedResourceAuth:   0,
	}
	for rows.Next() {
		var (
			resource string
			version  int64
		)
		if err = rows.Scan(&resource, &version); err != nil {
			return nil, fmt.Errorf("postgres store: scan distributed version: %w", err)
		}
		versions[strings.TrimSpace(resource)] = version
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres store: iterate distributed versions: %w", err)
	}

	return versions, nil
}

func (s *PostgresStore) beginTx(ctx context.Context) (*sql.Tx, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres store: not initialized")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("postgres store: begin transaction: %w", err)
	}
	return tx, nil
}

func commitTx(tx *sql.Tx) error {
	if tx == nil {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres store: commit transaction: %w", err)
	}
	return nil
}

func rollbackTx(tx *sql.Tx) {
	if tx == nil {
		return
	}
	_ = tx.Rollback()
}

func (s *PostgresStore) recordDistributedChangeTx(ctx context.Context, tx *sql.Tx, resource, operation, message string, payload json.RawMessage) (*DistributedChange, error) {
	if tx == nil {
		return nil, fmt.Errorf("postgres store: nil transaction")
	}
	if payload == nil {
		payload = json.RawMessage(`{}`)
	}
	change := &DistributedChange{
		Resource:  resource,
		Operation: operation,
		Message:   strings.TrimSpace(message),
		Payload:   payload,
	}

	versionTable := s.distributedVersionTable()
	outboxTable := s.distributedOutboxTable()
	query := fmt.Sprintf(`
		WITH next_version AS (
			INSERT INTO %s (resource, version, updated_at)
			VALUES ($1, 1, NOW())
			ON CONFLICT (resource)
			DO UPDATE SET version = %s.version + 1, updated_at = NOW()
			RETURNING version
		)
		INSERT INTO %s (resource, version, operation, message, payload, created_at)
		SELECT $1, version, $2, $3, $4, NOW()
		FROM next_version
		RETURNING id, version, created_at
	`, versionTable, versionTable, outboxTable)
	if err := tx.QueryRowContext(
		ctx,
		query,
		resource,
		operation,
		change.Message,
		payload,
	).Scan(&change.EventID, &change.Version, &change.CreatedAt); err != nil {
		return nil, fmt.Errorf("postgres store: record distributed change: %w", err)
	}

	return change, nil
}

func (s *PostgresStore) publishDistributedChange(change *DistributedChange) {
	if s == nil || change == nil {
		return
	}
	notifier := s.currentDistributedNotifier()
	if notifier == nil {
		return
	}

	copyChange := *change
	if copyChange.CreatedAt.IsZero() {
		copyChange.CreatedAt = time.Now().UTC()
	}

	go func(notifier DistributedChangeNotifier, change DistributedChange) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := notifier.PublishDistributedChange(ctx, change); err != nil {
			log.WithError(err).Warnf(
				"postgres store: failed to publish distributed change resource=%s version=%d",
				change.Resource,
				change.Version,
			)
		}
	}(notifier, copyChange)
}

func loadTextRecordTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (string, bool, error) {
	if tx == nil {
		return "", false, fmt.Errorf("postgres store: nil transaction")
	}

	var content string
	err := tx.QueryRowContext(ctx, query, args...).Scan(&content)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	default:
		return content, true, nil
	}
}

func marshalDistributedPayload(payload any) json.RawMessage {
	if payload == nil {
		return json.RawMessage(`{}`)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
