package cpadashboard

import (
	"math"
	"testing"
	"time"
)

func TestComputeQuotaConsumptionCountsOnlyDrops(t *testing.T) {
	base := time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC)
	points := []QuotaPoint{
		{CPASource: "cpa1", AuthFile: "a.json", CollectedAt: base, WeeklyRemain: 90},
		{CPASource: "cpa1", AuthFile: "a.json", CollectedAt: base.Add(time.Hour), WeeklyRemain: 80},
		{CPASource: "cpa1", AuthFile: "a.json", CollectedAt: base.Add(2 * time.Hour), WeeklyRemain: 100},
		{CPASource: "cpa1", AuthFile: "a.json", CollectedAt: base.Add(3 * time.Hour), WeeklyRemain: 70},
	}

	got := computeQuotaConsumption(points)["cpa1"]
	if got.ConsumptionPercent != 40 {
		t.Fatalf("consumption = %.2f, want 40", got.ConsumptionPercent)
	}
	if got.AccountCount != 1 {
		t.Fatalf("account count = %d, want 1", got.AccountCount)
	}
}

func TestComputeQuotaConsumptionCanExceedOneHundredAcrossAccounts(t *testing.T) {
	base := time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC)
	points := []QuotaPoint{
		{CPASource: "cpa3", AuthFile: "a.json", AccountEmail: "one@example.com", CollectedAt: base, WeeklyRemain: 100},
		{CPASource: "cpa3", AuthFile: "a.json", AccountEmail: "one@example.com", CollectedAt: base.Add(time.Hour), WeeklyRemain: 20},
		{CPASource: "cpa3", AuthFile: "b.json", AccountEmail: "two@example.com", CollectedAt: base, WeeklyRemain: 100},
		{CPASource: "cpa3", AuthFile: "b.json", AccountEmail: "two@example.com", CollectedAt: base.Add(time.Hour), WeeklyRemain: 45},
	}

	got := computeQuotaConsumption(points)["cpa3"]
	if got.ConsumptionPercent != 135 {
		t.Fatalf("consumption = %.2f, want 135", got.ConsumptionPercent)
	}
	if got.AccountCount != 2 {
		t.Fatalf("account count = %d, want 2", got.AccountCount)
	}
}

func TestComputeQuotaConsumptionGroupsSameEmailAcrossAuthFiles(t *testing.T) {
	base := time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC)
	points := []QuotaPoint{
		{CPASource: "cpa2", AuthFile: "old.json", AccountEmail: "same@example.com", CollectedAt: base, WeeklyRemain: 90},
		{CPASource: "cpa2", AuthFile: "new.json", AccountEmail: "same@example.com", CollectedAt: base, WeeklyRemain: 95},
		{CPASource: "cpa2", AuthFile: "new.json", AccountEmail: "same@example.com", CollectedAt: base.Add(time.Hour), WeeklyRemain: 80},
	}

	got := computeQuotaConsumption(points)["cpa2"]
	if got.ConsumptionPercent != 15 {
		t.Fatalf("consumption = %.2f, want 15", got.ConsumptionPercent)
	}
	if got.AccountCount != 1 {
		t.Fatalf("account count = %d, want 1", got.AccountCount)
	}
}

func TestBuildEfficiencyRowNormalizesToFullWeeklyQuota(t *testing.T) {
	row := buildEfficiencyRow("cpa3", UsageSummary{
		CPASource:           "cpa3",
		RequestCount:        2,
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 200,
		CacheReadTokens:     300,
		TotalCost:           1.5,
		ActualCost:          1.2,
	}, QuotaConsumption{CPASource: "cpa3", ConsumptionPercent: 25})

	if row.Per100.InputTokens != 4000 {
		t.Fatalf("per100 input = %.2f, want 4000", row.Per100.InputTokens)
	}
	if row.MonthlyEstimate.InputTokens != 17200 {
		t.Fatalf("monthly input = %.2f, want 17200", row.MonthlyEstimate.InputTokens)
	}
	if math.Abs(row.Per100.TotalCost-6) > 0.0001 {
		t.Fatalf("per100 total cost = %.4f, want 6", row.Per100.TotalCost)
	}
	if math.Abs(row.MonthlyEstimate.TotalCost-25.8) > 0.0001 {
		t.Fatalf("monthly total cost = %.4f, want 25.8", row.MonthlyEstimate.TotalCost)
	}
}
