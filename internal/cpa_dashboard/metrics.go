package cpadashboard

import (
	"sort"
	"strings"
)

const monthlyEstimateMultiplier = 4.3

func computeQuotaConsumption(points []QuotaPoint) map[string]QuotaConsumption {
	type groupKey struct {
		cpa      string
		identity string
	}
	grouped := make(map[groupKey][]QuotaPoint)
	for _, point := range points {
		cpa := strings.TrimSpace(point.CPASource)
		identity := strings.TrimSpace(point.AccountEmail)
		if identity == "" {
			identity = strings.TrimSpace(point.AuthFile)
		}
		if cpa == "" || identity == "" {
			continue
		}
		grouped[groupKey{cpa: cpa, identity: identity}] = append(grouped[groupKey{cpa: cpa, identity: identity}], point)
	}

	out := make(map[string]QuotaConsumption)
	accounts := make(map[string]map[string]struct{})
	for key, values := range grouped {
		sort.Slice(values, func(i, j int) bool {
			return values[i].CollectedAt.Before(values[j].CollectedAt)
		})
		if accounts[key.cpa] == nil {
			accounts[key.cpa] = make(map[string]struct{})
		}
		accounts[key.cpa][key.identity] = struct{}{}

		consumption := out[key.cpa]
		consumption.CPASource = key.cpa
		samples := collapseQuotaSamples(values)
		consumption.SampleCount += int64(len(samples))
		for i := 1; i < len(samples); i++ {
			delta := samples[i-1].WeeklyRemain - samples[i].WeeklyRemain
			if delta > 0 {
				consumption.ConsumptionPercent += delta
			}
		}
		out[key.cpa] = consumption
	}
	for cpa, auths := range accounts {
		consumption := out[cpa]
		consumption.AccountCount = int64(len(auths))
		out[cpa] = consumption
	}
	return out
}

func collapseQuotaSamples(values []QuotaPoint) []QuotaPoint {
	if len(values) <= 1 {
		return values
	}
	out := make([]QuotaPoint, 0, len(values))
	for _, value := range values {
		if len(out) == 0 || !out[len(out)-1].CollectedAt.Equal(value.CollectedAt) {
			out = append(out, value)
			continue
		}
		if value.WeeklyRemain > out[len(out)-1].WeeklyRemain {
			out[len(out)-1] = value
		}
	}
	return out
}

func buildEfficiencyRow(cpa string, usage UsageSummary, quota QuotaConsumption) EfficiencyRow {
	totalTokens := usage.InputTokens + usage.OutputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
	row := EfficiencyRow{
		CPASource:             cpa,
		WeeklyConsumption:     quota.ConsumptionPercent,
		RequestCount:          usage.RequestCount,
		InputTokens:           usage.InputTokens,
		OutputTokens:          usage.OutputTokens,
		CacheCreationTokens:   usage.CacheCreationTokens,
		CacheReadTokens:       usage.CacheReadTokens,
		CacheCreation5MTokens: usage.CacheCreation5MTokens,
		CacheCreation1HTokens: usage.CacheCreation1HTokens,
		TotalTokens:           totalTokens,
		TotalCost:             usage.TotalCost,
		ActualCost:            usage.ActualCost,
		AverageDurationMS:     usage.AverageDurationMS,
	}
	if quota.ConsumptionPercent <= 0 {
		row.SampleWarning = "No weekly quota decrease was observed in this range."
		return row
	}
	scale := 100 / quota.ConsumptionPercent
	row.Per100 = EfficiencyProjection{
		InputTokens:         float64(usage.InputTokens) * scale,
		OutputTokens:        float64(usage.OutputTokens) * scale,
		CacheCreationTokens: float64(usage.CacheCreationTokens) * scale,
		CacheReadTokens:     float64(usage.CacheReadTokens) * scale,
		TotalTokens:         float64(totalTokens) * scale,
		TotalCost:           usage.TotalCost * scale,
		ActualCost:          usage.ActualCost * scale,
	}
	row.MonthlyEstimate = scaleProjection(row.Per100, monthlyEstimateMultiplier)
	if quota.ConsumptionPercent < sampleWarningPercent {
		row.SampleWarning = "Quota consumption is below 10%; normalized estimates may be noisy."
	}
	return row
}

func scaleProjection(in EfficiencyProjection, multiplier float64) EfficiencyProjection {
	return EfficiencyProjection{
		InputTokens:         in.InputTokens * multiplier,
		OutputTokens:        in.OutputTokens * multiplier,
		CacheCreationTokens: in.CacheCreationTokens * multiplier,
		CacheReadTokens:     in.CacheReadTokens * multiplier,
		TotalTokens:         in.TotalTokens * multiplier,
		TotalCost:           in.TotalCost * multiplier,
		ActualCost:          in.ActualCost * multiplier,
	}
}

func sumEfficiencyRows(rows []EfficiencyRow) EfficiencyRow {
	total := EfficiencyRow{CPASource: "all"}
	var durationWeighted float64
	for _, row := range rows {
		total.WeeklyConsumption += row.WeeklyConsumption
		total.RequestCount += row.RequestCount
		total.InputTokens += row.InputTokens
		total.OutputTokens += row.OutputTokens
		total.CacheCreationTokens += row.CacheCreationTokens
		total.CacheReadTokens += row.CacheReadTokens
		total.CacheCreation5MTokens += row.CacheCreation5MTokens
		total.CacheCreation1HTokens += row.CacheCreation1HTokens
		total.TotalTokens += row.TotalTokens
		total.TotalCost += row.TotalCost
		total.ActualCost += row.ActualCost
		durationWeighted += row.AverageDurationMS * float64(row.RequestCount)
	}
	if total.RequestCount > 0 {
		total.AverageDurationMS = durationWeighted / float64(total.RequestCount)
	}
	total = buildEfficiencyRow("all", UsageSummary{
		CPASource:             "all",
		RequestCount:          total.RequestCount,
		InputTokens:           total.InputTokens,
		OutputTokens:          total.OutputTokens,
		CacheCreationTokens:   total.CacheCreationTokens,
		CacheReadTokens:       total.CacheReadTokens,
		CacheCreation5MTokens: total.CacheCreation5MTokens,
		CacheCreation1HTokens: total.CacheCreation1HTokens,
		TotalCost:             total.TotalCost,
		ActualCost:            total.ActualCost,
		AverageDurationMS:     total.AverageDurationMS,
	}, QuotaConsumption{CPASource: "all", ConsumptionPercent: total.WeeklyConsumption})
	return total
}
