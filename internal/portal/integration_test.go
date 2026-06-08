package portal

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeSub2API struct {
	nextUserID int64
	nextKeyID  int64
	users      map[string]Sub2APIUser
	keys       map[string]Sub2APIKey
	balances   map[int64]float64
}

func newFakeSub2API() *fakeSub2API {
	return &fakeSub2API{
		nextUserID: 10,
		nextKeyID:  100,
		users:      make(map[string]Sub2APIUser),
		keys:       make(map[string]Sub2APIKey),
		balances:   make(map[int64]float64),
	}
}

func (f *fakeSub2API) EnsureUser(_ context.Context, email string, _ string) (Sub2APIUser, error) {
	if user, ok := f.users[email]; ok {
		return user, nil
	}
	f.nextUserID++
	user := Sub2APIUser{ID: f.nextUserID, Email: email}
	f.users[email] = user
	return user, nil
}

func (f *fakeSub2API) CreateAPIKey(_ context.Context, email string, _ string, name string) (Sub2APIKey, error) {
	f.nextKeyID++
	key := Sub2APIKey{
		ID:      f.nextKeyID,
		Key:     "sk-test-" + email,
		Name:    name,
		GroupID: 7,
		Status:  statusActive,
	}
	f.keys[email] = key
	return key, nil
}

func (f *fakeSub2API) AddBalance(_ context.Context, userID int64, amount float64, _ string) (Sub2APIUser, error) {
	f.balances[userID] += amount
	return Sub2APIUser{ID: userID, Email: "user@example.com", Balance: f.balances[userID]}, nil
}

func TestPortalMVPFlowWithPostgres(t *testing.T) {
	dsn := os.Getenv("PORTAL_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("PORTAL_TEST_DATABASE_DSN is not set")
	}
	ctx := t.Context()
	pool, errPool := pgxpool.New(ctx, dsn)
	if errPool != nil {
		t.Fatalf("open pool: %v", errPool)
	}
	defer pool.Close()

	store := &Store{pool: pool}
	if errReset := resetPortalIntegrationDatabase(ctx, pool); errReset != nil {
		t.Fatalf("reset database: %v", errReset)
	}
	if errMigrate := store.Migrate(ctx); errMigrate != nil {
		t.Fatalf("migrate: %v", errMigrate)
	}

	cfg := Config{
		SessionSecret: "integration-secret",
		SessionTTL:    time.Hour,
		BootstrapAdmin: BootstrapAdminConfig{
			Email:    "admin@example.com",
			Password: "admin-password",
		},
		Sub2API: Sub2APIConfig{
			DefaultGroup: "cpa-openai",
		},
	}
	service := NewService(cfg, store, newFakeSub2API())
	if errBootstrap := service.BootstrapAdmin(ctx); errBootstrap != nil {
		t.Fatalf("bootstrap admin: %v", errBootstrap)
	}

	user, token, _, errRegister := service.Register(ctx, "user@example.com", "user-password")
	if errRegister != nil {
		t.Fatalf("register: %v", errRegister)
	}
	if token == "" {
		t.Fatal("register returned empty session token")
	}
	mapping, errMapping := store.GetSub2APIUserMapping(ctx, user.ID)
	if errMapping != nil {
		t.Fatalf("sub2api mapping: %v", errMapping)
	}
	if mapping.Sub2APIUserID == 0 {
		t.Fatal("sub2api mapping missing user id")
	}

	key, errKey := service.CreateAPIKey(ctx, user, "Portal key")
	if errKey != nil {
		t.Fatalf("create api key: %v", errKey)
	}
	if key.Key == "" || key.Sub2APIKeyID == 0 {
		t.Fatalf("created key missing values: %#v", key)
	}

	_, errUsage := pool.Exec(ctx, `
INSERT INTO public.usage_logs (
  api_key_id, created_at, model, requested_model, upstream_model,
  input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
  total_cost, actual_cost, duration_ms
) VALUES ($1, now(), 'gpt-test', 'gpt-test', 'gpt-test', 10, 20, 0, 5, 0.15, 0.12, 321)`, key.Sub2APIKeyID)
	if errUsage != nil {
		t.Fatalf("insert usage: %v", errUsage)
	}
	summary, errSummary := service.UsageSummary(ctx, user, UsageFilter{})
	if errSummary != nil {
		t.Fatalf("usage summary: %v", errSummary)
	}
	if summary.RequestCount != 1 || summary.TotalTokens != 35 || summary.TotalCost != 0.15 {
		t.Fatalf("summary = %#v, want one 35-token row", summary)
	}

	order, errOrder := service.CreateRechargeOrder(ctx, user, 25.5, "USD", "manual test")
	if errOrder != nil {
		t.Fatalf("create recharge order: %v", errOrder)
	}
	admin, errAdmin := store.GetUserByEmail(ctx, "admin@example.com")
	if errAdmin != nil {
		t.Fatalf("get admin: %v", errAdmin)
	}
	ledger, errConfirm := service.AdminConfirmRechargeOrder(ctx, admin, order.ID)
	if errConfirm != nil {
		t.Fatalf("confirm recharge order: %v", errConfirm)
	}
	if ledger.Amount != 25.5 || ledger.Sub2APIBalanceAfter == nil || *ledger.Sub2APIBalanceAfter != 25.5 {
		t.Fatalf("ledger = %#v, want 25.5 balance", ledger)
	}
}

func resetPortalIntegrationDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	_, errExec := pool.Exec(ctx, `
DROP SCHEMA IF EXISTS portal CASCADE;
DROP TABLE IF EXISTS public.usage_logs;
CREATE TABLE public.usage_logs (
  id BIGSERIAL PRIMARY KEY,
  api_key_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  model TEXT,
  requested_model TEXT,
  upstream_model TEXT,
  input_tokens BIGINT,
  output_tokens BIGINT,
  cache_creation_tokens BIGINT,
  cache_read_tokens BIGINT,
  total_cost DOUBLE PRECISION,
  actual_cost DOUBLE PRECISION,
  duration_ms DOUBLE PRECISION
);`)
	return errExec
}
