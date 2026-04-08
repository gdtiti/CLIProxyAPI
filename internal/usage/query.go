package usage

import (
	"fmt"
	"time"
)

const defaultSnapshotDetailsLimit = 50000

// QueryAggregates returns aggregate rows within the given time range.
// If instanceID is empty, all instances are included.
func (s *PGStore) QueryAggregates(from, to time.Time, instanceID string) ([]AggregateRow, error) {
	q := `SELECT instance_id, api_key, model, bucket_hour,
		total_requests, success_count, failure_count,
		input_tokens, output_tokens, reasoning_tokens,
		cached_tokens, total_tokens
		FROM usage_aggregates WHERE bucket_hour >= $1 AND bucket_hour < $2`
	args := []interface{}{from, to}
	if instanceID != "" {
		q += " AND instance_id = $3"
		args = append(args, instanceID)
	}
	q += " ORDER BY bucket_hour DESC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query aggregates: %w", err)
	}
	defer rows.Close()

	var result []AggregateRow
	for rows.Next() {
		var r AggregateRow
		if err := rows.Scan(
			&r.InstanceID, &r.APIKey, &r.Model, &r.BucketHour,
			&r.TotalRequests, &r.SuccessCount, &r.FailureCount,
			&r.InputTokens, &r.OutputTokens, &r.ReasoningTokens,
			&r.CachedTokens, &r.TotalTokens,
		); err != nil {
			return nil, fmt.Errorf("query aggregates scan: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// QueryDetails returns detail rows within the given time range, limited by limit.
func (s *PGStore) QueryDetails(from, to time.Time, limit int) ([]DetailRow, error) {
	return s.QueryDetailsByRange(from, to, "", limit)
}

// QueryDetailsByRange returns detail rows within the given time range.
// If instanceID is empty, all instances are included.
// If limit <= 0, no limit is applied.
func (s *PGStore) QueryDetailsByRange(from, to time.Time, instanceID string, limit int) ([]DetailRow, error) {
	q := `SELECT api_key, model, timestamp, source, auth_index,
		failed, input_tokens, output_tokens, reasoning_tokens,
		cached_tokens, total_tokens
		FROM usage_details WHERE timestamp >= $1 AND timestamp < $2`
	args := []interface{}{from, to}
	if instanceID != "" {
		q += fmt.Sprintf(" AND instance_id = $%d", len(args)+1)
		args = append(args, instanceID)
	}
	q += " ORDER BY timestamp DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query details: %w", err)
	}
	defer rows.Close()
	var result []DetailRow
	for rows.Next() {
		var dr DetailRow
		var d RequestDetail
		if err := rows.Scan(
			&dr.APIKey, &dr.Model, &d.Timestamp, &d.Source, &d.AuthIndex,
			&d.Failed, &d.Tokens.InputTokens, &d.Tokens.OutputTokens,
			&d.Tokens.ReasoningTokens, &d.Tokens.CachedTokens,
			&d.Tokens.TotalTokens,
		); err != nil {
			return nil, fmt.Errorf("query details scan: %w", err)
		}
		dr.Detail = d
		result = append(result, dr)
	}
	return result, rows.Err()
}

// QueryTotals returns summed counters across all aggregate rows.
// If instanceID is empty, all instances are summed.
func (s *PGStore) QueryTotals(instanceID string) (totalRequests, successCount, failureCount, totalTokens int64, err error) {
	q := `SELECT COALESCE(SUM(total_requests),0),
		COALESCE(SUM(success_count),0),
		COALESCE(SUM(failure_count),0),
		COALESCE(SUM(total_tokens),0)
		FROM usage_aggregates`
	var args []interface{}
	if instanceID != "" {
		q += " WHERE instance_id = $1"
		args = append(args, instanceID)
	}
	err = s.db.QueryRow(q, args...).Scan(
		&totalRequests, &successCount, &failureCount, &totalTokens,
	)
	if err != nil {
		err = fmt.Errorf("query totals: %w", err)
	}
	return
}

// QuerySnapshot builds a StatisticsSnapshot from aggregate data,
// compatible with the existing in-memory Snapshot format.
func (s *PGStore) QuerySnapshot(from, to time.Time, instanceID string) (StatisticsSnapshot, error) {
	aggs, err := s.QueryAggregates(from, to, instanceID)
	if err != nil {
		return StatisticsSnapshot{}, err
	}
	details, err := s.QueryDetailsByRange(from, to, instanceID, defaultSnapshotDetailsLimit)
	if err != nil {
		return StatisticsSnapshot{}, err
	}
	return buildSnapshotFromRows(aggs, details), nil
}

// buildSnapshotFromRows converts aggregate and detail rows into a snapshot.
func buildSnapshotFromRows(aggs []AggregateRow, details []DetailRow) StatisticsSnapshot {
	snap := StatisticsSnapshot{
		APIs:           make(map[string]APISnapshot),
		RequestsByDay:  make(map[string]int64),
		RequestsByHour: make(map[string]int64),
		TokensByDay:    make(map[string]int64),
		TokensByHour:   make(map[string]int64),
	}
	for _, a := range aggs {
		snap.TotalRequests += a.TotalRequests
		snap.SuccessCount += a.SuccessCount
		snap.FailureCount += a.FailureCount
		snap.TotalTokens += a.TotalTokens
		buildAPIs(&snap, a)
		buildTimeBuckets(&snap, a)
	}
	for _, d := range details {
		buildDetails(&snap, d)
	}
	return snap
}

// buildAPIs populates the APIs map in the snapshot from an aggregate row.
func buildAPIs(snap *StatisticsSnapshot, a AggregateRow) {
	api, ok := snap.APIs[a.APIKey]
	if !ok {
		api = APISnapshot{Models: make(map[string]ModelSnapshot)}
	}
	api.TotalRequests += a.TotalRequests
	api.TotalTokens += a.TotalTokens
	m := api.Models[a.Model]
	m.TotalRequests += a.TotalRequests
	m.TotalTokens += a.TotalTokens
	api.Models[a.Model] = m
	snap.APIs[a.APIKey] = api
}

// buildDetails populates model details in the snapshot from a detail row.
func buildDetails(snap *StatisticsSnapshot, d DetailRow) {
	api, ok := snap.APIs[d.APIKey]
	if !ok {
		api = APISnapshot{Models: make(map[string]ModelSnapshot)}
	}
	m, ok := api.Models[d.Model]
	if !ok {
		m = ModelSnapshot{}
	}
	m.Details = append(m.Details, d.Detail)
	api.Models[d.Model] = m
	snap.APIs[d.APIKey] = api
}

// buildTimeBuckets populates day/hour maps in the snapshot from an aggregate row.
func buildTimeBuckets(snap *StatisticsSnapshot, a AggregateRow) {
	day := a.BucketHour.Format("2006-01-02")
	hour := formatHour(a.BucketHour.Hour())
	snap.RequestsByDay[day] += a.TotalRequests
	snap.RequestsByHour[hour] += a.TotalRequests
	snap.TokensByDay[day] += a.TotalTokens
	snap.TokensByHour[hour] += a.TotalTokens
}
