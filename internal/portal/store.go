package portal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists Portal data and reads scoped Sub2API usage from Postgres.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore opens a Postgres connection pool.
func NewStore(ctx context.Context, cfg DatabaseConfig) (*Store, error) {
	pool, errParse := pgxpool.New(ctx, cfg.DSN())
	if errParse != nil {
		return nil, fmt.Errorf("portal postgres: open pool: %w", errParse)
	}
	if errPing := pool.Ping(ctx); errPing != nil {
		pool.Close()
		return nil, fmt.Errorf("portal postgres: ping: %w", errPing)
	}
	return &Store{pool: pool}, nil
}

// Close closes the underlying connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Migrate creates the Portal schema and tables.
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("portal store is not initialized")
	}
	_, errExec := s.pool.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS portal;

CREATE TABLE IF NOT EXISTS portal.users (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS portal.sessions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES portal.users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS portal.sub2api_user_mappings (
  portal_user_id BIGINT PRIMARY KEY REFERENCES portal.users(id) ON DELETE CASCADE,
  sub2api_user_id BIGINT NOT NULL UNIQUE,
  sub2api_email TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS portal.api_keys (
  id BIGSERIAL PRIMARY KEY,
  portal_user_id BIGINT NOT NULL REFERENCES portal.users(id) ON DELETE CASCADE,
  sub2api_key_id BIGINT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  key_preview TEXT NOT NULL,
  group_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_portal_api_keys_user_created
  ON portal.api_keys (portal_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS portal.recharge_orders (
  id BIGSERIAL PRIMARY KEY,
  portal_user_id BIGINT NOT NULL REFERENCES portal.users(id) ON DELETE CASCADE,
  amount NUMERIC(18,6) NOT NULL,
  currency TEXT NOT NULL DEFAULT 'USD',
  status TEXT NOT NULL DEFAULT 'pending',
  note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  confirmed_at TIMESTAMPTZ,
  confirmed_by BIGINT REFERENCES portal.users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_portal_recharge_orders_user_created
  ON portal.recharge_orders (portal_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_portal_recharge_orders_status_created
  ON portal.recharge_orders (status, created_at DESC);

CREATE TABLE IF NOT EXISTS portal.ledger_entries (
  id BIGSERIAL PRIMARY KEY,
  portal_user_id BIGINT NOT NULL REFERENCES portal.users(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  amount NUMERIC(18,6) NOT NULL,
  currency TEXT NOT NULL DEFAULT 'USD',
  related_order_id BIGINT REFERENCES portal.recharge_orders(id) ON DELETE SET NULL,
  sub2api_balance_after NUMERIC(18,6),
  note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_portal_ledger_user_created
  ON portal.ledger_entries (portal_user_id, created_at DESC);
`)
	if errExec != nil {
		return fmt.Errorf("portal postgres: migrate: %w", errExec)
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, email string, passwordHash string, role string) (User, error) {
	var user User
	errQuery := s.pool.QueryRow(ctx, `
INSERT INTO portal.users (email, password_hash, role, status)
VALUES ($1, $2, $3, $4)
RETURNING id, email, password_hash, role, status, created_at`,
		email, passwordHash, role, statusActive).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
	)
	if errQuery != nil {
		if isUniqueViolation(errQuery) {
			return User{}, ErrAlreadyExists
		}
		return User{}, fmt.Errorf("portal postgres: create user: %w", errQuery)
	}
	return user, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	return s.scanUser(ctx, `
SELECT id, email, password_hash, role, status, created_at
FROM portal.users
WHERE email = $1`, email)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	return s.scanUser(ctx, `
SELECT id, email, password_hash, role, status, created_at
FROM portal.users
WHERE id = $1`, id)
}

func (s *Store) scanUser(ctx context.Context, query string, args ...any) (User, error) {
	var user User
	errQuery := s.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
	)
	if errQuery != nil {
		if errors.Is(errQuery, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("portal postgres: scan user: %w", errQuery)
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (Session, error) {
	var session Session
	errQuery := s.pool.QueryRow(ctx, `
INSERT INTO portal.sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING id, user_id, token_hash, expires_at, created_at`,
		userID, tokenHash, expiresAt).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if errQuery != nil {
		return Session{}, fmt.Errorf("portal postgres: create session: %w", errQuery)
	}
	return session, nil
}

func (s *Store) UserBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	var user User
	errQuery := s.pool.QueryRow(ctx, `
SELECT u.id, u.email, u.password_hash, u.role, u.status, u.created_at
FROM portal.sessions sess
JOIN portal.users u ON u.id = sess.user_id
WHERE sess.token_hash = $1
  AND sess.expires_at > $2
  AND u.status = 'active'`, tokenHash, now).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
	)
	if errQuery != nil {
		if errors.Is(errQuery, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("portal postgres: user by session: %w", errQuery)
	}
	return user, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, errExec := s.pool.Exec(ctx, `DELETE FROM portal.sessions WHERE token_hash = $1`, tokenHash)
	if errExec != nil {
		return fmt.Errorf("portal postgres: delete session: %w", errExec)
	}
	return nil
}

func (s *Store) UpsertSub2APIUserMapping(ctx context.Context, mapping Sub2APIUserMapping) error {
	_, errExec := s.pool.Exec(ctx, `
INSERT INTO portal.sub2api_user_mappings (portal_user_id, sub2api_user_id, sub2api_email)
VALUES ($1, $2, $3)
ON CONFLICT (portal_user_id) DO UPDATE
SET sub2api_user_id = EXCLUDED.sub2api_user_id,
    sub2api_email = EXCLUDED.sub2api_email`,
		mapping.PortalUserID, mapping.Sub2APIUserID, mapping.Sub2APIEmail)
	if errExec != nil {
		return fmt.Errorf("portal postgres: upsert sub2api mapping: %w", errExec)
	}
	return nil
}

func (s *Store) GetSub2APIUserMapping(ctx context.Context, portalUserID int64) (Sub2APIUserMapping, error) {
	var mapping Sub2APIUserMapping
	errQuery := s.pool.QueryRow(ctx, `
SELECT portal_user_id, sub2api_user_id, sub2api_email, created_at
FROM portal.sub2api_user_mappings
WHERE portal_user_id = $1`, portalUserID).Scan(
		&mapping.PortalUserID,
		&mapping.Sub2APIUserID,
		&mapping.Sub2APIEmail,
		&mapping.CreatedAt,
	)
	if errQuery != nil {
		if errors.Is(errQuery, pgx.ErrNoRows) {
			return Sub2APIUserMapping{}, ErrNotFound
		}
		return Sub2APIUserMapping{}, fmt.Errorf("portal postgres: get sub2api mapping: %w", errQuery)
	}
	return mapping, nil
}

func (s *Store) CreateAPIKey(ctx context.Context, item APIKey) (APIKey, error) {
	var out APIKey
	errQuery := s.pool.QueryRow(ctx, `
INSERT INTO portal.api_keys (portal_user_id, sub2api_key_id, name, key_preview, group_name, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, portal_user_id, sub2api_key_id, name, key_preview, group_name, status, created_at`,
		item.PortalUserID, item.Sub2APIKeyID, item.Name, item.KeyPreview, item.GroupName, statusActive).Scan(
		&out.ID,
		&out.PortalUserID,
		&out.Sub2APIKeyID,
		&out.Name,
		&out.KeyPreview,
		&out.GroupName,
		&out.Status,
		&out.CreatedAt,
	)
	if errQuery != nil {
		return APIKey{}, fmt.Errorf("portal postgres: create api key mapping: %w", errQuery)
	}
	return out, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, userID int64) ([]APIKey, error) {
	rows, errQuery := s.pool.Query(ctx, `
SELECT id, portal_user_id, sub2api_key_id, name, key_preview, group_name, status, created_at
FROM portal.api_keys
WHERE portal_user_id = $1
ORDER BY created_at DESC`, userID)
	if errQuery != nil {
		return nil, fmt.Errorf("portal postgres: list api keys: %w", errQuery)
	}
	defer rows.Close()

	items := make([]APIKey, 0)
	for rows.Next() {
		var item APIKey
		if errScan := rows.Scan(
			&item.ID,
			&item.PortalUserID,
			&item.Sub2APIKeyID,
			&item.Name,
			&item.KeyPreview,
			&item.GroupName,
			&item.Status,
			&item.CreatedAt,
		); errScan != nil {
			return nil, fmt.Errorf("portal postgres: scan api key: %w", errScan)
		}
		items = append(items, item)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("portal postgres: api key rows: %w", errRows)
	}
	return items, nil
}

func (s *Store) GetAPIKey(ctx context.Context, userID int64, keyID int64) (APIKey, error) {
	var item APIKey
	errQuery := s.pool.QueryRow(ctx, `
SELECT id, portal_user_id, sub2api_key_id, name, key_preview, group_name, status, created_at
FROM portal.api_keys
WHERE portal_user_id = $1
  AND id = $2`, userID, keyID).Scan(
		&item.ID,
		&item.PortalUserID,
		&item.Sub2APIKeyID,
		&item.Name,
		&item.KeyPreview,
		&item.GroupName,
		&item.Status,
		&item.CreatedAt,
	)
	if errQuery != nil {
		if errors.Is(errQuery, pgx.ErrNoRows) {
			return APIKey{}, ErrNotFound
		}
		return APIKey{}, fmt.Errorf("portal postgres: get api key: %w", errQuery)
	}
	return item, nil
}

func (s *Store) UsageSummary(ctx context.Context, userID int64, filter UsageFilter) (UsageSummary, error) {
	var out UsageSummary
	errQuery := s.pool.QueryRow(ctx, `
SELECT
  COUNT(u.id)::bigint,
  COALESCE(SUM(u.input_tokens), 0)::bigint,
  COALESCE(SUM(u.output_tokens), 0)::bigint,
  COALESCE(SUM(u.cache_creation_tokens), 0)::bigint,
  COALESCE(SUM(u.cache_read_tokens), 0)::bigint,
  COALESCE(SUM(
    COALESCE(u.input_tokens, 0) +
    COALESCE(u.output_tokens, 0) +
    COALESCE(u.cache_creation_tokens, 0) +
    COALESCE(u.cache_read_tokens, 0)
  ), 0)::bigint,
  COALESCE(SUM(u.total_cost), 0)::float8,
  COALESCE(SUM(u.actual_cost), 0)::float8,
  COALESCE(AVG(u.duration_ms), 0)::float8
FROM portal.api_keys k
LEFT JOIN public.usage_logs u ON u.api_key_id = k.sub2api_key_id
  AND u.created_at >= $2
  AND u.created_at < $3
WHERE k.portal_user_id = $1
  AND ($4::bigint = 0 OR k.id = $4)`,
		userID, filter.Start, filter.End, filter.APIKeyID).Scan(
		&out.RequestCount,
		&out.InputTokens,
		&out.OutputTokens,
		&out.CacheCreationTokens,
		&out.CacheReadTokens,
		&out.TotalTokens,
		&out.TotalCost,
		&out.ActualCost,
		&out.AverageDurationMS,
	)
	if errQuery != nil {
		return UsageSummary{}, fmt.Errorf("portal postgres: usage summary: %w", errQuery)
	}
	return out, nil
}

func (s *Store) UsageRecords(ctx context.Context, userID int64, filter UsageFilter) ([]UsageRecord, error) {
	rows, errQuery := s.pool.Query(ctx, `
SELECT
  u.id,
  u.created_at,
  k.id AS portal_api_key_id,
  k.name,
  COALESCE(u.model, ''),
  COALESCE(u.requested_model, ''),
  COALESCE(u.upstream_model, ''),
  COALESCE(u.input_tokens, 0)::bigint,
  COALESCE(u.output_tokens, 0)::bigint,
  COALESCE(u.cache_creation_tokens, 0)::bigint,
  COALESCE(u.cache_read_tokens, 0)::bigint,
  (
    COALESCE(u.input_tokens, 0) +
    COALESCE(u.output_tokens, 0) +
    COALESCE(u.cache_creation_tokens, 0) +
    COALESCE(u.cache_read_tokens, 0)
  )::bigint,
  COALESCE(u.total_cost, 0)::float8,
  COALESCE(u.actual_cost, 0)::float8,
  COALESCE(u.duration_ms, 0)::float8
FROM portal.api_keys k
JOIN public.usage_logs u ON u.api_key_id = k.sub2api_key_id
WHERE k.portal_user_id = $1
  AND u.created_at >= $2
  AND u.created_at < $3
  AND ($4::bigint = 0 OR k.id = $4)
ORDER BY u.created_at DESC, u.id DESC
LIMIT $5`, userID, filter.Start, filter.End, filter.APIKeyID, filter.Limit)
	if errQuery != nil {
		return nil, fmt.Errorf("portal postgres: usage records: %w", errQuery)
	}
	defer rows.Close()

	items := make([]UsageRecord, 0)
	for rows.Next() {
		var item UsageRecord
		if errScan := rows.Scan(
			&item.ID,
			&item.CreatedAt,
			&item.APIKeyID,
			&item.APIKeyName,
			&item.Model,
			&item.RequestedModel,
			&item.UpstreamModel,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalTokens,
			&item.TotalCost,
			&item.ActualCost,
			&item.DurationMS,
		); errScan != nil {
			return nil, fmt.Errorf("portal postgres: scan usage record: %w", errScan)
		}
		items = append(items, item)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("portal postgres: usage record rows: %w", errRows)
	}
	return items, nil
}

func (s *Store) CreateRechargeOrder(ctx context.Context, userID int64, amount float64, currency string, note string) (RechargeOrder, error) {
	var order RechargeOrder
	var confirmedAt sql.NullTime
	var confirmedBy sql.NullInt64
	errQuery := s.pool.QueryRow(ctx, `
INSERT INTO portal.recharge_orders (portal_user_id, amount, currency, status, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, portal_user_id, amount::float8, currency, status, note, created_at, confirmed_at, confirmed_by`,
		userID, amount, currency, statusPending, note).Scan(
		&order.ID,
		&order.UserID,
		&order.Amount,
		&order.Currency,
		&order.Status,
		&order.Note,
		&order.CreatedAt,
		&confirmedAt,
		&confirmedBy,
	)
	if errQuery != nil {
		return RechargeOrder{}, fmt.Errorf("portal postgres: create recharge order: %w", errQuery)
	}
	applyRechargeOrderNullableFields(&order, confirmedAt, confirmedBy)
	return order, nil
}

func (s *Store) ListRechargeOrders(ctx context.Context, userID int64, status string, admin bool) ([]RechargeOrder, error) {
	query := `
SELECT o.id, o.portal_user_id, u.email, o.amount::float8, o.currency, o.status, o.note, o.created_at, o.confirmed_at, o.confirmed_by
FROM portal.recharge_orders o
JOIN portal.users u ON u.id = o.portal_user_id
WHERE ($1::bigint = 0 OR o.portal_user_id = $1)
  AND ($2 = '' OR o.status = $2)
ORDER BY o.created_at DESC`
	if !admin {
		query = `
SELECT o.id, o.portal_user_id, '' AS email, o.amount::float8, o.currency, o.status, o.note, o.created_at, o.confirmed_at, o.confirmed_by
FROM portal.recharge_orders o
WHERE o.portal_user_id = $1
  AND ($2 = '' OR o.status = $2)
ORDER BY o.created_at DESC`
	}
	rows, errQuery := s.pool.Query(ctx, query, userID, status)
	if errQuery != nil {
		return nil, fmt.Errorf("portal postgres: list recharge orders: %w", errQuery)
	}
	defer rows.Close()

	items := make([]RechargeOrder, 0)
	for rows.Next() {
		var item RechargeOrder
		var confirmedAt sql.NullTime
		var confirmedBy sql.NullInt64
		if errScan := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.UserEmail,
			&item.Amount,
			&item.Currency,
			&item.Status,
			&item.Note,
			&item.CreatedAt,
			&confirmedAt,
			&confirmedBy,
		); errScan != nil {
			return nil, fmt.Errorf("portal postgres: scan recharge order: %w", errScan)
		}
		applyRechargeOrderNullableFields(&item, confirmedAt, confirmedBy)
		items = append(items, item)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("portal postgres: recharge order rows: %w", errRows)
	}
	return items, nil
}

func (s *Store) MarkRechargeOrderProcessing(ctx context.Context, orderID int64, adminID int64) (RechargeOrder, error) {
	var order RechargeOrder
	var confirmedAt sql.NullTime
	var confirmedBy sql.NullInt64
	errQuery := s.pool.QueryRow(ctx, `
UPDATE portal.recharge_orders
SET status = $3,
    confirmed_by = $2
WHERE id = $1
  AND status = $4
RETURNING id, portal_user_id, amount::float8, currency, status, note, created_at, confirmed_at, confirmed_by`,
		orderID, adminID, statusProcessing, statusPending).Scan(
		&order.ID,
		&order.UserID,
		&order.Amount,
		&order.Currency,
		&order.Status,
		&order.Note,
		&order.CreatedAt,
		&confirmedAt,
		&confirmedBy,
	)
	if errQuery != nil {
		if errors.Is(errQuery, pgx.ErrNoRows) {
			return RechargeOrder{}, ErrNotFound
		}
		return RechargeOrder{}, fmt.Errorf("portal postgres: mark recharge processing: %w", errQuery)
	}
	applyRechargeOrderNullableFields(&order, confirmedAt, confirmedBy)
	return order, nil
}

func (s *Store) ResetRechargeOrderPending(ctx context.Context, orderID int64) error {
	_, errExec := s.pool.Exec(ctx, `
UPDATE portal.recharge_orders
SET status = $2,
    confirmed_by = NULL
WHERE id = $1
  AND status = $3`, orderID, statusPending, statusProcessing)
	if errExec != nil {
		return fmt.Errorf("portal postgres: reset recharge pending: %w", errExec)
	}
	return nil
}

func (s *Store) ConfirmRechargeOrder(ctx context.Context, order RechargeOrder, adminID int64, balanceAfter float64) (LedgerEntry, error) {
	tx, errBegin := s.pool.Begin(ctx)
	if errBegin != nil {
		return LedgerEntry{}, fmt.Errorf("portal postgres: begin confirm recharge: %w", errBegin)
	}
	defer func() {
		if errRollback := tx.Rollback(ctx); errRollback != nil && !errors.Is(errRollback, pgx.ErrTxClosed) {
			// Nothing useful can be returned from a defer here; the caller receives the primary error path.
		}
	}()

	var confirmedAt time.Time
	errUpdate := tx.QueryRow(ctx, `
UPDATE portal.recharge_orders
SET status = $3,
    confirmed_at = now(),
    confirmed_by = $2
WHERE id = $1
  AND status = $4
RETURNING confirmed_at`,
		order.ID, adminID, statusConfirmed, statusProcessing).Scan(&confirmedAt)
	if errUpdate != nil {
		if errors.Is(errUpdate, pgx.ErrNoRows) {
			return LedgerEntry{}, ErrNotFound
		}
		return LedgerEntry{}, fmt.Errorf("portal postgres: confirm recharge order: %w", errUpdate)
	}
	order.ConfirmedAt = &confirmedAt

	var ledger LedgerEntry
	var relatedOrderID sql.NullInt64
	var balanceAfterValue sql.NullFloat64
	errLedger := tx.QueryRow(ctx, `
INSERT INTO portal.ledger_entries (
  portal_user_id, type, amount, currency, related_order_id, sub2api_balance_after, note
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, portal_user_id, type, amount::float8, currency, related_order_id, sub2api_balance_after::float8, note, created_at`,
		order.UserID, ledgerTypeRecharge, order.Amount, order.Currency, order.ID, balanceAfter, order.Note).Scan(
		&ledger.ID,
		&ledger.UserID,
		&ledger.Type,
		&ledger.Amount,
		&ledger.Currency,
		&relatedOrderID,
		&balanceAfterValue,
		&ledger.Note,
		&ledger.CreatedAt,
	)
	if errLedger != nil {
		return LedgerEntry{}, fmt.Errorf("portal postgres: create ledger entry: %w", errLedger)
	}
	if relatedOrderID.Valid {
		ledger.RelatedOrderID = &relatedOrderID.Int64
	}
	if balanceAfterValue.Valid {
		ledger.Sub2APIBalanceAfter = &balanceAfterValue.Float64
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return LedgerEntry{}, fmt.Errorf("portal postgres: commit confirm recharge: %w", errCommit)
	}
	return ledger, nil
}

func (s *Store) CancelRechargeOrder(ctx context.Context, orderID int64, adminID int64) error {
	tag, errExec := s.pool.Exec(ctx, `
UPDATE portal.recharge_orders
SET status = $3,
    confirmed_by = $2
WHERE id = $1
  AND status IN ($4, $5)`, orderID, adminID, statusCancelled, statusPending, statusProcessing)
	if errExec != nil {
		return fmt.Errorf("portal postgres: cancel recharge order: %w", errExec)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListLedgerEntries(ctx context.Context, userID int64) ([]LedgerEntry, error) {
	rows, errQuery := s.pool.Query(ctx, `
SELECT id, portal_user_id, type, amount::float8, currency, related_order_id, sub2api_balance_after::float8, note, created_at
FROM portal.ledger_entries
WHERE portal_user_id = $1
ORDER BY created_at DESC`, userID)
	if errQuery != nil {
		return nil, fmt.Errorf("portal postgres: list ledger: %w", errQuery)
	}
	defer rows.Close()

	items := make([]LedgerEntry, 0)
	for rows.Next() {
		var item LedgerEntry
		var relatedOrderID sql.NullInt64
		var balanceAfter sql.NullFloat64
		if errScan := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Type,
			&item.Amount,
			&item.Currency,
			&relatedOrderID,
			&balanceAfter,
			&item.Note,
			&item.CreatedAt,
		); errScan != nil {
			return nil, fmt.Errorf("portal postgres: scan ledger: %w", errScan)
		}
		if relatedOrderID.Valid {
			item.RelatedOrderID = &relatedOrderID.Int64
		}
		if balanceAfter.Valid {
			item.Sub2APIBalanceAfter = &balanceAfter.Float64
		}
		items = append(items, item)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("portal postgres: ledger rows: %w", errRows)
	}
	return items, nil
}

// Domain errors returned by Portal services and stores.
var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrForbidden     = errors.New("forbidden")
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func normalizeCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "USD"
	}
	return value
}

func applyRechargeOrderNullableFields(order *RechargeOrder, confirmedAt sql.NullTime, confirmedBy sql.NullInt64) {
	if confirmedAt.Valid {
		order.ConfirmedAt = &confirmedAt.Time
	}
	if confirmedBy.Valid {
		order.ConfirmedBy = &confirmedBy.Int64
	}
}
