package usage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DetailRow wraps a RequestDetail with API key and model info for PG insertion.
type DetailRow struct {
	APIKey string
	Model  string
	Detail RequestDetail
}

// AggregateRow represents a pre-aggregated hourly bucket for PG upsert.
type AggregateRow struct {
	InstanceID      string
	APIKey          string
	Model           string
	BucketHour      time.Time
	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
}

// PGStore provides PostgreSQL read/write operations for usage data.
type PGStore struct {
	db         *sql.DB
	instanceID string
}

// NewPGStore creates a new PGStore instance.
func NewPGStore(db *sql.DB, instanceID string) *PGStore {
	return &PGStore{db: db, instanceID: instanceID}
}

// BatchInsertDetails inserts multiple detail rows into usage_details.
func (s *PGStore) BatchInsertDetails(rows []DetailRow) error {
	if len(rows) == 0 {
		return nil
	}
	const cols = 13
	vStrs := make([]string, 0, len(rows))
	args := make([]interface{}, 0, len(rows)*cols)
	for i, r := range rows {
		b := i * cols
		vStrs = append(vStrs, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			b+1, b+2, b+3, b+4, b+5, b+6, b+7,
			b+8, b+9, b+10, b+11, b+12, b+13,
		))
		d := r.Detail
		args = append(args,
			s.instanceID, r.APIKey, r.Model,
			d.Timestamp, d.Source, d.AuthIndex, d.Failed,
			d.Tokens.InputTokens, d.Tokens.OutputTokens,
			d.Tokens.ReasoningTokens, d.Tokens.CachedTokens,
			d.Tokens.TotalTokens, time.Now(),
		)
	}
	q := `INSERT INTO usage_details
		(instance_id,api_key,model,timestamp,source,auth_index,
		 failed,input_tokens,output_tokens,reasoning_tokens,
		 cached_tokens,total_tokens,created_at) VALUES `
	q += strings.Join(vStrs, ",")
	if _, err := s.db.Exec(q, args...); err != nil {
		return fmt.Errorf("pg_store: batch insert: %w", err)
	}
	return nil
}

// UpsertAggregates upserts hourly aggregate rows into usage_aggregates.
func (s *PGStore) UpsertAggregates(aggs []AggregateRow) error {
	if len(aggs) == 0 {
		return nil
	}
	const upsertSQL = `INSERT INTO usage_aggregates
		(instance_id,api_key,model,bucket_hour,
		 total_requests,success_count,failure_count,
		 input_tokens,output_tokens,reasoning_tokens,
		 cached_tokens,total_tokens,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW())
		ON CONFLICT (instance_id,api_key,model,bucket_hour)
		DO UPDATE SET
		 total_requests   = usage_aggregates.total_requests   + EXCLUDED.total_requests,
		 success_count    = usage_aggregates.success_count    + EXCLUDED.success_count,
		 failure_count    = usage_aggregates.failure_count    + EXCLUDED.failure_count,
		 input_tokens     = usage_aggregates.input_tokens     + EXCLUDED.input_tokens,
		 output_tokens    = usage_aggregates.output_tokens    + EXCLUDED.output_tokens,
		 reasoning_tokens = usage_aggregates.reasoning_tokens + EXCLUDED.reasoning_tokens,
		 cached_tokens    = usage_aggregates.cached_tokens    + EXCLUDED.cached_tokens,
		 total_tokens     = usage_aggregates.total_tokens     + EXCLUDED.total_tokens,
		 updated_at       = NOW()`
	for _, a := range aggs {
		inst := a.InstanceID
		if inst == "" {
			inst = s.instanceID
		}
		_, err := s.db.Exec(upsertSQL,
			inst, a.APIKey, a.Model, a.BucketHour,
			a.TotalRequests, a.SuccessCount, a.FailureCount,
			a.InputTokens, a.OutputTokens, a.ReasoningTokens,
			a.CachedTokens, a.TotalTokens,
		)
		if err != nil {
			return fmt.Errorf("pg_store: upsert aggregate: %w", err)
		}
	}
	return nil
}

// DeleteBefore removes usage data older than the given time.
func (s *PGStore) DeleteBefore(before time.Time) error {
	if _, err := s.db.Exec(
		`DELETE FROM usage_details WHERE created_at < $1`, before,
	); err != nil {
		return fmt.Errorf("pg_store: delete details before: %w", err)
	}
	if _, err := s.db.Exec(
		`DELETE FROM usage_aggregates WHERE bucket_hour < $1`, before,
	); err != nil {
		return fmt.Errorf("pg_store: delete aggregates before: %w", err)
	}
	return nil
}

// DeleteAll removes all usage data from both tables.
func (s *PGStore) DeleteAll() error {
	if _, err := s.db.Exec(`DELETE FROM usage_details`); err != nil {
		return fmt.Errorf("pg_store: delete all details: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM usage_aggregates`); err != nil {
		return fmt.Errorf("pg_store: delete all aggregates: %w", err)
	}
	return nil
}

// end of pg_store.go

// getInstanceID 返回实例标识。
func (s *PGStore) getInstanceID() string {
	return s.instanceID
}
