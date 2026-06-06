package quotacollector

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSnapshotFromAccountParsesValidQuota(t *testing.T) {
	account := quotaReportAccount{
		AuthIndex:    "codex-1",
		AuthFile:     "codex-1.json",
		Display:      "person@example.com",
		Status:       "valid",
		Subscription: json.RawMessage(`{"plan_type":"pro","expired":false}`),
		Quota: quotaMetadata{
			Known:    true,
			PlanType: "plus",
			Windows: []quotaWindow{
				{ID: "five-hour", Label: "5h", RemainingPercent: floatPtr(44), ResetAt: "2026-06-05T12:30:00Z"},
				{ID: "weekly", Label: "weekly", RemainingPercent: floatPtr(73), ResetAt: "2026-06-11T02:05:00Z"},
			},
		},
		QuotaRaw: json.RawMessage(`{"known":true,"plan_type":"plus","windows":[]}`),
	}

	snapshot := snapshotFromAccount("cpa1", ReasonHourly, time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC), account)
	if snapshot.Status != StatusSuccess {
		t.Fatalf("status = %s, want %s", snapshot.Status, StatusSuccess)
	}
	if snapshot.AccountPlan != "plus" {
		t.Fatalf("account plan = %s, want plus", snapshot.AccountPlan)
	}
	if snapshot.FiveHourRemainingPercent == nil || *snapshot.FiveHourRemainingPercent != 44 {
		t.Fatalf("five hour remaining = %#v, want 44", snapshot.FiveHourRemainingPercent)
	}
	if snapshot.WeeklyRemainingPercent == nil || *snapshot.WeeklyRemainingPercent != 73 {
		t.Fatalf("weekly remaining = %#v, want 73", snapshot.WeeklyRemainingPercent)
	}
	if snapshot.FiveHourResetAt == nil || snapshot.FiveHourResetAt.Format(time.RFC3339) != "2026-06-05T12:30:00Z" {
		t.Fatalf("five hour reset = %#v", snapshot.FiveHourResetAt)
	}
	if snapshot.WeeklyResetAt == nil || snapshot.WeeklyResetAt.Format(time.RFC3339) != "2026-06-11T02:05:00Z" {
		t.Fatalf("weekly reset = %#v", snapshot.WeeklyResetAt)
	}
}

func TestSnapshotFromAccountMissingQuotaIsError(t *testing.T) {
	account := quotaReportAccount{
		AuthIndex: "codex-1",
		Status:    "valid",
		Quota: quotaMetadata{
			Known:   false,
			Message: "quota unavailable",
		},
	}

	snapshot := snapshotFromAccount("cpa1", ReasonHourly, time.Now(), account)
	if snapshot.Status != StatusError {
		t.Fatalf("status = %s, want error", snapshot.Status)
	}
	if snapshot.ErrorCategory != ErrorQuotaUnavailable {
		t.Fatalf("error category = %s, want %s", snapshot.ErrorCategory, ErrorQuotaUnavailable)
	}
	if !snapshot.DataStale {
		t.Fatalf("data_stale = false, want true")
	}
}

func TestSnapshotFromAccountSubscriptionExpiredIsError(t *testing.T) {
	account := quotaReportAccount{
		AuthIndex:    "codex-1",
		Status:       "valid",
		Subscription: json.RawMessage(`{"plan_type":"plus","expired":true}`),
		Quota: quotaMetadata{
			Known: true,
			Windows: []quotaWindow{
				{ID: "five-hour", RemainingPercent: floatPtr(50)},
			},
		},
	}

	snapshot := snapshotFromAccount("cpa1", ReasonHourly, time.Now(), account)
	if snapshot.Status != StatusError {
		t.Fatalf("status = %s, want error", snapshot.Status)
	}
	if snapshot.ErrorCategory != ErrorSubscriptionExpired {
		t.Fatalf("error category = %s, want %s", snapshot.ErrorCategory, ErrorSubscriptionExpired)
	}
}

func TestClassifyAccountError(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"missing access token", ErrorAuthExpired},
		{"quota response is not JSON", ErrorParse},
		{"quota request failed: HTTP Unauthorized", ErrorAuthExpired},
		{"subscription expired", ErrorSubscriptionExpired},
		{"quota unavailable", ErrorQuotaUnavailable},
		{"something else", ErrorUnknown},
	}
	for _, tc := range cases {
		if got := classifyAccountError(tc.reason); got != tc.want {
			t.Fatalf("classifyAccountError(%q) = %s, want %s", tc.reason, got, tc.want)
		}
	}
}

func floatPtr(v float64) *float64 { return &v }
