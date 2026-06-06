package quotacollector

import (
	"encoding/json"
	"time"
)

const (
	StatusSuccess = "success"
	StatusError   = "error"

	RunStatusSuccess = "success"
	RunStatusPartial = "partial"
	RunStatusError   = "error"

	ReasonStartup     = "startup"
	ReasonHourly      = "hourly"
	ReasonResetBefore = "reset_before"
	ReasonResetAfter  = "reset_after"
	ReasonManual      = "manual"

	ErrorAuthExpired         = "auth_expired"
	ErrorSubscriptionExpired = "subscription_expired"
	ErrorQuotaUnavailable    = "quota_unavailable"
	ErrorNetwork             = "network_error"
	ErrorHTTP                = "http_error"
	ErrorParse               = "parse_error"
	ErrorInstanceUnavailable = "instance_unavailable"
	ErrorUnknown             = "unknown_error"
)

// CollectionRun records one collector cycle across configured CPA instances.
type CollectionRun struct {
	ID                  int64
	StartedAt           time.Time
	FinishedAt          time.Time
	Reason              string
	Status              string
	AttemptedInstances  int
	SuccessfulInstances int
	FailedInstances     int
	ErrorMessage        string
}

// Snapshot records one account state observed during one collection run.
type Snapshot struct {
	RunID                    int64
	CollectedAt              time.Time
	CPASource                string
	AuthFile                 string
	AuthIndex                string
	AccountDisplay           string
	AccountPlan              string
	Status                   string
	WeeklyRemainingPercent   *float64
	FiveHourRemainingPercent *float64
	WeeklyResetAt            *time.Time
	FiveHourResetAt          *time.Time
	QuotaJSON                json.RawMessage
	CollectionReason         string
	ErrorCategory            string
	ErrorMessage             string
	ErrorRaw                 json.RawMessage
	DataStale                bool
}

// ResetTask is an in-memory request to collect near a quota reset time.
type ResetTask struct {
	ExecuteAt time.Time
	Reason    string
}
