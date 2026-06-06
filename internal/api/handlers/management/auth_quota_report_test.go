package management

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestGetAuthQuotaReportFiltersProviderAndCountsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	if _, errRegister := manager.Register(context.Background(), &coreauth.Auth{
		ID:       "codex-auth",
		Provider: "codex",
		FileName: "codex-auth.json",
		Metadata: map[string]any{
			"email": "person@example.com",
		},
	}); errRegister != nil {
		t.Fatalf("register codex auth: %v", errRegister)
	}
	if _, errRegister := manager.Register(context.Background(), &coreauth.Auth{
		ID:       "claude-auth",
		Provider: "claude",
		Metadata: map[string]any{
			"email": "person@example.com",
		},
	}); errRegister != nil {
		t.Fatalf("register claude auth: %v", errRegister)
	}

	handler := &Handler{authManager: manager}
	router := gin.New()
	router.GET("/report", handler.GetAuthQuotaReport)

	req := httptest.NewRequest(http.MethodGet, "/report?provider=codex", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp authQuotaReportResponse
	if errDecode := json.Unmarshal(rr.Body.Bytes(), &resp); errDecode != nil {
		t.Fatalf("decode response: %v", errDecode)
	}
	if resp.Summary.Total != 1 || resp.Summary.Valid != 0 || resp.Summary.Invalid != 1 {
		t.Fatalf("summary = %+v, want total=1 valid=0 invalid=1", resp.Summary)
	}
	if len(resp.Accounts) != 1 {
		t.Fatalf("accounts len = %d, want 1", len(resp.Accounts))
	}
	if got := resp.Accounts[0].Reason; got != "missing access token" {
		t.Fatalf("reason = %q, want missing access token", got)
	}
	if got := resp.Accounts[0].Display; got != "person@example.com" {
		t.Fatalf("display = %q, want full email", got)
	}
	if got := resp.Accounts[0].AuthFile; got != "codex-auth.json" {
		t.Fatalf("auth_file = %q, want codex-auth.json", got)
	}
}

func TestCodexUsageQuotaMetadataParsesWindows(t *testing.T) {
	payload := map[string]any{
		"plan_type": "plus",
		"rate_limit": map[string]any{
			"primary_window": map[string]any{
				"used_percent":         100.0,
				"reset_at":             1780427640.0,
				"limit_window_seconds": 18000.0,
			},
			"secondary_window": map[string]any{
				"used_percent":         16.0,
				"reset_at":             1781014440.0,
				"limit_window_seconds": 604800.0,
			},
		},
	}

	quota := codexUsageQuotaMetadata(payload, nil)
	if got := quota["plan_type"]; got != "plus" {
		t.Fatalf("plan_type = %#v, want plus", got)
	}
	windows, ok := quota["windows"].([]map[string]any)
	if !ok {
		t.Fatalf("windows type = %T, want []map[string]any", quota["windows"])
	}
	if len(windows) != 2 {
		t.Fatalf("windows len = %d, want 2", len(windows))
	}
	if got := windows[0]["remaining_percent"]; got != 0.0 {
		t.Fatalf("first remaining_percent = %#v, want 0", got)
	}
	if got := windows[1]["remaining_percent"]; got != 84.0 {
		t.Fatalf("second remaining_percent = %#v, want 84", got)
	}
}

func TestCodexSubscriptionForQuotaReportParsesRemainingTime(t *testing.T) {
	auth := &coreauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{
			"id_token": fakeJWT(map[string]any{
				"https://api.openai.com/auth": map[string]any{
					"chatgpt_plan_type":                 "plus",
					"chatgpt_subscription_active_start": "2026-06-01T00:00:00Z",
					"chatgpt_subscription_active_until": "2026-06-04T12:30:00Z",
				},
			}),
		},
	}

	subscription := codexSubscriptionForQuotaReport(auth, time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC))
	if got := subscription["plan_type"]; got != "plus" {
		t.Fatalf("plan_type = %#v, want plus", got)
	}
	if got := subscription["active_until"]; got != "2026-06-04T12:30:00Z" {
		t.Fatalf("active_until = %#v, want 2026-06-04T12:30:00Z", got)
	}
	if got := subscription["remaining_seconds"]; got != int64(174600) {
		t.Fatalf("remaining_seconds = %#v, want 174600", got)
	}
	if got := subscription["remaining_label"]; got != "2d 0h" {
		t.Fatalf("remaining_label = %#v, want 2d 0h", got)
	}
	if got := subscription["expired"]; got != false {
		t.Fatalf("expired = %#v, want false", got)
	}
}

func fakeJWT(claims map[string]any) string {
	header := map[string]any{"alg": "none", "typ": "JWT"}
	return base64RawJSON(header) + "." + base64RawJSON(claims) + ".signature"
}

func base64RawJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}
