package cpadashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type dashboardStore interface {
	QuotaObservationWindows(ctx context.Context, filter QueryFilter) ([]QuotaObservationWindow, error)
	QuotaPoints(ctx context.Context, filter QueryFilter) ([]QuotaPoint, error)
	UsageSummaries(ctx context.Context, filter QueryFilter) ([]UsageSummary, error)
	LatestCollectionAt(ctx context.Context) (*time.Time, error)
	CurrentAccounts(ctx context.Context, cpa string) ([]CPAAccountRow, error)
	UsageBuckets(ctx context.Context, filter QueryFilter) ([]UsageBucket, error)
	CleanupCandidates(ctx context.Context, cpa string) ([]CleanupCandidate, error)
	Filters(ctx context.Context) (FiltersResponse, error)
}

// Service coordinates report queries and calculations.
type Service struct {
	store dashboardStore
	now   func() time.Time
}

// NewService creates a dashboard report service.
func NewService(store dashboardStore) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) QuotaEfficiency(ctx context.Context, filter QueryFilter) (EfficiencyResponse, error) {
	observations, errObservations := s.store.QuotaObservationWindows(ctx, filter)
	if errObservations != nil {
		return EfficiencyResponse{}, errObservations
	}
	windows := buildEffectiveQuotaWindows(filter, observations)

	sort.Slice(windows, func(i, j int) bool {
		return windows[i].CPASource < windows[j].CPASource
	})

	rows := make([]EfficiencyRow, 0, len(windows))
	for i := range windows {
		window := windows[i]
		if !window.EffectiveStart.Before(window.EffectiveEnd) {
			rows = append(rows, EfficiencyRow{
				CPASource:        window.CPASource,
				EffectiveStart:   &window.EffectiveStart,
				EffectiveEnd:     &window.EffectiveEnd,
				QuotaSampleCount: window.QuotaSampleCount,
				SampleWarning:    window.Warning,
			})
			continue
		}

		alignedFilter := filter
		alignedFilter.CPA = window.CPASource
		alignedFilter.Start = window.EffectiveStart
		alignedFilter.End = window.EffectiveEnd

		points, errPoints := s.store.QuotaPoints(ctx, alignedFilter)
		if errPoints != nil {
			return EfficiencyResponse{}, errPoints
		}
		quotaByCPA := computeQuotaConsumption(points)
		quota := quotaByCPA[window.CPASource]

		usageItems, errUsage := s.store.UsageSummaries(ctx, alignedFilter)
		if errUsage != nil {
			return EfficiencyResponse{}, errUsage
		}
		usage := usageForCPA(window.CPASource, usageItems)

		row := buildEfficiencyRow(window.CPASource, usage, quota)
		row.EffectiveStart = &window.EffectiveStart
		row.EffectiveEnd = &window.EffectiveEnd
		row.QuotaSampleCount = quota.SampleCount
		windows[i].QuotaSampleCount = quota.SampleCount
		if row.QuotaSampleCount < 2 {
			row.SampleWarning = "Quota sample window has fewer than two samples; normalized estimates may be noisy or unavailable."
			windows[i].Warning = row.SampleWarning
		}
		rows = append(rows, row)
	}
	effectiveStart, effectiveEnd := aggregateEffectiveRange(windows)
	resp := EfficiencyResponse{
		Filter:          filter,
		Rows:            rows,
		Total:           sumEfficiencyRows(rows),
		Windows:         windows,
		RequestedStart:  filter.Start,
		RequestedEnd:    filter.End,
		EffectiveStart:  effectiveStart,
		EffectiveEnd:    effectiveEnd,
		AlignmentNotice: buildAlignmentNotice(filter, windows),
		GeneratedAt:     s.now().UTC(),
	}
	if strings.TrimSpace(filter.Model) != "" || filter.APIKeyID != 0 {
		resp.AttributionWarning = "Token filters apply to Sub2API usage. Weekly quota consumption is measured at CPA instance level, so filtered efficiency is directional unless all traffic in the window matches the filter."
	}
	return resp, nil
}

func (s *Service) Overview(ctx context.Context) (OverviewResponse, error) {
	now := s.now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayFilter := QueryFilter{Start: todayStart, End: now}
	sevenDayFilter := QueryFilter{Start: now.AddDate(0, 0, -defaultRangeDays), End: now}

	latest, errLatest := s.store.LatestCollectionAt(ctx)
	if errLatest != nil {
		return OverviewResponse{}, errLatest
	}
	accounts, errAccounts := s.store.CurrentAccounts(ctx, "")
	if errAccounts != nil {
		return OverviewResponse{}, errAccounts
	}
	todayEfficiency, errToday := s.QuotaEfficiency(ctx, todayFilter)
	if errToday != nil {
		return OverviewResponse{}, errToday
	}
	sevenDayEfficiency, errSeven := s.QuotaEfficiency(ctx, sevenDayFilter)
	if errSeven != nil {
		return OverviewResponse{}, errSeven
	}

	var successCount int64
	var errorCount int64
	for _, account := range accounts {
		if account.Status == "success" && !account.DataStale {
			successCount++
		} else {
			errorCount++
		}
	}
	return OverviewResponse{
		GeneratedAt:             now.UTC(),
		LatestCollectionAt:      latest,
		CurrentAccounts:         accounts,
		CurrentSuccessAccounts:  successCount,
		CurrentErrorAccounts:    errorCount,
		TodayUsage:              todayEfficiency.Total,
		SevenDayEfficiencyTotal: sevenDayEfficiency.Total,
		TodayAlignmentNotice:    todayEfficiency.AlignmentNotice,
		SevenDayAlignmentNotice: sevenDayEfficiency.AlignmentNotice,
	}, nil
}

func buildEffectiveQuotaWindows(filter QueryFilter, observations []QuotaObservationWindow) []EffectiveQuotaWindow {
	windows := make([]EffectiveQuotaWindow, 0, len(observations))
	for _, observation := range observations {
		start := filter.Start
		end := filter.End
		aligned := false
		if observation.FirstCollectedAt.After(start) {
			start = observation.FirstCollectedAt
			aligned = true
		}
		if observation.LatestCollectedAt.Before(end) {
			end = observation.LatestCollectedAt
			aligned = true
		}
		window := EffectiveQuotaWindow{
			CPASource:      observation.CPASource,
			RequestedStart: filter.Start,
			RequestedEnd:   filter.End,
			EffectiveStart: start,
			EffectiveEnd:   end,
			Aligned:        aligned,
		}
		if !start.Before(end) {
			window.Warning = "Quota collection has no complete observation window inside the requested range."
		}
		windows = append(windows, window)
	}
	return windows
}

func usageForCPA(cpa string, items []UsageSummary) UsageSummary {
	for _, item := range items {
		if item.CPASource == cpa {
			return item
		}
	}
	return UsageSummary{CPASource: cpa}
}

func aggregateEffectiveRange(windows []EffectiveQuotaWindow) (*time.Time, *time.Time) {
	var start *time.Time
	var end *time.Time
	for _, window := range windows {
		if !window.EffectiveStart.Before(window.EffectiveEnd) {
			continue
		}
		if start == nil || window.EffectiveStart.Before(*start) {
			value := window.EffectiveStart
			start = &value
		}
		if end == nil || window.EffectiveEnd.After(*end) {
			value := window.EffectiveEnd
			end = &value
		}
	}
	return start, end
}

func buildAlignmentNotice(filter QueryFilter, windows []EffectiveQuotaWindow) string {
	if len(windows) == 0 {
		return "No successful quota collection samples were available for the requested range, so quota efficiency cannot be aligned."
	}
	aligned := false
	for _, window := range windows {
		if window.Aligned {
			aligned = true
			break
		}
	}
	if !aligned {
		return ""
	}
	start, end := aggregateEffectiveRange(windows)
	if start == nil || end == nil {
		return "Quota collection samples were found, but no complete aligned observation window is available."
	}
	return fmt.Sprintf(
		"Usage and cost were aligned to the quota collection window. Requested %s to %s; effective window %s to %s.",
		filter.Start.Format(time.RFC3339),
		filter.End.Format(time.RFC3339),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
}
