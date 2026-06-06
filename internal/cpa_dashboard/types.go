package cpadashboard

import "time"

const (
	defaultRangeDays     = 7
	maxRangeDays         = 90
	sampleWarningPercent = 10
)

// QueryFilter describes the dashboard's shared report filters.
type QueryFilter struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	CPA      string    `json:"cpa_source"`
	Model    string    `json:"model"`
	APIKeyID int64     `json:"api_key_id"`
}

// QuotaObservationWindow describes the successful quota samples available for one CPA.
type QuotaObservationWindow struct {
	CPASource         string
	FirstCollectedAt  time.Time
	LatestCollectedAt time.Time
}

// EffectiveQuotaWindow describes the time range that can be safely aligned.
type EffectiveQuotaWindow struct {
	CPASource        string    `json:"cpa_source"`
	RequestedStart   time.Time `json:"requested_start"`
	RequestedEnd     time.Time `json:"requested_end"`
	EffectiveStart   time.Time `json:"effective_start"`
	EffectiveEnd     time.Time `json:"effective_end"`
	QuotaSampleCount int64     `json:"quota_sample_count"`
	Aligned          bool      `json:"aligned"`
	Warning          string    `json:"warning,omitempty"`
}

// UsageSummary stores Sub2API usage totals.
type UsageSummary struct {
	CPASource             string  `json:"cpa_source"`
	RequestCount          int64   `json:"request_count"`
	InputTokens           int64   `json:"input_tokens"`
	OutputTokens          int64   `json:"output_tokens"`
	CacheCreationTokens   int64   `json:"cache_creation_tokens"`
	CacheReadTokens       int64   `json:"cache_read_tokens"`
	CacheCreation5MTokens int64   `json:"cache_creation_5m_tokens"`
	CacheCreation1HTokens int64   `json:"cache_creation_1h_tokens"`
	TotalCost             float64 `json:"total_cost"`
	ActualCost            float64 `json:"actual_cost"`
	AverageDurationMS     float64 `json:"average_duration_ms"`
}

// QuotaPoint stores one successful weekly quota sample.
type QuotaPoint struct {
	CPASource    string
	AuthFile     string
	AccountEmail string
	CollectedAt  time.Time
	WeeklyRemain float64
}

// QuotaConsumption stores a CPA's cumulative weekly quota consumption.
type QuotaConsumption struct {
	CPASource          string  `json:"cpa_source"`
	ConsumptionPercent float64 `json:"weekly_consumption_percent"`
	SampleCount        int64   `json:"sample_count"`
	AccountCount       int64   `json:"account_count"`
}

// EfficiencyRow is the main quota-to-usage report row.
type EfficiencyRow struct {
	CPASource             string               `json:"cpa_source"`
	EffectiveStart        *time.Time           `json:"effective_start,omitempty"`
	EffectiveEnd          *time.Time           `json:"effective_end,omitempty"`
	QuotaSampleCount      int64                `json:"quota_sample_count"`
	WeeklyConsumption     float64              `json:"weekly_consumption_percent"`
	RequestCount          int64                `json:"request_count"`
	InputTokens           int64                `json:"input_tokens"`
	OutputTokens          int64                `json:"output_tokens"`
	CacheCreationTokens   int64                `json:"cache_creation_tokens"`
	CacheReadTokens       int64                `json:"cache_read_tokens"`
	CacheCreation5MTokens int64                `json:"cache_creation_5m_tokens"`
	CacheCreation1HTokens int64                `json:"cache_creation_1h_tokens"`
	TotalTokens           int64                `json:"total_tokens"`
	TotalCost             float64              `json:"total_cost"`
	ActualCost            float64              `json:"actual_cost"`
	AverageDurationMS     float64              `json:"average_duration_ms"`
	Per100                EfficiencyProjection `json:"per_100_percent"`
	MonthlyEstimate       EfficiencyProjection `json:"monthly_estimate"`
	SampleWarning         string               `json:"sample_warning,omitempty"`
}

// EfficiencyProjection stores normalized token and cost estimates.
type EfficiencyProjection struct {
	InputTokens         float64 `json:"input_tokens"`
	OutputTokens        float64 `json:"output_tokens"`
	CacheCreationTokens float64 `json:"cache_creation_tokens"`
	CacheReadTokens     float64 `json:"cache_read_tokens"`
	TotalTokens         float64 `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
	ActualCost          float64 `json:"actual_cost"`
}

// EfficiencyResponse is returned by /api/quota-efficiency.
type EfficiencyResponse struct {
	Filter             QueryFilter            `json:"filter"`
	Rows               []EfficiencyRow        `json:"rows"`
	Total              EfficiencyRow          `json:"total"`
	Windows            []EffectiveQuotaWindow `json:"windows"`
	RequestedStart     time.Time              `json:"requested_start"`
	RequestedEnd       time.Time              `json:"requested_end"`
	EffectiveStart     *time.Time             `json:"effective_start,omitempty"`
	EffectiveEnd       *time.Time             `json:"effective_end,omitempty"`
	AlignmentNotice    string                 `json:"alignment_notice,omitempty"`
	AttributionWarning string                 `json:"attribution_warning,omitempty"`
	GeneratedAt        time.Time              `json:"generated_at"`
}

// OverviewResponse is returned by /api/overview.
type OverviewResponse struct {
	GeneratedAt             time.Time       `json:"generated_at"`
	LatestCollectionAt      *time.Time      `json:"latest_collection_at,omitempty"`
	CurrentAccounts         []CPAAccountRow `json:"current_accounts"`
	CurrentSuccessAccounts  int64           `json:"current_success_accounts"`
	CurrentErrorAccounts    int64           `json:"current_error_accounts"`
	TodayUsage              EfficiencyRow   `json:"today_usage"`
	SevenDayEfficiencyTotal EfficiencyRow   `json:"seven_day_efficiency_total"`
	TodayAlignmentNotice    string          `json:"today_alignment_notice,omitempty"`
	SevenDayAlignmentNotice string          `json:"seven_day_alignment_notice,omitempty"`
}

// CPAAccountRow describes one current CPA account health row.
type CPAAccountRow struct {
	CPASource                  string     `json:"cpa_source"`
	AccountEmail               string     `json:"account_email"`
	AuthFile                   string     `json:"auth_file"`
	AccountPlan                string     `json:"account_plan"`
	Status                     string     `json:"status"`
	WeeklyRemainingPercent     *float64   `json:"weekly_remaining_percent,omitempty"`
	FiveHourRemainingPercent   *float64   `json:"five_hour_remaining_percent,omitempty"`
	WeeklyResetAt              *time.Time `json:"weekly_reset_at,omitempty"`
	FiveHourResetAt            *time.Time `json:"five_hour_reset_at,omitempty"`
	CollectedAt                time.Time  `json:"collected_at"`
	DataStale                  bool       `json:"data_stale"`
	ErrorCategory              string     `json:"error_category,omitempty"`
	ErrorMessage               string     `json:"error_message,omitempty"`
	AlternativeSuccessAuthFile string     `json:"alternative_success_auth_file,omitempty"`
}

// UsageBucket describes one time bucket for usage charts.
type UsageBucket struct {
	BucketStart         time.Time `json:"bucket_start"`
	CPASource           string    `json:"cpa_source"`
	RequestCount        int64     `json:"request_count"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	TotalCost           float64   `json:"total_cost"`
	ActualCost          float64   `json:"actual_cost"`
}

// CleanupCandidate describes a read-only auth cleanup candidate.
type CleanupCandidate struct {
	CPASource                 string    `json:"cpa_source"`
	AccountEmail              string    `json:"account_email"`
	AuthFile                  string    `json:"auth_file"`
	Status                    string    `json:"status"`
	ErrorCategory             string    `json:"error_category,omitempty"`
	ErrorMessage              string    `json:"error_message,omitempty"`
	LatestCollectedAt         time.Time `json:"latest_collected_at"`
	FailureSnapshotsLast30Day int64     `json:"failure_snapshots_last_30d"`
	HasSuccessSameEmail       bool      `json:"has_success_same_email"`
	Reason                    string    `json:"reason"`
}

// FilterOption is a generic UI filter option.
type FilterOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// FiltersResponse contains available dashboard filters.
type FiltersResponse struct {
	CPASources []FilterOption `json:"cpa_sources"`
	Models     []FilterOption `json:"models"`
	APIKeys    []FilterOption `json:"api_keys"`
}
