package cpadashboard

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store reads dashboard data from Postgres.
type Store struct {
	pool *pgxpool.Pool
}

// DatabaseConfig describes the Postgres target used by the capacity reports.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN returns a pgx-compatible Postgres connection string.
func (c DatabaseConfig) DSN() string {
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     "/" + c.Name,
		RawQuery: q.Encode(),
	}
	return u.String()
}

// NewStore opens a Postgres connection pool.
func NewStore(ctx context.Context, cfg DatabaseConfig) (*Store, error) {
	pool, errParse := pgxpool.New(ctx, cfg.DSN())
	if errParse != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: open pool: %w", errParse)
	}
	if errPing := pool.Ping(ctx); errPing != nil {
		pool.Close()
		return nil, fmt.Errorf("cpa dashboard postgres: ping: %w", errPing)
	}
	return &Store{pool: pool}, nil
}

// Close closes the underlying connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) QuotaObservationWindows(ctx context.Context, filter QueryFilter) ([]QuotaObservationWindow, error) {
	rows, errQuery := s.pool.Query(ctx, `
SELECT
  cpa_source,
  MIN(collected_at) AS first_collected_at,
  MAX(collected_at) AS latest_collected_at
FROM cpa_monitor.cpa_quota_snapshots
WHERE status = 'success'
  AND data_stale = false
  AND weekly_remaining_percent IS NOT NULL
  AND collected_at < $1
  AND ($2 = '' OR cpa_source = $2)
GROUP BY cpa_source
ORDER BY cpa_source`, filter.End, filter.CPA)
	if errQuery != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: query quota observation windows: %w", errQuery)
	}
	defer rows.Close()

	windows := make([]QuotaObservationWindow, 0)
	for rows.Next() {
		var window QuotaObservationWindow
		if errScan := rows.Scan(&window.CPASource, &window.FirstCollectedAt, &window.LatestCollectedAt); errScan != nil {
			return nil, fmt.Errorf("cpa dashboard postgres: scan quota observation window: %w", errScan)
		}
		windows = append(windows, window)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: quota observation window rows: %w", errRows)
	}
	return windows, nil
}

func (s *Store) QuotaPoints(ctx context.Context, filter QueryFilter) ([]QuotaPoint, error) {
	rows, errQuery := s.pool.Query(ctx, `
WITH successful_points AS (
  SELECT
    cpa_source,
    auth_file,
    account_display,
    COALESCE(NULLIF(account_display, ''), auth_file) AS account_identity,
    collected_at,
    weekly_remaining_percent::float8 AS weekly_remaining_percent
  FROM cpa_monitor.cpa_quota_snapshots
  WHERE status = 'success'
    AND data_stale = false
    AND weekly_remaining_percent IS NOT NULL
    AND ($3 = '' OR cpa_source = $3)
),
latest_before AS (
  SELECT DISTINCT ON (cpa_source, account_identity)
    cpa_source, auth_file, account_display, collected_at, weekly_remaining_percent
  FROM successful_points
  WHERE collected_at < $1
  ORDER BY cpa_source, account_identity, collected_at DESC
),
in_range AS (
  SELECT
    cpa_source, auth_file, account_display, collected_at, weekly_remaining_percent
  FROM successful_points
  WHERE collected_at >= $1
    AND collected_at < $2
)
SELECT cpa_source, auth_file, account_display, collected_at, weekly_remaining_percent
FROM (
  SELECT * FROM latest_before
  UNION ALL
  SELECT * FROM in_range
) points
ORDER BY cpa_source, auth_file, collected_at`, filter.Start, filter.End, filter.CPA)
	if errQuery != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: query quota points: %w", errQuery)
	}
	defer rows.Close()

	points := make([]QuotaPoint, 0)
	for rows.Next() {
		var point QuotaPoint
		if errScan := rows.Scan(&point.CPASource, &point.AuthFile, &point.AccountEmail, &point.CollectedAt, &point.WeeklyRemain); errScan != nil {
			return nil, fmt.Errorf("cpa dashboard postgres: scan quota point: %w", errScan)
		}
		points = append(points, point)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: quota point rows: %w", errRows)
	}
	return points, nil
}

func (s *Store) UsageSummaries(ctx context.Context, filter QueryFilter) ([]UsageSummary, error) {
	rows, errQuery := s.pool.Query(ctx, `
WITH usage_with_cpa AS (
  SELECT
    u.*,
    CASE
      WHEN COALESCE(a.credentials->>'base_url', '') LIKE '%cpa1:%' OR a.name ILIKE 'cpa1%' THEN 'cpa1'
      WHEN COALESCE(a.credentials->>'base_url', '') LIKE '%cpa2:%' OR a.name ILIKE 'cpa2%' THEN 'cpa2'
      WHEN COALESCE(a.credentials->>'base_url', '') LIKE '%cpa3:%' OR a.name ILIKE 'cpa3%' THEN 'cpa3'
      ELSE COALESCE(NULLIF(a.name, ''), 'unknown')
    END AS cpa_source
  FROM public.usage_logs u
  LEFT JOIN public.accounts a ON a.id = u.account_id
  WHERE u.created_at >= $1
    AND u.created_at < $2
    AND ($4 = '' OR COALESCE(u.model, '') = $4 OR COALESCE(u.requested_model, '') = $4 OR COALESCE(u.upstream_model, '') = $4)
    AND ($5::bigint = 0 OR u.api_key_id = $5)
)
SELECT
  cpa_source,
  COUNT(*)::bigint,
  COALESCE(SUM(input_tokens), 0)::bigint,
  COALESCE(SUM(output_tokens), 0)::bigint,
  COALESCE(SUM(cache_creation_tokens), 0)::bigint,
  COALESCE(SUM(cache_read_tokens), 0)::bigint,
  COALESCE(SUM(cache_creation_5m_tokens), 0)::bigint,
  COALESCE(SUM(cache_creation_1h_tokens), 0)::bigint,
  COALESCE(SUM(total_cost), 0)::float8,
  COALESCE(SUM(actual_cost), 0)::float8,
  COALESCE(AVG(duration_ms), 0)::float8
FROM usage_with_cpa
WHERE ($3 = '' OR cpa_source = $3)
GROUP BY cpa_source
ORDER BY cpa_source`, filter.Start, filter.End, filter.CPA, filter.Model, filter.APIKeyID)
	if errQuery != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: query usage summaries: %w", errQuery)
	}
	defer rows.Close()

	summaries := make([]UsageSummary, 0)
	for rows.Next() {
		var item UsageSummary
		if errScan := rows.Scan(
			&item.CPASource,
			&item.RequestCount,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.CacheCreation5MTokens,
			&item.CacheCreation1HTokens,
			&item.TotalCost,
			&item.ActualCost,
			&item.AverageDurationMS,
		); errScan != nil {
			return nil, fmt.Errorf("cpa dashboard postgres: scan usage summary: %w", errScan)
		}
		summaries = append(summaries, item)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: usage summary rows: %w", errRows)
	}
	return summaries, nil
}

func (s *Store) CurrentAccounts(ctx context.Context, cpa string) ([]CPAAccountRow, error) {
	rows, errQuery := s.pool.Query(ctx, `
WITH latest_run AS (
  SELECT id FROM cpa_monitor.cpa_quota_collection_runs ORDER BY id DESC LIMIT 1
),
current_rows AS (
  SELECT s.*
  FROM cpa_monitor.cpa_quota_snapshots s
  JOIN latest_run r ON r.id = s.run_id
  WHERE ($1 = '' OR s.cpa_source = $1)
),
ranked AS (
  SELECT
    current_rows.*,
    ROW_NUMBER() OVER (
      PARTITION BY cpa_source, account_display
      ORDER BY
        CASE WHEN status = 'success' AND data_stale = false THEN 0 ELSE 1 END,
        collected_at DESC,
        auth_file ASC
    ) AS rn,
    MAX(CASE WHEN status = 'success' AND data_stale = false THEN auth_file ELSE '' END)
      OVER (PARTITION BY cpa_source, account_display) AS success_auth_file
  FROM current_rows
)
SELECT
  cpa_source,
  account_display,
  auth_file,
  account_plan,
  status,
  weekly_remaining_percent::float8,
  five_hour_remaining_percent::float8,
  weekly_reset_at,
  five_hour_reset_at,
  collected_at,
  data_stale,
  error_category,
  error_message,
  CASE WHEN success_auth_file <> auth_file THEN success_auth_file ELSE '' END AS alternative_success_auth_file
FROM ranked
WHERE rn = 1
ORDER BY cpa_source, account_display`, cpa)
	if errQuery != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: query current accounts: %w", errQuery)
	}
	defer rows.Close()

	out := make([]CPAAccountRow, 0)
	for rows.Next() {
		var row CPAAccountRow
		var weekly sql.NullFloat64
		var fiveHour sql.NullFloat64
		var weeklyReset sql.NullTime
		var fiveHourReset sql.NullTime
		if errScan := rows.Scan(
			&row.CPASource,
			&row.AccountEmail,
			&row.AuthFile,
			&row.AccountPlan,
			&row.Status,
			&weekly,
			&fiveHour,
			&weeklyReset,
			&fiveHourReset,
			&row.CollectedAt,
			&row.DataStale,
			&row.ErrorCategory,
			&row.ErrorMessage,
			&row.AlternativeSuccessAuthFile,
		); errScan != nil {
			return nil, fmt.Errorf("cpa dashboard postgres: scan current account: %w", errScan)
		}
		row.WeeklyRemainingPercent = nullableFloatPtr(weekly)
		row.FiveHourRemainingPercent = nullableFloatPtr(fiveHour)
		row.WeeklyResetAt = nullableTimePtr(weeklyReset)
		row.FiveHourResetAt = nullableTimePtr(fiveHourReset)
		out = append(out, row)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: current account rows: %w", errRows)
	}
	return out, nil
}

func (s *Store) LatestCollectionAt(ctx context.Context) (*time.Time, error) {
	var latest sql.NullTime
	errQuery := s.pool.QueryRow(ctx, `
SELECT MAX(finished_at) FROM cpa_monitor.cpa_quota_collection_runs WHERE status = 'success'`).Scan(&latest)
	if errQuery != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: query latest collection: %w", errQuery)
	}
	return nullableTimePtr(latest), nil
}

func (s *Store) UsageBuckets(ctx context.Context, filter QueryFilter) ([]UsageBucket, error) {
	rows, errQuery := s.pool.Query(ctx, `
WITH usage_with_cpa AS (
  SELECT
    u.*,
    CASE
      WHEN COALESCE(a.credentials->>'base_url', '') LIKE '%cpa1:%' OR a.name ILIKE 'cpa1%' THEN 'cpa1'
      WHEN COALESCE(a.credentials->>'base_url', '') LIKE '%cpa2:%' OR a.name ILIKE 'cpa2%' THEN 'cpa2'
      WHEN COALESCE(a.credentials->>'base_url', '') LIKE '%cpa3:%' OR a.name ILIKE 'cpa3%' THEN 'cpa3'
      ELSE COALESCE(NULLIF(a.name, ''), 'unknown')
    END AS cpa_source
  FROM public.usage_logs u
  LEFT JOIN public.accounts a ON a.id = u.account_id
  WHERE u.created_at >= $1
    AND u.created_at < $2
    AND ($4 = '' OR COALESCE(u.model, '') = $4 OR COALESCE(u.requested_model, '') = $4 OR COALESCE(u.upstream_model, '') = $4)
    AND ($5::bigint = 0 OR u.api_key_id = $5)
)
SELECT
  date_trunc('hour', created_at) AS bucket_start,
  cpa_source,
  COUNT(*)::bigint,
  COALESCE(SUM(input_tokens), 0)::bigint,
  COALESCE(SUM(output_tokens), 0)::bigint,
  COALESCE(SUM(cache_creation_tokens), 0)::bigint,
  COALESCE(SUM(cache_read_tokens), 0)::bigint,
  COALESCE(SUM(total_cost), 0)::float8,
  COALESCE(SUM(actual_cost), 0)::float8
FROM usage_with_cpa
WHERE ($3 = '' OR cpa_source = $3)
GROUP BY bucket_start, cpa_source
ORDER BY bucket_start, cpa_source`, filter.Start, filter.End, filter.CPA, filter.Model, filter.APIKeyID)
	if errQuery != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: query usage buckets: %w", errQuery)
	}
	defer rows.Close()

	out := make([]UsageBucket, 0)
	for rows.Next() {
		var item UsageBucket
		if errScan := rows.Scan(
			&item.BucketStart,
			&item.CPASource,
			&item.RequestCount,
			&item.InputTokens,
			&item.OutputTokens,
			&item.CacheCreationTokens,
			&item.CacheReadTokens,
			&item.TotalCost,
			&item.ActualCost,
		); errScan != nil {
			return nil, fmt.Errorf("cpa dashboard postgres: scan usage bucket: %w", errScan)
		}
		out = append(out, item)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: usage bucket rows: %w", errRows)
	}
	return out, nil
}

func (s *Store) CleanupCandidates(ctx context.Context, cpa string) ([]CleanupCandidate, error) {
	rows, errQuery := s.pool.Query(ctx, `
WITH latest_file AS (
  SELECT DISTINCT ON (cpa_source, auth_file)
    cpa_source, auth_file, account_display, status, error_category, error_message, collected_at, data_stale
  FROM cpa_monitor.cpa_quota_snapshots
  WHERE ($1 = '' OR cpa_source = $1)
  ORDER BY cpa_source, auth_file, collected_at DESC
),
failures AS (
  SELECT cpa_source, auth_file, COUNT(*)::bigint AS failure_count
  FROM cpa_monitor.cpa_quota_snapshots
  WHERE collected_at >= now() - interval '30 days'
    AND (status <> 'success' OR data_stale = true)
    AND ($1 = '' OR cpa_source = $1)
  GROUP BY cpa_source, auth_file
),
success_email AS (
  SELECT DISTINCT cpa_source, account_display
  FROM latest_file
  WHERE status = 'success' AND data_stale = false
)
SELECT
  l.cpa_source,
  l.account_display,
  l.auth_file,
  l.status,
  l.error_category,
  l.error_message,
  l.collected_at,
  COALESCE(f.failure_count, 0)::bigint,
  EXISTS (
    SELECT 1
    FROM success_email s
    WHERE s.cpa_source = l.cpa_source
      AND s.account_display = l.account_display
  ) AS has_success_same_email
FROM latest_file l
LEFT JOIN failures f ON f.cpa_source = l.cpa_source AND f.auth_file = l.auth_file
WHERE l.status <> 'success' OR l.data_stale = true
ORDER BY l.cpa_source, has_success_same_email DESC, l.account_display, l.auth_file`, cpa)
	if errQuery != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: query cleanup candidates: %w", errQuery)
	}
	defer rows.Close()

	out := make([]CleanupCandidate, 0)
	for rows.Next() {
		var item CleanupCandidate
		if errScan := rows.Scan(
			&item.CPASource,
			&item.AccountEmail,
			&item.AuthFile,
			&item.Status,
			&item.ErrorCategory,
			&item.ErrorMessage,
			&item.LatestCollectedAt,
			&item.FailureSnapshotsLast30Day,
			&item.HasSuccessSameEmail,
		); errScan != nil {
			return nil, fmt.Errorf("cpa dashboard postgres: scan cleanup candidate: %w", errScan)
		}
		if item.HasSuccessSameEmail {
			item.Reason = "Same email has a current successful auth file; this failed file is a cleanup candidate."
		} else {
			item.Reason = "The latest snapshot is failed or stale; keep for review before cleanup."
		}
		out = append(out, item)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("cpa dashboard postgres: cleanup candidate rows: %w", errRows)
	}
	return out, nil
}

func (s *Store) Filters(ctx context.Context) (FiltersResponse, error) {
	var resp FiltersResponse
	cpaRows, errCPA := s.pool.Query(ctx, `
SELECT DISTINCT cpa_source
FROM cpa_monitor.cpa_quota_snapshots
WHERE cpa_source <> ''
ORDER BY cpa_source`)
	if errCPA != nil {
		return resp, fmt.Errorf("cpa dashboard postgres: query cpa filters: %w", errCPA)
	}
	for cpaRows.Next() {
		var value string
		if errScan := cpaRows.Scan(&value); errScan != nil {
			cpaRows.Close()
			return resp, fmt.Errorf("cpa dashboard postgres: scan cpa filter: %w", errScan)
		}
		resp.CPASources = append(resp.CPASources, FilterOption{ID: value, Label: value})
	}
	cpaRows.Close()
	if errRows := cpaRows.Err(); errRows != nil {
		return resp, fmt.Errorf("cpa dashboard postgres: cpa filter rows: %w", errRows)
	}

	modelRows, errModels := s.pool.Query(ctx, `
SELECT DISTINCT model
FROM public.usage_logs
WHERE model IS NOT NULL AND model <> ''
ORDER BY model`)
	if errModels != nil {
		return resp, fmt.Errorf("cpa dashboard postgres: query model filters: %w", errModels)
	}
	for modelRows.Next() {
		var value string
		if errScan := modelRows.Scan(&value); errScan != nil {
			modelRows.Close()
			return resp, fmt.Errorf("cpa dashboard postgres: scan model filter: %w", errScan)
		}
		resp.Models = append(resp.Models, FilterOption{ID: value, Label: value})
	}
	modelRows.Close()
	if errRows := modelRows.Err(); errRows != nil {
		return resp, fmt.Errorf("cpa dashboard postgres: model filter rows: %w", errRows)
	}

	keyRows, errKeys := s.pool.Query(ctx, `
SELECT id, COALESCE(NULLIF(name, ''), 'API Key ' || id::text)
FROM public.api_keys
WHERE COALESCE(status, '') <> 'deleted'
ORDER BY id`)
	if errKeys != nil {
		return resp, fmt.Errorf("cpa dashboard postgres: query api key filters: %w", errKeys)
	}
	for keyRows.Next() {
		var id int64
		var label string
		if errScan := keyRows.Scan(&id, &label); errScan != nil {
			keyRows.Close()
			return resp, fmt.Errorf("cpa dashboard postgres: scan api key filter: %w", errScan)
		}
		resp.APIKeys = append(resp.APIKeys, FilterOption{ID: strconv.FormatInt(id, 10), Label: label})
	}
	keyRows.Close()
	if errRows := keyRows.Err(); errRows != nil {
		return resp, fmt.Errorf("cpa dashboard postgres: api key filter rows: %w", errRows)
	}
	return resp, nil
}

func nullableFloatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func nullableTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}
