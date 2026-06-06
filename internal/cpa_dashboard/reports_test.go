package cpadashboard

import (
	"context"
	"testing"
	"time"
)

type fakeDashboardStore struct {
	observations []QuotaObservationWindow
	usageFilters []QueryFilter
	quotaFilters []QueryFilter
}

func (f *fakeDashboardStore) QuotaObservationWindows(_ context.Context, _ QueryFilter) ([]QuotaObservationWindow, error) {
	return f.observations, nil
}

func (f *fakeDashboardStore) QuotaPoints(_ context.Context, filter QueryFilter) ([]QuotaPoint, error) {
	f.quotaFilters = append(f.quotaFilters, filter)
	return []QuotaPoint{
		{CPASource: filter.CPA, AuthFile: "a.json", AccountEmail: "a@example.com", CollectedAt: filter.Start, WeeklyRemain: 100},
		{CPASource: filter.CPA, AuthFile: "a.json", AccountEmail: "a@example.com", CollectedAt: filter.End, WeeklyRemain: 90},
	}, nil
}

func (f *fakeDashboardStore) UsageSummaries(_ context.Context, filter QueryFilter) ([]UsageSummary, error) {
	f.usageFilters = append(f.usageFilters, filter)
	return []UsageSummary{{
		CPASource:    filter.CPA,
		RequestCount: 10,
		InputTokens:  100,
		OutputTokens: 50,
		TotalCost:    1,
		ActualCost:   1,
	}}, nil
}

func (f *fakeDashboardStore) LatestCollectionAt(context.Context) (*time.Time, error) {
	return nil, nil
}

func (f *fakeDashboardStore) CurrentAccounts(context.Context, string) ([]CPAAccountRow, error) {
	return nil, nil
}

func (f *fakeDashboardStore) UsageBuckets(context.Context, QueryFilter) ([]UsageBucket, error) {
	return nil, nil
}

func (f *fakeDashboardStore) CleanupCandidates(context.Context, string) ([]CleanupCandidate, error) {
	return nil, nil
}

func (f *fakeDashboardStore) Filters(context.Context) (FiltersResponse, error) {
	return FiltersResponse{}, nil
}

func TestQuotaEfficiencyAlignsUsageToQuotaObservationWindow(t *testing.T) {
	requestedStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	quotaStart := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	quotaEnd := time.Date(2026, 6, 6, 9, 0, 0, 0, time.UTC)
	requestedEnd := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)

	store := &fakeDashboardStore{observations: []QuotaObservationWindow{{
		CPASource:         "cpa1",
		FirstCollectedAt:  quotaStart,
		LatestCollectedAt: quotaEnd,
	}}}
	service := NewService(store)

	resp, errReport := service.QuotaEfficiency(context.Background(), QueryFilter{
		Start: requestedStart,
		End:   requestedEnd,
	})
	if errReport != nil {
		t.Fatalf("quota efficiency: %v", errReport)
	}
	if len(store.usageFilters) != 1 {
		t.Fatalf("usage query count = %d, want 1", len(store.usageFilters))
	}
	usageFilter := store.usageFilters[0]
	if !usageFilter.Start.Equal(quotaStart) || !usageFilter.End.Equal(quotaEnd) {
		t.Fatalf("usage window = %s to %s, want %s to %s", usageFilter.Start, usageFilter.End, quotaStart, quotaEnd)
	}
	if len(store.quotaFilters) != 1 {
		t.Fatalf("quota query count = %d, want 1", len(store.quotaFilters))
	}
	quotaFilter := store.quotaFilters[0]
	if !quotaFilter.Start.Equal(quotaStart) || !quotaFilter.End.Equal(quotaEnd) {
		t.Fatalf("quota window = %s to %s, want %s to %s", quotaFilter.Start, quotaFilter.End, quotaStart, quotaEnd)
	}
	if resp.AlignmentNotice == "" {
		t.Fatalf("expected alignment notice")
	}
	if resp.EffectiveStart == nil || !resp.EffectiveStart.Equal(quotaStart) {
		t.Fatalf("effective start = %v, want %s", resp.EffectiveStart, quotaStart)
	}
	if resp.EffectiveEnd == nil || !resp.EffectiveEnd.Equal(quotaEnd) {
		t.Fatalf("effective end = %v, want %s", resp.EffectiveEnd, quotaEnd)
	}
}

func TestQuotaEfficiencySkipsUsageWhenNoCompleteQuotaWindow(t *testing.T) {
	requestedStart := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	quotaLatest := time.Date(2026, 6, 5, 23, 0, 0, 0, time.UTC)
	requestedEnd := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)

	store := &fakeDashboardStore{observations: []QuotaObservationWindow{{
		CPASource:         "cpa1",
		FirstCollectedAt:  time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC),
		LatestCollectedAt: quotaLatest,
	}}}
	service := NewService(store)

	resp, errReport := service.QuotaEfficiency(context.Background(), QueryFilter{
		Start: requestedStart,
		End:   requestedEnd,
	})
	if errReport != nil {
		t.Fatalf("quota efficiency: %v", errReport)
	}
	if len(store.usageFilters) != 0 {
		t.Fatalf("usage query count = %d, want 0", len(store.usageFilters))
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(resp.Rows))
	}
	if resp.Rows[0].SampleWarning == "" {
		t.Fatalf("expected sample warning")
	}
}
