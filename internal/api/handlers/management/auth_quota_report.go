package management

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	codexauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/codex"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

const codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"
const codexQuotaUserAgent = "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal"

type authQuotaReportSummary struct {
	Total   int `json:"total"`
	Valid   int `json:"valid"`
	Invalid int `json:"invalid"`
}

type authQuotaReportAccount struct {
	AuthIndex        string         `json:"auth_index"`
	AuthFile         string         `json:"auth_file,omitempty"`
	Provider         string         `json:"provider"`
	Display          string         `json:"display,omitempty"`
	Status           string         `json:"status"`
	Reason           string         `json:"reason,omitempty"`
	TokenRefreshed   bool           `json:"token_refreshed"`
	ExpiresAt        string         `json:"expires_at,omitempty"`
	LastRefresh      string         `json:"last_refresh,omitempty"`
	Subscription     map[string]any `json:"subscription,omitempty"`
	RuntimeQuota     map[string]any `json:"runtime_quota,omitempty"`
	Quota            map[string]any `json:"quota"`
	RecentSuccessful int64          `json:"recent_successful"`
	RecentFailed     int64          `json:"recent_failed"`
}

type authQuotaReportResponse struct {
	Provider    string                   `json:"provider"`
	GeneratedAt time.Time                `json:"generated_at"`
	Summary     authQuotaReportSummary   `json:"summary"`
	Accounts    []authQuotaReportAccount `json:"accounts"`
}

// GetAuthQuotaReport returns a real-time auth quota report.
//
// For Codex OAuth auths, this follows the management panel behavior: use the
// current access token to call ChatGPT's wham/usage endpoint and parse quota
// windows from that response.
func (h *Handler) GetAuthQuotaReport(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}

	provider := strings.ToLower(strings.TrimSpace(c.Query("provider")))
	if provider == "" {
		provider = "codex"
	}

	auths := h.authManager.List()
	resp := authQuotaReportResponse{
		Provider:    provider,
		GeneratedAt: time.Now().UTC(),
		Accounts:    make([]authQuotaReportAccount, 0, len(auths)),
	}

	for _, auth := range auths {
		if auth == nil {
			continue
		}
		if provider != "all" && !strings.EqualFold(strings.TrimSpace(auth.Provider), provider) {
			continue
		}
		resp.Summary.Total++
		entry := h.buildQuotaReportAccount(c.Request.Context(), auth)
		if entry.Status == "valid" {
			resp.Summary.Valid++
		} else {
			resp.Summary.Invalid++
		}
		resp.Accounts = append(resp.Accounts, entry)
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) buildQuotaReportAccount(ctx context.Context, auth *coreauth.Auth) authQuotaReportAccount {
	auth.EnsureIndex()
	entry := authQuotaReportAccount{
		AuthIndex:        auth.Index,
		AuthFile:         authFileNameForQuotaReport(auth),
		Provider:         strings.TrimSpace(auth.Provider),
		Display:          displayNameForQuotaReport(auth),
		Status:           "invalid",
		RecentSuccessful: auth.Success,
		RecentFailed:     auth.Failed,
		Quota: map[string]any{
			"known":   false,
			"message": "quota unavailable",
		},
	}
	if strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		entry.Subscription = codexSubscriptionForQuotaReport(auth, time.Now().UTC())
	}

	if auth.Disabled || auth.Status == coreauth.StatusDisabled {
		entry.Reason = "disabled"
		return entry
	}

	entry.RuntimeQuota = runtimeQuotaForReport(auth)

	switch strings.ToLower(strings.TrimSpace(auth.Provider)) {
	case "codex":
		return h.fetchCodexQuotaForReport(ctx, auth, entry)
	default:
		entry.Reason = "provider quota report is not supported"
		return entry
	}
}

func (h *Handler) fetchCodexQuotaForReport(ctx context.Context, auth *coreauth.Auth, entry authQuotaReportAccount) authQuotaReportAccount {
	accessToken := tokenValueForAuth(auth)
	if accessToken == "" {
		entry.Reason = "missing access token"
		return entry
	}

	if ctx == nil {
		ctx = context.Background()
	}

	quotaCtx, cancel := context.WithTimeout(ctx, defaultAPICallTimeout)
	defer cancel()

	req, errReq := http.NewRequestWithContext(quotaCtx, http.MethodGet, codexUsageURL, nil)
	if errReq != nil {
		entry.Reason = "failed to build quota request"
		return entry
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", codexQuotaUserAgent)
	if accountID := codexAccountIDForQuotaReport(auth); accountID != "" {
		req.Header.Set("Chatgpt-Account-Id", accountID)
	}

	httpClient := &http.Client{
		Timeout:   defaultAPICallTimeout,
		Transport: h.apiCallTransport(auth, ""),
	}
	resp, errDo := httpClient.Do(req)
	if errDo != nil {
		entry.Reason = "quota request failed"
		return entry
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("response body close error: %v", errClose)
		}
	}()

	body, errRead := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if errRead != nil {
		entry.Reason = "failed to read quota response"
		return entry
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		statusText := strings.TrimSpace(http.StatusText(resp.StatusCode))
		if statusText == "" {
			statusText = strconv.Itoa(resp.StatusCode)
		}
		entry.Reason = sanitizeReportReason("quota request failed: HTTP " + statusText)
		return entry
	}

	var payload map[string]any
	if errDecode := json.Unmarshal(body, &payload); errDecode != nil {
		entry.Reason = "quota response is not JSON"
		return entry
	}

	entry.Status = "valid"
	entry.Reason = ""
	entry.TokenRefreshed = false
	entry.Display = displayNameForQuotaReport(auth)
	entry.RuntimeQuota = runtimeQuotaForReport(auth)
	entry.Subscription = codexSubscriptionForQuotaReport(auth, time.Now().UTC())
	entry.Quota = codexUsageQuotaMetadata(payload, auth)
	return entry
}

func codexUsageQuotaMetadata(payload map[string]any, auth *coreauth.Auth) map[string]any {
	out := map[string]any{
		"known":   true,
		"type":    "codex-wham-usage",
		"message": "quota fetched via ChatGPT wham usage",
	}
	if plan := strings.TrimSpace(stringFromMap(payload, "plan_type", "planType")); plan != "" {
		out["plan_type"] = strings.ToLower(plan)
	} else if plan := codexPlanTypeForQuotaReport(auth); plan != "" {
		out["plan_type"] = plan
	}
	out["windows"] = codexQuotaWindows(payload)
	return out
}

func codexPlanTypeForQuotaReport(auth *coreauth.Auth) string {
	if auth == nil || auth.Metadata == nil {
		return ""
	}
	idToken := stringValue(auth.Metadata, "id_token")
	if idToken == "" {
		return ""
	}
	claims, err := codexauth.ParseJWTToken(idToken)
	if err != nil || claims == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(claims.CodexAuthInfo.ChatgptPlanType))
}

func codexAccountIDForQuotaReport(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if accountID := stringValue(auth.Metadata, "account_id"); accountID != "" {
		return accountID
	}
	if auth.Metadata == nil {
		return ""
	}
	idToken := stringValue(auth.Metadata, "id_token")
	if idToken == "" {
		return ""
	}
	claims, err := codexauth.ParseJWTToken(idToken)
	if err != nil || claims == nil {
		return ""
	}
	return strings.TrimSpace(claims.CodexAuthInfo.ChatgptAccountID)
}

func codexSubscriptionForQuotaReport(auth *coreauth.Auth, now time.Time) map[string]any {
	if auth == nil || auth.Metadata == nil {
		return nil
	}
	idToken := stringValue(auth.Metadata, "id_token")
	if idToken == "" {
		return nil
	}
	claims, err := codexauth.ParseJWTToken(idToken)
	if err != nil || claims == nil {
		return nil
	}

	out := map[string]any{}
	if plan := strings.ToLower(strings.TrimSpace(claims.CodexAuthInfo.ChatgptPlanType)); plan != "" {
		out["plan_type"] = plan
	}
	if startAt, ok := timeFromAny(claims.CodexAuthInfo.ChatgptSubscriptionActiveStart); ok {
		out["active_start"] = startAt.UTC().Format(time.RFC3339)
	}
	if untilAt, ok := timeFromAny(claims.CodexAuthInfo.ChatgptSubscriptionActiveUntil); ok {
		untilAt = untilAt.UTC()
		remaining := int64(untilAt.Sub(now.UTC()).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		out["active_until"] = untilAt.Format(time.RFC3339)
		out["remaining_seconds"] = remaining
		out["remaining_label"] = durationLabel(time.Duration(remaining) * time.Second)
		out["expired"] = !untilAt.After(now.UTC())
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func codexQuotaWindows(payload map[string]any) []map[string]any {
	windows := make([]map[string]any, 0, 6)
	windows = append(windows, codexQuotaWindowsFromRateLimit("five-hour", "5h", mapFromAny(firstMapValue(payload, "rate_limit", "rateLimit")), false)...)
	windows = append(windows, codexQuotaWindowsFromRateLimit("weekly", "weekly", mapFromAny(firstMapValue(payload, "rate_limit", "rateLimit")), true)...)
	windows = append(windows, codexQuotaWindowsFromRateLimit("code-review-five-hour", "code review 5h", mapFromAny(firstMapValue(payload, "code_review_rate_limit", "codeReviewRateLimit")), false)...)
	windows = append(windows, codexQuotaWindowsFromRateLimit("code-review-weekly", "code review weekly", mapFromAny(firstMapValue(payload, "code_review_rate_limit", "codeReviewRateLimit")), true)...)

	additional, ok := firstMapValue(payload, "additional_rate_limits", "additionalRateLimits").([]any)
	if !ok {
		return compactQuotaWindows(windows)
	}
	for i, raw := range additional {
		item := mapFromAny(raw)
		rateLimit := mapFromAny(firstMapValue(item, "rate_limit", "rateLimit"))
		if len(rateLimit) == 0 {
			continue
		}
		name := strings.TrimSpace(stringFromMap(item, "limit_name", "limitName", "metered_feature", "meteredFeature"))
		if name == "" {
			name = "additional " + strconv.Itoa(i+1)
		}
		idBase := quotaWindowID(name)
		windows = append(windows, codexQuotaWindowsFromRateLimit(idBase+"-five-hour", name+" 5h", rateLimit, false)...)
		windows = append(windows, codexQuotaWindowsFromRateLimit(idBase+"-weekly", name+" weekly", rateLimit, true)...)
	}
	return compactQuotaWindows(windows)
}

func codexQuotaWindowsFromRateLimit(id string, label string, rateLimit map[string]any, weekly bool) []map[string]any {
	if len(rateLimit) == 0 {
		return nil
	}
	primary := mapFromAny(firstMapValue(rateLimit, "primary_window", "primaryWindow"))
	secondary := mapFromAny(firstMapValue(rateLimit, "secondary_window", "secondaryWindow"))
	selected := primary
	if weekly {
		selected = secondary
	}
	if len(selected) == 0 {
		return nil
	}
	window := quotaWindowMap(id, label, selected)
	if window == nil {
		return nil
	}
	return []map[string]any{window}
}

func quotaWindowMap(id string, label string, raw map[string]any) map[string]any {
	used, hasUsed := floatFromAny(firstMapValue(raw, "used_percent", "usedPercent"))
	out := map[string]any{
		"id":    id,
		"label": label,
	}
	if hasUsed {
		remaining := 100 - used
		if remaining < 0 {
			remaining = 0
		}
		if remaining > 100 {
			remaining = 100
		}
		out["used_percent"] = used
		out["remaining_percent"] = remaining
	}
	if resetAt, ok := floatFromAny(firstMapValue(raw, "reset_at", "resetAt")); ok && resetAt > 0 {
		reset := time.Unix(int64(resetAt), 0).UTC()
		out["reset_at"] = reset.Format(time.RFC3339)
		out["reset_label"] = reset.Format("01/02 15:04")
	} else if resetAfter, ok := floatFromAny(firstMapValue(raw, "reset_after_seconds", "resetAfterSeconds")); ok && resetAfter > 0 {
		reset := time.Now().UTC().Add(time.Duration(resetAfter) * time.Second)
		out["reset_at"] = reset.Format(time.RFC3339)
		out["reset_after_seconds"] = resetAfter
		out["reset_label"] = reset.Format("01/02 15:04")
	}
	if seconds, ok := floatFromAny(firstMapValue(raw, "limit_window_seconds", "limitWindowSeconds")); ok {
		out["limit_window_seconds"] = seconds
	}
	if len(out) == 2 {
		return nil
	}
	return out
}

func compactQuotaWindows(windows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(windows))
	for _, window := range windows {
		if len(window) > 0 {
			out = append(out, window)
		}
	}
	return out
}

func firstMapValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func mapFromAny(raw any) map[string]any {
	if typed, ok := raw.(map[string]any); ok {
		return typed
	}
	return nil
}

func stringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		switch typed := m[key].(type) {
		case string:
			if value := strings.TrimSpace(typed); value != "" {
				return value
			}
		case float64:
			return strconv.Itoa(int(typed))
		}
	}
	return ""
}

func floatFromAny(raw any) (float64, bool) {
	switch typed := raw.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		if typed = strings.TrimSpace(typed); typed != "" {
			if out, err := strconv.ParseFloat(typed, 64); err == nil {
				return out, true
			}
		}
	}
	return 0, false
}

func timeFromAny(raw any) (time.Time, bool) {
	switch typed := raw.(type) {
	case time.Time:
		if typed.IsZero() {
			return time.Time{}, false
		}
		return typed, true
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
			return time.Time{}, false
		}
		if ts, err := time.Parse(time.RFC3339, value); err == nil {
			return ts, true
		}
		if ts, err := time.Parse("2006-01-02", value); err == nil {
			return ts, true
		}
		if seconds, err := strconv.ParseFloat(value, 64); err == nil {
			return unixTimeFromFloat(seconds)
		}
	case float64:
		return unixTimeFromFloat(typed)
	case float32:
		return unixTimeFromFloat(float64(typed))
	case int:
		return unixTimeFromFloat(float64(typed))
	case int64:
		return unixTimeFromFloat(float64(typed))
	case json.Number:
		if seconds, err := typed.Float64(); err == nil {
			return unixTimeFromFloat(seconds)
		}
	}
	return time.Time{}, false
}

func unixTimeFromFloat(value float64) (time.Time, bool) {
	if value <= 0 {
		return time.Time{}, false
	}
	if value > 100000000000 {
		value = value / 1000
	}
	seconds := int64(value)
	nanos := int64((value - float64(seconds)) * 1e9)
	return time.Unix(seconds, nanos), true
}

func durationLabel(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}
	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)
	if days > 0 {
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h"
	}
	if hours > 0 {
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}
	if minutes > 0 {
		return strconv.Itoa(minutes) + "m"
	}
	return "<1m"
}

func quotaWindowID(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func runtimeQuotaForReport(auth *coreauth.Auth) map[string]any {
	if auth == nil {
		return nil
	}
	out := make(map[string]any)
	if auth.Quota.Exceeded || auth.Quota.Reason != "" || !auth.Quota.NextRecoverAt.IsZero() {
		out["exceeded"] = auth.Quota.Exceeded
		if auth.Quota.Reason != "" {
			out["reason"] = auth.Quota.Reason
		}
		if !auth.Quota.NextRecoverAt.IsZero() {
			out["next_recover_at"] = auth.Quota.NextRecoverAt
		}
	}
	if !auth.NextRetryAfter.IsZero() {
		out["next_retry_after"] = auth.NextRetryAfter
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func displayNameForQuotaReport(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if email := authEmail(auth); email != "" {
		return sanitizeReportReason(email)
	}
	if auth.Label != "" {
		return sanitizeReportReason(auth.Label)
	}
	if auth.FileName != "" {
		return sanitizeReportReason(auth.FileName)
	}
	return sanitizeReportReason(auth.ID)
}

func authFileNameForQuotaReport(auth *coreauth.Auth) string {
	if auth == nil {
		return ""
	}
	if name := strings.TrimSpace(auth.FileName); name != "" {
		return sanitizeReportReason(name)
	}
	if path := strings.TrimSpace(authAttribute(auth, "path")); path != "" {
		return sanitizeReportReason(filepath.Base(path))
	}
	return sanitizeReportReason(auth.ID)
}

func sanitizeReportReason(raw string) string {
	raw = strings.ReplaceAll(raw, "\r", " ")
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.TrimSpace(raw)
	if len(raw) > 240 {
		return raw[:240] + "..."
	}
	return raw
}
