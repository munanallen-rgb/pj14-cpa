package quotacollector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	quotaReportPath = "/v0/management/auth-quota-report?provider=codex"
	requestTimeout  = 30 * time.Second
	maxResponseBody = 8 << 20
)

// Client fetches quota reports from CPA management APIs.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a quota report client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

func (c *Client) FetchReport(ctx context.Context, instance InstanceConfig) (quotaReportResponse, error) {
	if c == nil || c.httpClient == nil {
		c = NewClient()
	}
	url := strings.TrimRight(instance.BaseURL, "/") + quotaReportPath
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		report, errFetch := c.fetchOnce(ctx, url, instance.ManagementKey)
		if errFetch == nil {
			return report, nil
		}
		lastErr = errFetch
		if !isRetryableFetchError(errFetch) {
			break
		}
	}
	return quotaReportResponse{}, lastErr
}

func (c *Client) fetchOnce(ctx context.Context, url string, key string) (quotaReportResponse, error) {
	req, errReq := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if errReq != nil {
		return quotaReportResponse{}, fmt.Errorf("%s: %w", ErrorParse, errReq)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, errDo := c.httpClient.Do(req)
	if errDo != nil {
		return quotaReportResponse{}, fmt.Errorf("%s: %w", ErrorNetwork, errDo)
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.WithError(errClose).Debug("quota collector response body close failed")
		}
	}()

	body, errRead := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if errRead != nil {
		return quotaReportResponse{}, fmt.Errorf("%s: %w", ErrorNetwork, errRead)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return quotaReportResponse{}, fmt.Errorf("%s: HTTP %d: %s", ErrorHTTP, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var report quotaReportResponse
	if errDecode := json.Unmarshal(body, &report); errDecode != nil {
		return quotaReportResponse{}, fmt.Errorf("%s: %w", ErrorParse, errDecode)
	}
	return report, nil
}

func isRetryableFetchError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, ErrorNetwork+":")
}

func classifyFetchError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, category := range []string{ErrorNetwork, ErrorHTTP, ErrorParse} {
		if strings.HasPrefix(msg, category+":") {
			return category
		}
	}
	return ErrorInstanceUnavailable
}
