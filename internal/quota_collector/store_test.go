package quotacollector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreMigrateAndInsertSnapshots(t *testing.T) {
	dsn := os.Getenv("CPA_QUOTA_TEST_DSN")
	if dsn == "" {
		t.Skip("CPA_QUOTA_TEST_DSN is not set")
	}

	ctx := context.Background()
	pool, errPool := pgxpool.New(ctx, dsn)
	if errPool != nil {
		t.Fatalf("open test pool: %v", errPool)
	}
	defer pool.Close()

	store := &Store{pool: pool}
	if errMigrate := store.Migrate(ctx); errMigrate != nil {
		t.Fatalf("migrate: %v", errMigrate)
	}
	if errMigrate := store.Migrate(ctx); errMigrate != nil {
		t.Fatalf("second migrate: %v", errMigrate)
	}

	startedAt := time.Now().UTC()
	runID, errRun := store.CreateRun(ctx, ReasonHourly, startedAt, 1)
	if errRun != nil {
		t.Fatalf("create run: %v", errRun)
	}
	remaining := 44.0
	if errInsert := store.InsertSnapshots(ctx, []Snapshot{
		{
			RunID:                    runID,
			CollectedAt:              startedAt,
			CPASource:                "test-cpa",
			AuthFile:                 "test-auth.json",
			AccountPlan:              "plus",
			Status:                   StatusSuccess,
			FiveHourRemainingPercent: &remaining,
			CollectionReason:         ReasonHourly,
		},
		{
			RunID:            runID,
			CollectedAt:      startedAt,
			CPASource:        "test-cpa",
			AuthFile:         "failed-auth.json",
			AccountPlan:      "unknown",
			Status:           StatusError,
			CollectionReason: ReasonHourly,
			ErrorCategory:    ErrorAuthExpired,
			ErrorMessage:     "missing access token",
			DataStale:        true,
		},
	}); errInsert != nil {
		t.Fatalf("insert snapshots: %v", errInsert)
	}

	if errFinish := store.FinishRun(ctx, CollectionRun{
		ID:                  runID,
		FinishedAt:          time.Now().UTC(),
		Status:              RunStatusPartial,
		SuccessfulInstances: 1,
		FailedInstances:     0,
	}); errFinish != nil {
		t.Fatalf("finish run: %v", errFinish)
	}

	var count int
	if errCount := pool.QueryRow(ctx, `SELECT count(*) FROM cpa_monitor.cpa_quota_snapshots WHERE run_id = $1`, runID).Scan(&count); errCount != nil {
		t.Fatalf("count snapshots: %v", errCount)
	}
	if count != 2 {
		t.Fatalf("snapshot count = %d, want 2", count)
	}
}
