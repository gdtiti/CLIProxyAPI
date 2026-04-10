package usage

import (
	"database/sql"
	"fmt"
	"strings"
)

// EnsureUsageSchema creates the usage_details and usage_aggregates tables
// along with all required indexes. If schema is non-empty, it creates the
// schema first and sets the search_path accordingly.
func EnsureUsageSchema(db *sql.DB, schema string) error {
	if db == nil {
		return fmt.Errorf("usage schema: db connection is nil")
	}

	// Step 1: Create schema and set search_path if needed
	if s := strings.TrimSpace(schema); s != "" {
		if _, err := db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", quoteIdent(s))); err != nil {
			return fmt.Errorf("usage schema: create schema: %w", err)
		}
		if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s, public", quoteIdent(s))); err != nil {
			return fmt.Errorf("usage schema: set search_path: %w", err)
		}
	}

	// Step 2: Create tables
	if err := createUsageDetailsTable(db); err != nil {
		return err
	}
	if err := createUsageAggregatesTable(db); err != nil {
		return err
	}

	// Step 3: Create indexes
	if err := createUsageIndexes(db); err != nil {
		return err
	}

	return nil
}

// quoteIdent quotes a SQL identifier to prevent injection.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// createUsageDetailsTable creates the usage_details table.
func createUsageDetailsTable(db *sql.DB) error {
	ddl := `CREATE TABLE IF NOT EXISTS usage_details (
		id               BIGSERIAL PRIMARY KEY,
		instance_id      VARCHAR(64)  NOT NULL,
		api_key          VARCHAR(512) NOT NULL,
		model            VARCHAR(256) NOT NULL,
		timestamp        TIMESTAMPTZ  NOT NULL,
		source           VARCHAR(512) DEFAULT '',
		auth_index       VARCHAR(128) DEFAULT '',
		failed           BOOLEAN      DEFAULT FALSE,
		input_tokens     BIGINT       DEFAULT 0,
		output_tokens    BIGINT       DEFAULT 0,
		reasoning_tokens BIGINT       DEFAULT 0,
		cached_tokens    BIGINT       DEFAULT 0,
		total_tokens     BIGINT       DEFAULT 0,
		created_at       TIMESTAMPTZ  DEFAULT NOW()
	)`
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("usage schema: create usage_details: %w", err)
	}
	return nil
}


// createUsageAggregatesTable creates the usage_aggregates table.
func createUsageAggregatesTable(db *sql.DB) error {
	ddl := `CREATE TABLE IF NOT EXISTS usage_aggregates (
		id               BIGSERIAL PRIMARY KEY,
		instance_id      VARCHAR(64)  NOT NULL,
		api_key          VARCHAR(512) NOT NULL,
		model            VARCHAR(256) NOT NULL,
		bucket_hour      TIMESTAMPTZ  NOT NULL,
		total_requests   BIGINT       DEFAULT 0,
		success_count    BIGINT       DEFAULT 0,
		failure_count    BIGINT       DEFAULT 0,
		input_tokens     BIGINT       DEFAULT 0,
		output_tokens    BIGINT       DEFAULT 0,
		reasoning_tokens BIGINT       DEFAULT 0,
		cached_tokens    BIGINT       DEFAULT 0,
		total_tokens     BIGINT       DEFAULT 0,
		updated_at       TIMESTAMPTZ  DEFAULT NOW(),
		UNIQUE (instance_id, api_key, model, bucket_hour)
	)`
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("usage schema: create usage_aggregates: %w", err)
	}
	return nil
}

// createUsageIndexes creates all required indexes for usage tables.
func createUsageIndexes(db *sql.DB) error {
	indexes := []struct {
		name string
		ddl  string
	}{
		{
			"idx_usage_details_ts",
			"CREATE INDEX IF NOT EXISTS idx_usage_details_ts ON usage_details (timestamp DESC)",
		},
		{
			"idx_usage_details_instance_ts",
			"CREATE INDEX IF NOT EXISTS idx_usage_details_instance_ts ON usage_details (instance_id, timestamp DESC)",
		},
		{
			"idx_usage_details_api_model",
			"CREATE INDEX IF NOT EXISTS idx_usage_details_api_model ON usage_details (api_key, model, timestamp DESC)",
		},
		{
			"idx_usage_details_created",
			"CREATE INDEX IF NOT EXISTS idx_usage_details_created ON usage_details (created_at)",
		},
		{
			"idx_usage_agg_bucket",
			"CREATE INDEX IF NOT EXISTS idx_usage_agg_bucket ON usage_aggregates (bucket_hour DESC)",
		},
		{
			"idx_usage_agg_instance",
			"CREATE INDEX IF NOT EXISTS idx_usage_agg_instance ON usage_aggregates (instance_id, bucket_hour DESC)",
		},
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx.ddl); err != nil {
			return fmt.Errorf("usage schema: create index %s: %w", idx.name, err)
		}
	}
	return nil
}
