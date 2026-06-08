package portal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	sub2APIRequestTimeout = 30 * time.Second
	sub2APIMaxBody        = 4 << 20
)

// Sub2API wraps the runtime gateway operations Portal needs.
type Sub2API interface {
	EnsureUser(ctx context.Context, email string, password string) (Sub2APIUser, error)
	CreateAPIKey(ctx context.Context, email string, password string, name string) (Sub2APIKey, error)
	GetAPIKey(ctx context.Context, email string, password string, keyID int64) (Sub2APIKey, error)
	AddBalance(ctx context.Context, userID int64, amount float64, note string) (Sub2APIUser, error)
}

// Sub2APIUser is the subset of Sub2API user data needed by Portal.
type Sub2APIUser struct {
	ID      int64   `json:"id"`
	Email   string  `json:"email"`
	Balance float64 `json:"balance"`
}

// Sub2APIGroup is the default routing group exposed to Portal users.
type Sub2APIGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Sub2APIKey is the key created by Sub2API.
type Sub2APIKey struct {
	ID      int64  `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	GroupID int64  `json:"group_id"`
	Status  string `json:"status"`
}

// Sub2APIClient calls Sub2API's HTTP API.
type Sub2APIClient struct {
	cfg        Sub2APIConfig
	httpClient *http.Client
}

// NewSub2APIClient creates a Sub2API HTTP client.
func NewSub2APIClient(cfg Sub2APIConfig) *Sub2APIClient {
	return &Sub2APIClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: sub2APIRequestTimeout,
		},
	}
}

// EnsureUser finds or creates a Sub2API user with access to the default group.
func (c *Sub2APIClient) EnsureUser(ctx context.Context, email string, password string) (Sub2APIUser, error) {
	adminToken, errLogin := c.login(ctx, c.cfg.AdminEmail, c.cfg.AdminPassword)
	if errLogin != nil {
		return Sub2APIUser{}, errLogin
	}
	group, errGroup := c.ensureDefaultGroup(ctx, adminToken)
	if errGroup != nil {
		return Sub2APIUser{}, errGroup
	}
	users, errUsers := c.listUsers(ctx, adminToken)
	if errUsers != nil {
		return Sub2APIUser{}, errUsers
	}
	for _, user := range users {
		if normalizeEmail(user.Email) == normalizeEmail(email) {
			return user, nil
		}
	}
	return c.createUser(ctx, adminToken, email, password, group.ID)
}

// CreateAPIKey creates a key in Sub2API as the mapped Sub2API user.
func (c *Sub2APIClient) CreateAPIKey(ctx context.Context, email string, password string, name string) (Sub2APIKey, error) {
	adminToken, errAdmin := c.login(ctx, c.cfg.AdminEmail, c.cfg.AdminPassword)
	if errAdmin != nil {
		return Sub2APIKey{}, errAdmin
	}
	group, errGroup := c.ensureDefaultGroup(ctx, adminToken)
	if errGroup != nil {
		return Sub2APIKey{}, errGroup
	}
	userToken, errUser := c.login(ctx, email, password)
	if errUser != nil {
		return Sub2APIKey{}, errUser
	}
	var key Sub2APIKey
	if errRequest := c.request(ctx, http.MethodPost, "/api/v1/keys", userToken, map[string]any{
		"name":     name,
		"group_id": group.ID,
		"quota":    c.cfg.DefaultKeyQuota,
		"status":   statusActive,
	}, &key); errRequest != nil {
		return Sub2APIKey{}, fmt.Errorf("sub2api: create api key: %w", errRequest)
	}
	return key, nil
}

// GetAPIKey retrieves a user's existing Sub2API key for copy-on-demand behavior.
func (c *Sub2APIClient) GetAPIKey(ctx context.Context, email string, password string, keyID int64) (Sub2APIKey, error) {
	userToken, errUser := c.login(ctx, email, password)
	if errUser != nil {
		return Sub2APIKey{}, errUser
	}
	keys, errKeys := c.listAPIKeys(ctx, userToken)
	if errKeys != nil {
		return Sub2APIKey{}, errKeys
	}
	for _, key := range keys {
		if key.ID == keyID {
			if strings.TrimSpace(key.Key) == "" {
				return Sub2APIKey{}, ErrNotFound
			}
			return key, nil
		}
	}
	return Sub2APIKey{}, ErrNotFound
}

// AddBalance applies a manual top-up to a Sub2API user.
func (c *Sub2APIClient) AddBalance(ctx context.Context, userID int64, amount float64, note string) (Sub2APIUser, error) {
	adminToken, errLogin := c.login(ctx, c.cfg.AdminEmail, c.cfg.AdminPassword)
	if errLogin != nil {
		return Sub2APIUser{}, errLogin
	}
	var user Sub2APIUser
	if errRequest := c.request(ctx, http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/balance", userID), adminToken, map[string]any{
		"balance":   amount,
		"operation": "add",
		"notes":     note,
	}, &user); errRequest != nil {
		return Sub2APIUser{}, fmt.Errorf("sub2api: add balance: %w", errRequest)
	}
	return user, nil
}

func (c *Sub2APIClient) login(ctx context.Context, email string, password string) (string, error) {
	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if errRequest := c.request(ctx, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	}, &resp); errRequest != nil {
		return "", fmt.Errorf("sub2api: login: %w", errRequest)
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return "", fmt.Errorf("sub2api: login response missing access_token")
	}
	return resp.AccessToken, nil
}

func (c *Sub2APIClient) ensureDefaultGroup(ctx context.Context, token string) (Sub2APIGroup, error) {
	groups, errGroups := c.listGroups(ctx, token)
	if errGroups != nil {
		return Sub2APIGroup{}, errGroups
	}
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.Name), c.cfg.DefaultGroup) {
			return group, nil
		}
	}
	var group Sub2APIGroup
	if errRequest := c.request(ctx, http.MethodPost, "/api/v1/admin/groups", token, map[string]any{
		"name":                   c.cfg.DefaultGroup,
		"description":            "CPA OpenAI-compatible upstream group",
		"platform":               "openai",
		"rate_multiplier":        1,
		"is_exclusive":           false,
		"status":                 statusActive,
		"subscription_type":      "standard",
		"allow_image_generation": true,
	}, &group); errRequest != nil {
		return Sub2APIGroup{}, fmt.Errorf("sub2api: create default group: %w", errRequest)
	}
	return group, nil
}

func (c *Sub2APIClient) listGroups(ctx context.Context, token string) ([]Sub2APIGroup, error) {
	var groups []Sub2APIGroup
	if errRequest := c.requestList(ctx, http.MethodGet, "/api/v1/admin/groups", token, nil, &groups); errRequest != nil {
		return nil, fmt.Errorf("sub2api: list groups: %w", errRequest)
	}
	return groups, nil
}

func (c *Sub2APIClient) listUsers(ctx context.Context, token string) ([]Sub2APIUser, error) {
	var users []Sub2APIUser
	if errRequest := c.requestList(ctx, http.MethodGet, "/api/v1/admin/users", token, nil, &users); errRequest != nil {
		return nil, fmt.Errorf("sub2api: list users: %w", errRequest)
	}
	return users, nil
}

func (c *Sub2APIClient) listAPIKeys(ctx context.Context, token string) ([]Sub2APIKey, error) {
	var keys []Sub2APIKey
	if errRequest := c.requestList(ctx, http.MethodGet, "/api/v1/keys", token, nil, &keys); errRequest != nil {
		return nil, fmt.Errorf("sub2api: list api keys: %w", errRequest)
	}
	return keys, nil
}

func (c *Sub2APIClient) createUser(ctx context.Context, token string, email string, password string, groupID int64) (Sub2APIUser, error) {
	var user Sub2APIUser
	name := email
	if at := strings.Index(email, "@"); at > 0 {
		name = email[:at]
	}
	if errRequest := c.request(ctx, http.MethodPost, "/api/v1/admin/users", token, map[string]any{
		"email":          email,
		"password":       password,
		"name":           name,
		"role":           roleUser,
		"status":         statusActive,
		"balance":        0,
		"quota":          0,
		"used_quota":     0,
		"allowed_groups": []int64{groupID},
	}, &user); errRequest != nil {
		return Sub2APIUser{}, fmt.Errorf("sub2api: create user: %w", errRequest)
	}
	return user, nil
}

func (c *Sub2APIClient) requestList(ctx context.Context, method string, path string, token string, body any, out any) error {
	var raw json.RawMessage
	if errRequest := c.requestRaw(ctx, method, path, token, body, &raw); errRequest != nil {
		return errRequest
	}
	items, errItems := dataItems(raw)
	if errItems != nil {
		return errItems
	}
	payload, errMarshal := json.Marshal(items)
	if errMarshal != nil {
		return fmt.Errorf("marshal list data: %w", errMarshal)
	}
	if errDecode := json.Unmarshal(payload, out); errDecode != nil {
		return fmt.Errorf("decode list data: %w", errDecode)
	}
	return nil
}

func (c *Sub2APIClient) request(ctx context.Context, method string, path string, token string, body any, out any) error {
	var raw json.RawMessage
	if errRequest := c.requestRaw(ctx, method, path, token, body, &raw); errRequest != nil {
		return errRequest
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if errDecode := json.Unmarshal(raw, out); errDecode != nil {
		return fmt.Errorf("decode data: %w", errDecode)
	}
	return nil
}

func (c *Sub2APIClient) requestRaw(ctx context.Context, method string, path string, token string, body any, out *json.RawMessage) error {
	if c == nil {
		return fmt.Errorf("sub2api client is not initialized")
	}
	var reader io.Reader
	if body != nil {
		payload, errMarshal := json.Marshal(body)
		if errMarshal != nil {
			return fmt.Errorf("encode request: %w", errMarshal)
		}
		reader = bytes.NewReader(payload)
	}
	target, errURL := joinURL(c.cfg.BaseURL, path)
	if errURL != nil {
		return errURL
	}
	req, errReq := http.NewRequestWithContext(ctx, method, target, reader)
	if errReq != nil {
		return fmt.Errorf("build request: %w", errReq)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, errDo := c.httpClient.Do(req)
	if errDo != nil {
		return fmt.Errorf("request failed: %w", errDo)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	rawBody, errRead := io.ReadAll(io.LimitReader(resp.Body, sub2APIMaxBody))
	if errRead != nil {
		return fmt.Errorf("read response: %w", errRead)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}
	var envelope struct {
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
		Error   string          `json:"error"`
	}
	if len(rawBody) == 0 {
		*out = nil
		return nil
	}
	if errDecode := json.Unmarshal(rawBody, &envelope); errDecode != nil {
		return fmt.Errorf("decode response: %w", errDecode)
	}
	if len(envelope.Data) == 0 {
		if envelope.Error != "" {
			return fmt.Errorf("%s", envelope.Error)
		}
		*out = rawBody
		return nil
	}
	*out = envelope.Data
	return nil
}

func dataItems(raw json.RawMessage) ([]json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []json.RawMessage{}, nil
	}
	if raw[0] == '[' {
		var out []json.RawMessage
		if errDecode := json.Unmarshal(raw, &out); errDecode != nil {
			return nil, fmt.Errorf("decode data array: %w", errDecode)
		}
		return out, nil
	}
	var wrapped struct {
		Items []json.RawMessage `json:"items"`
	}
	if errDecode := json.Unmarshal(raw, &wrapped); errDecode == nil && wrapped.Items != nil {
		return wrapped.Items, nil
	}
	return []json.RawMessage{raw}, nil
}

func joinURL(baseURL string, path string) (string, error) {
	base, errParse := url.Parse(strings.TrimRight(baseURL, "/"))
	if errParse != nil {
		return "", fmt.Errorf("parse sub2api base url: %w", errParse)
	}
	trimmed := strings.TrimLeft(path, "/")
	if trimmed == "" {
		return base.String(), nil
	}
	if base.Path == "" || base.Path == "/" {
		base.Path = "/" + trimmed
	} else {
		base.Path = strings.TrimRight(base.Path, "/") + "/" + trimmed
	}
	return base.String(), nil
}
