package quotacollector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists collection runs and snapshots.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore opens a Postgres connection pool.
func NewStore(ctx context.Context, cfg DatabaseConfig) (*Store, error) {
	pool, errParse := pgxpool.New(ctx, cfg.DSN())
	if errParse != nil {
		return nil, fmt.Errorf("quota collector postgres: open pool: %w", errParse)
	}
	if errPing := pool.Ping(ctx); errPing != nil {
		pool.Close()
		return nil, fmt.Errorf("quota collector postgres: ping: %w", errPing)
	}
	return &Store{pool: pool}, nil
}

// Close closes the underlying connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Migrate creates the collector schema and tables.
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("quota collector store is not initialized")
	}
	_, errExec := s.pool.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS cpa_monitor;

CREATE TABLE IF NOT EXISTS cpa_monitor.cpa_quota_collection_runs (
  id BIGSERIAL PRIMARY KEY,
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ,
  reason TEXT NOT NULL,
  status TEXT NOT NULL,
  attempted_instances INTEGER NOT NULL DEFAULT 0,
  successful_instances INTEGER NOT NULL DEFAULT 0,
  failed_instances INTEGER NOT NULL DEFAULT 0,
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cpa_monitor.cpa_quota_snapshots (
  id BIGSERIAL PRIMARY KEY,
  run_id BIGINT REFERENCES cpa_monitor.cpa_quota_collection_runs(id) ON DELETE SET NULL,
  collected_at TIMESTAMPTZ NOT NULL,
  cpa_source TEXT NOT NULL,
  auth_file TEXT NOT NULL DEFAULT '',
  auth_index TEXT NOT NULL DEFAULT '',
  account_display TEXT NOT NULL DEFAULT '',
  account_plan TEXT NOT NULL DEFAULT 'unknown',
  status TEXT NOT NULL,
  weekly_remaining_percent NUMERIC(8,4),
  five_hour_remaining_percent NUMERIC(8,4),
  weekly_reset_at TIMESTAMPTZ,
  five_hour_reset_at TIMESTAMPTZ,
  quota_json JSONB,
  collection_reason TEXT NOT NULL,
  error_category TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  error_raw JSONB,
  data_stale BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cpa_quota_snapshots_collected_at
  ON cpa_monitor.cpa_quota_snapshots (collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_cpa_quota_snapshots_source_auth_time
  ON cpa_monitor.cpa_quota_snapshots (cpa_source, auth_file, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_cpa_quota_snapshots_status_time
  ON cpa_monitor.cpa_quota_snapshots (status, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_cpa_quota_collection_runs_started_at
  ON cpa_monitor.cpa_quota_collection_runs (started_at DESC);
`)
	if errExec != nil {
		return fmt.Errorf("quota collector postgres: migrate: %w", errExec)
	}
	return nil
}

func (s *Store) CreateRun(ctx context.Context, reason string, startedAt time.Time, attempted int) (int64, error) {
	var id int64
	errQuery := s.pool.QueryRow(ctx, `
INSERT INTO cpa_monitor.cpa_quota_collection_runs
  (started_at, reason, status, attempted_instances)
VALUES ($1, $2, $3, $4)
RETURNING id`, startedAt, reason, RunStatusError, attempted).Scan(&id)
	if errQuery != nil {
		return 0, fmt.Errorf("quota collector postgres: create run: %w", errQuery)
	}
	return id, nil
}

func (s *Store) FinishRun(ctx context.Context, run CollectionRun) error {
	_, errExec := s.pool.Exec(ctx, `
UPDATE cpa_monitor.cpa_quota_collection_runs
SET finished_at = $2,
    status = $3,
    successful_instances = $4,
    failed_instances = $5,
    error_message = $6
WHERE id = $1`, run.ID, run.FinishedAt, run.Status, run.SuccessfulInstances, run.FailedInstances, run.ErrorMessage)
	if errExec != nil {
		return fmt.Errorf("quota collector postgres: finish run: %w", errExec)
	}
	return nil
}

func (s *Store) InsertSnapshots(ctx context.Context, snapshots []Snapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, snapshot := range snapshots {
		batch.Queue(`
INSERT INTO cpa_monitor.cpa_quota_snapshots (
  run_id, collected_at, cpa_source, auth_file, auth_index, account_display, account_plan,
  status, weekly_remaining_percent, five_hour_remaining_percent, weekly_reset_at,
  five_hour_reset_at, quota_json, collection_reason, error_category, error_message,
  error_raw, data_stale
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11,
  $12, $13, $14, $15, $16,
  $17, $18
)`,
			snapshot.RunID, snapshot.CollectedAt, snapshot.CPASource, snapshot.AuthFile,
			snapshot.AuthIndex, snapshot.AccountDisplay, snapshot.AccountPlan, snapshot.Status,
			snapshot.WeeklyRemainingPercent, snapshot.FiveHourRemainingPercent,
			snapshot.WeeklyResetAt, snapshot.FiveHourResetAt, jsonOrNil(snapshot.QuotaJSON),
			snapshot.CollectionReason, snapshot.ErrorCategory, snapshot.ErrorMessage,
			jsonOrNil(snapshot.ErrorRaw), snapshot.DataStale)
	}
	results := s.pool.SendBatch(ctx, batch)
	for range snapshots {
		if _, errExec := results.Exec(); errExec != nil {
			_ = results.Close()
			return fmt.Errorf("quota collector postgres: insert snapshot: %w", errExec)
		}
	}
	if errClose := results.Close(); errClose != nil {
		return fmt.Errorf("quota collector postgres: close snapshot batch: %w", errClose)
	}
	return nil
}

func jsonOrNil(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return string(raw)
}
