package portal

import "time"

const (
	roleUser  = "user"
	roleAdmin = "admin"

	statusActive     = "active"
	statusDisabled   = "disabled"
	statusPending    = "pending"
	statusConfirmed  = "confirmed"
	statusCancelled  = "cancelled"
	statusProcessing = "processing"

	ledgerTypeRecharge = "recharge"
)

// User is a Portal account.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session stores an authenticated Portal browser session.
type Session struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Sub2APIUserMapping connects a Portal user to a Sub2API user.
type Sub2APIUserMapping struct {
	PortalUserID  int64     `json:"portal_user_id"`
	Sub2APIUserID int64     `json:"sub2api_user_id"`
	Sub2APIEmail  string    `json:"sub2api_email"`
	CreatedAt     time.Time `json:"created_at"`
}

// APIKey stores the Portal-side view of a Sub2API key.
type APIKey struct {
	ID           int64     `json:"id"`
	PortalUserID int64     `json:"-"`
	Sub2APIKeyID int64     `json:"sub2api_key_id"`
	Name         string    `json:"name"`
	KeyPreview   string    `json:"key_preview"`
	GroupName    string    `json:"group_name"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// APIKeyCreateResult returns the full key once at creation time.
type APIKeyCreateResult struct {
	APIKey
	Key string `json:"key"`
}

// UsageSummary stores aggregate Sub2API usage for one Portal user.
type UsageSummary struct {
	RequestCount        int64   `json:"request_count"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
	ActualCost          float64 `json:"actual_cost"`
	AverageDurationMS   float64 `json:"average_duration_ms"`
}

// UsageRecord stores one normalized Sub2API usage row.
type UsageRecord struct {
	ID                  int64     `json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	APIKeyID            int64     `json:"api_key_id"`
	APIKeyName          string    `json:"api_key_name"`
	Model               string    `json:"model"`
	RequestedModel      string    `json:"requested_model"`
	UpstreamModel       string    `json:"upstream_model"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	TotalTokens         int64     `json:"total_tokens"`
	TotalCost           float64   `json:"total_cost"`
	ActualCost          float64   `json:"actual_cost"`
	DurationMS          float64   `json:"duration_ms"`
}

// RechargeOrder is a manual top-up request.
type RechargeOrder struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	UserEmail   string     `json:"user_email,omitempty"`
	Amount      float64    `json:"amount"`
	Currency    string     `json:"currency"`
	Status      string     `json:"status"`
	Note        string     `json:"note"`
	CreatedAt   time.Time  `json:"created_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	ConfirmedBy *int64     `json:"confirmed_by,omitempty"`
}

// LedgerEntry is an immutable Portal accounting event.
type LedgerEntry struct {
	ID                  int64     `json:"id"`
	UserID              int64     `json:"user_id"`
	Type                string    `json:"type"`
	Amount              float64   `json:"amount"`
	Currency            string    `json:"currency"`
	RelatedOrderID      *int64    `json:"related_order_id,omitempty"`
	Sub2APIBalanceAfter *float64  `json:"sub2api_balance_after,omitempty"`
	Note                string    `json:"note"`
	CreatedAt           time.Time `json:"created_at"`
}

// AuthContext is the authenticated request identity.
type AuthContext struct {
	User User
}

// UsageFilter scopes usage reports.
type UsageFilter struct {
	Start    time.Time
	End      time.Time
	APIKeyID int64
	Limit    int
}
