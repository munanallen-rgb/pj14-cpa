package quotacollector

import (
	"encoding/json"
	"strings"
	"time"
)

type quotaReportResponse struct {
	Provider    string               `json:"provider"`
	GeneratedAt time.Time            `json:"generated_at"`
	Summary     quotaReportSummary   `json:"summary"`
	Accounts    []quotaReportAccount `json:"accounts"`
}

type quotaReportSummary struct {
	Total   int `json:"total"`
	Valid   int `json:"valid"`
	Invalid int `json:"invalid"`
}

type quotaReportAccount struct {
	AuthIndex        string          `json:"auth_index"`
	AuthFile         string          `json:"auth_file"`
	Provider         string          `json:"provider"`
	Display          string          `json:"display"`
	Status           string          `json:"status"`
	Reason           string          `json:"reason"`
	Subscription     json.RawMessage `json:"subscription"`
	RuntimeQuota     json.RawMessage `json:"runtime_quota"`
	Quota            quotaMetadata   `json:"quota"`
	QuotaRaw         json.RawMessage `json:"-"`
	RecentSuccessful int64           `json:"recent_successful"`
	RecentFailed     int64           `json:"recent_failed"`
}

type quotaMetadata struct {
	Known    bool          `json:"known"`
	Type     string        `json:"type"`
	Message  string        `json:"message"`
	PlanType string        `json:"plan_type"`
	Windows  []quotaWindow `json:"windows"`
}

type quotaWindow struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	UsedPercent      *float64 `json:"used_percent"`
	RemainingPercent *float64 `json:"remaining_percent"`
	ResetAt          string   `json:"reset_at"`
	ResetLabel       string   `json:"reset_label"`
}

func (a *quotaReportAccount) UnmarshalJSON(data []byte) error {
	type alias quotaReportAccount
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	tmp.QuotaRaw = raw["quota"]
	*a = quotaReportAccount(tmp)
	return nil
}

func snapshotsFromReport(instance string, reason string, collectedAt time.Time, report quotaReportResponse) []Snapshot {
	out := make([]Snapshot, 0, len(report.Accounts))
	for _, account := range report.Accounts {
		out = append(out, snapshotFromAccount(instance, reason, collectedAt, account))
	}
	return out
}

func snapshotFromAccount(instance string, reason string, collectedAt time.Time, account quotaReportAccount) Snapshot {
	accountPlan := strings.ToLower(strings.TrimSpace(account.Quota.PlanType))
	subPlan, subscriptionExpired := planFromSubscription(account.Subscription)
	if accountPlan == "" {
		accountPlan = subPlan
	}
	if accountPlan == "" {
		accountPlan = "unknown"
	}

	fiveHour, weekly := quotaWindow{}, quotaWindow{}
	for _, window := range account.Quota.Windows {
		id := strings.ToLower(strings.TrimSpace(window.ID))
		label := strings.ToLower(strings.TrimSpace(window.Label))
		switch {
		case fiveHour.ID == "" && (id == "five-hour" || label == "5h" || strings.Contains(label, "five")):
			fiveHour = window
		case weekly.ID == "" && (id == "weekly" || strings.Contains(label, "weekly")):
			weekly = window
		}
	}

	fiveHourReset := parseReportTime(fiveHour.ResetAt)
	weeklyReset := parseReportTime(weekly.ResetAt)
	status := StatusSuccess
	errorCategory := ""
	errorMessage := ""
	dataStale := false
	if subscriptionExpired {
		status = StatusError
		errorCategory = ErrorSubscriptionExpired
		errorMessage = "subscription expired"
		dataStale = true
	} else if !strings.EqualFold(account.Status, "valid") {
		status = StatusError
		errorCategory = classifyAccountError(account.Reason)
		errorMessage = account.Reason
		dataStale = true
	} else if !account.Quota.Known || len(account.Quota.Windows) == 0 {
		status = StatusError
		errorCategory = ErrorQuotaUnavailable
		errorMessage = firstNonEmpty(account.Quota.Message, "quota unavailable")
		dataStale = true
	}

	return Snapshot{
		CollectedAt:              collectedAt,
		CPASource:                instance,
		AuthFile:                 firstNonEmpty(account.AuthFile, account.AuthIndex),
		AuthIndex:                account.AuthIndex,
		AccountDisplay:           account.Display,
		AccountPlan:              accountPlan,
		Status:                   status,
		WeeklyRemainingPercent:   weekly.RemainingPercent,
		FiveHourRemainingPercent: fiveHour.RemainingPercent,
		WeeklyResetAt:            weeklyReset,
		FiveHourResetAt:          fiveHourReset,
		QuotaJSON:                nullableJSON(account.QuotaRaw),
		CollectionReason:         reason,
		ErrorCategory:            errorCategory,
		ErrorMessage:             errorMessage,
		ErrorRaw:                 accountErrorRaw(account),
		DataStale:                dataStale,
	}
}

func planFromSubscription(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var sub struct {
		PlanType string `json:"plan_type"`
		Expired  bool   `json:"expired"`
	}
	if err := json.Unmarshal(raw, &sub); err != nil {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(sub.PlanType)), sub.Expired
}

func classifyAccountError(reason string) string {
	lower := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case lower == "":
		return ErrorUnknown
	case strings.Contains(lower, "missing access token") || strings.Contains(lower, "invalid management key") || strings.Contains(lower, "unauthorized"):
		return ErrorAuthExpired
	case strings.Contains(lower, "expired") || strings.Contains(lower, "subscription"):
		return ErrorSubscriptionExpired
	case strings.Contains(lower, "http"):
		return ErrorHTTP
	case strings.Contains(lower, "json") || strings.Contains(lower, "parse"):
		return ErrorParse
	case strings.Contains(lower, "quota"):
		return ErrorQuotaUnavailable
	default:
		return ErrorUnknown
	}
}

func parseReportTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	ts = ts.UTC()
	return &ts
}

func accountErrorRaw(account quotaReportAccount) json.RawMessage {
	if account.Reason == "" {
		return nil
	}
	raw, err := json.Marshal(map[string]string{"reason": account.Reason})
	if err != nil {
		return nil
	}
	return raw
}

func nullableJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
