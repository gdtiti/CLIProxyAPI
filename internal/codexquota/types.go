package codexquota

import "time"

// Snapshot stores the latest persisted state for a Codex auth account.
type Snapshot struct {
	AuthID           string    `json:"auth_id"`
	AuthIndex        string    `json:"auth_index"`
	Provider         string    `json:"provider"`
	FileName         string    `json:"file_name,omitempty"`
	Label            string    `json:"label,omitempty"`
	AccountType      string    `json:"account_type,omitempty"`
	Account          string    `json:"account,omitempty"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	Status           string    `json:"status"`
	StatusMessage    string    `json:"status_message,omitempty"`
	LastErrorMessage string    `json:"last_error_message,omitempty"`
	Disabled         bool      `json:"disabled"`
	Unavailable      bool      `json:"unavailable"`
	QuotaExceeded    bool      `json:"quota_exceeded"`
	QuotaReason      string    `json:"quota_reason,omitempty"`
	QuotaModel       string    `json:"quota_model,omitempty"`
	NextRecoverAt    time.Time `json:"next_recover_at,omitempty"`
	LastRefreshedAt  time.Time `json:"last_refreshed_at,omitempty"`
	NextRefreshAfter time.Time `json:"next_refresh_after,omitempty"`
	NextRetryAfter   time.Time `json:"next_retry_after,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// UsageRollup stores persisted request and token aggregates per auth account.
type UsageRollup struct {
	AuthID          string    `json:"auth_id"`
	AuthIndex       string    `json:"auth_index"`
	Provider        string    `json:"provider"`
	Account         string    `json:"account,omitempty"`
	RequestCount    int64     `json:"request_count"`
	InputTokens     int64     `json:"input_tokens"`
	OutputTokens    int64     `json:"output_tokens"`
	CachedTokens    int64     `json:"cached_tokens"`
	ReasoningTokens int64     `json:"reasoning_tokens"`
	TotalTokens     int64     `json:"total_tokens"`
	RecoveredTokens *int64    `json:"recovered_tokens,omitempty"`
	AvgInputTokens  float64   `json:"avg_input_tokens"`
	AvgOutputTokens float64   `json:"avg_output_tokens"`
	AvgCachedTokens float64   `json:"avg_cached_tokens"`
	AvgReasoning    float64   `json:"avg_reasoning_tokens"`
	AvgTotalTokens  float64   `json:"avg_total_tokens"`
	LastRequestedAt time.Time `json:"last_requested_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// SnapshotView combines the latest snapshot with persisted usage aggregates.
type SnapshotView struct {
	Snapshot
	Usage UsageRollup `json:"usage"`
}

// Event stores persisted account lifecycle and quota history.
type Event struct {
	ID              string     `json:"id"`
	AuthID          string     `json:"auth_id"`
	AuthIndex       string     `json:"auth_index"`
	Provider        string     `json:"provider"`
	EventType       string     `json:"event_type"`
	Reason          string     `json:"reason,omitempty"`
	StatusMessage   string     `json:"status_message,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	HTTPStatus      int        `json:"http_status,omitempty"`
	Disabled        bool       `json:"disabled"`
	Unavailable     bool       `json:"unavailable"`
	QuotaExceeded   bool       `json:"quota_exceeded"`
	QuotaReason     string     `json:"quota_reason,omitempty"`
	QuotaModel      string     `json:"quota_model,omitempty"`
	DisabledAt      *time.Time `json:"disabled_at,omitempty"`
	EnabledAt       *time.Time `json:"enabled_at,omitempty"`
	RecoverAt       *time.Time `json:"recover_at,omitempty"`
	RequestCount    int64      `json:"request_count"`
	InputTokens     int64      `json:"input_tokens"`
	OutputTokens    int64      `json:"output_tokens"`
	CachedTokens    int64      `json:"cached_tokens"`
	ReasoningTokens int64      `json:"reasoning_tokens"`
	TotalTokens     int64      `json:"total_tokens"`
	RecoveredTokens *int64     `json:"recovered_tokens,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}
