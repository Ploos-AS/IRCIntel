package core

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPostgresRollupBackfillIsBoundedAndResumable(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, `
TRUNCATE observation_rollups_hourly, observation_rollups_daily, observations RESTART IDENTITY;
DROP TABLE IF EXISTS observation_rollup_backfill_state`); err != nil {
		t.Fatal(err)
	}

	for _, row := range []struct {
		agentID string
		at      time.Time
		reach   bool
	}{
		{"a", time.Date(2026, 9, 1, 1, 15, 0, 0, time.UTC), true},
		{"b", time.Date(2026, 9, 2, 2, 30, 0, 0, time.UTC), false},
		{"c", time.Date(2026, 9, 3, 3, 45, 0, 0, time.UTC), true},
	} {
		if _, err := store.pool.Exec(ctx, `
INSERT INTO observations(agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json)
VALUES ($1, $2, 'irc.backfill.example', '6697', true,
        jsonb_build_object('result', jsonb_build_object('reachable', $3, 'dual_stack_ok', false)))`, row.agentID, row.at, row.reach); err != nil {
			t.Fatal(err)
		}
	}

	status, err := store.ObservationRollupBackfillStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Initialized {
		t.Fatalf("unexpected initialized status: %+v", status)
	}

	status, err = store.RunObservationRollupBackfillBatch(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Initialized || status.Completed || status.NextStart == nil || status.UpperBound == nil {
		t.Fatalf("unexpected first batch status: %+v", status)
	}
	wantNext := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	if !status.NextStart.Equal(wantNext) {
		t.Fatalf("next_start=%s want=%s", status.NextStart, wantNext)
	}
	var firstBatchCount int64
	if err := store.pool.QueryRow(ctx, `SELECT COALESCE(SUM(observation_count),0) FROM observation_rollups_daily`).Scan(&firstBatchCount); err != nil {
		t.Fatal(err)
	}
	if firstBatchCount != 1 {
		t.Fatalf("first batch count=%d want=1", firstBatchCount)
	}

	// Simulate a Core restart between batches. The checkpoint must survive and
	// the next run must resume from the persisted boundary rather than restart.
	databaseURL := os.Getenv("IRCINTEL_TEST_POSTGRES_URL")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	resumed, err := OpenPostgresStore(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()

	persisted, err := resumed.ObservationRollupBackfillStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.Initialized || persisted.NextStart == nil || !persisted.NextStart.Equal(wantNext) {
		t.Fatalf("checkpoint did not survive restart: %+v", persisted)
	}

	status, err = resumed.RunObservationRollupBackfillBatch(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if status.Completed {
		t.Fatalf("second batch completed too early: %+v", status)
	}
	status, err = resumed.RunObservationRollupBackfillBatch(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Completed || status.NextStart == nil || status.UpperBound == nil || !status.NextStart.Equal(*status.UpperBound) {
		t.Fatalf("final status=%+v", status)
	}

	var dailyTotal, hourlyTotal int64
	if err := resumed.pool.QueryRow(ctx, `SELECT COALESCE(SUM(observation_count),0) FROM observation_rollups_daily`).Scan(&dailyTotal); err != nil {
		t.Fatal(err)
	}
	if err := resumed.pool.QueryRow(ctx, `SELECT COALESCE(SUM(observation_count),0) FROM observation_rollups_hourly`).Scan(&hourlyTotal); err != nil {
		t.Fatal(err)
	}
	if dailyTotal != 3 || hourlyTotal != 3 {
		t.Fatalf("rollup totals daily=%d hourly=%d want=3/3", dailyTotal, hourlyTotal)
	}

	// Completed runs are idempotent and must not inflate aggregates.
	status, err = resumed.RunObservationRollupBackfillBatch(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Completed {
		t.Fatalf("completed checkpoint regressed: %+v", status)
	}
	var after int64
	if err := resumed.pool.QueryRow(ctx, `SELECT COALESCE(SUM(observation_count),0) FROM observation_rollups_daily`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != 3 {
		t.Fatalf("idempotent rerun inflated totals=%d", after)
	}
}

func TestPostgresRollupBackfillValidation(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.RunObservationRollupBackfillBatch(0); err == nil {
		t.Fatal("expected zero span error")
	}
	if _, err := store.RunObservationRollupBackfillBatch(maxRollupBackfillSpan + time.Hour); err == nil {
		t.Fatal("expected excessive span error")
	}
}
