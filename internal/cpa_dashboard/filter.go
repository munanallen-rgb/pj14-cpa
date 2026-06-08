package cpadashboard

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ParseQueryFilter parses shared dashboard report filters from an HTTP request.
func ParseQueryFilter(r *http.Request) (QueryFilter, error) {
	now := time.Now()
	query := r.URL.Query()
	start, errStart := parseTimeParam(query.Get("start"))
	if errStart != nil {
		return QueryFilter{}, fmt.Errorf("invalid start")
	}
	end, errEnd := parseTimeParam(query.Get("end"))
	if errEnd != nil {
		return QueryFilter{}, fmt.Errorf("invalid end")
	}
	if start.IsZero() {
		start = now.AddDate(0, 0, -defaultRangeDays)
	}
	if end.IsZero() {
		end = now
	}
	if !start.Before(end) {
		return QueryFilter{}, fmt.Errorf("start must be before end")
	}
	if end.Sub(start) > time.Duration(maxRangeDays)*24*time.Hour {
		return QueryFilter{}, fmt.Errorf("time range must be %d days or less", maxRangeDays)
	}
	var apiKeyID int64
	if rawAPIKey := strings.TrimSpace(query.Get("api_key_id")); rawAPIKey != "" {
		parsed, errParse := strconv.ParseInt(rawAPIKey, 10, 64)
		if errParse != nil || parsed < 0 {
			return QueryFilter{}, fmt.Errorf("invalid api_key_id")
		}
		apiKeyID = parsed
	}
	return QueryFilter{
		Start:    start,
		End:      end,
		CPA:      strings.TrimSpace(query.Get("cpa_source")),
		Model:    strings.TrimSpace(query.Get("model")),
		APIKeyID: apiKeyID,
	}, nil
}

func parseTimeParam(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if parsed, errParse := time.Parse(time.RFC3339, raw); errParse == nil {
		return parsed, nil
	}
	if parsed, errParse := time.Parse("2006-01-02", raw); errParse == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid time")
}
