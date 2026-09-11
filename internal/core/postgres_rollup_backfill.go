package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxRollupBackfillSpan = 90 * 24 * time.Hour

type RollupBackfillStatus struct {
	Initialized bool       `json:"initialized"`
	Completed   bool       `json:"completed"`
	NextStart   *time.Time `json:"next_start,omitempty"`
	UpperBound  *time.Time `json:"upper_bound,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

func ensurePostgresRollupBackfillTable(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS observation_rollup_backfill_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    next_start TIMESTAMPTZ,
    upper_bound TIMESTAMPTZ,
    completed BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	return err
}

func scanRollupBackfillStatus(row pgx.Row) (RollupBackfillStatus, error) {
	var next, upper, updated pgtype.Timestamptz
	var completed bool
	if err := row.Scan(&next, &upper, &completed, &updated); err != nil {
		return RollupBackfillStatus{}, err
	}
	status := RollupBackfillStatus{Initialized: true, Completed: completed}
	if next.Valid && next.InfinityModifier == pgtype.Finite {
		t := next.Time.UTC()
		status.NextStart = &t
	}
	if upper.Valid && upper.InfinityModifier == pgtype.Finite {
		t := upper.Time.UTC()
		status.UpperBound = &t
	}
	if updated.Valid && updated.InfinityModifier == pgtype.Finite {
		t := updated.Time.UTC()
		status.UpdatedAt = &t
	}
	return status, nil
}

func initializeRollupBackfillTx(ctx context.Context, tx pgx.Tx) (RollupBackfillStatus, error) {
	if err := ensurePostgresRollupBackfillTable(ctx, tx); err != nil {
		return RollupBackfillStatus{}, err
	}
	status, err := scanRollupBackfillStatus(tx.QueryRow(ctx, `
SELECT next_start, upper_bound, completed, updated_at
FROM observation_rollup_backfill_state
WHERE id = 1
FOR UPDATE`))
	if err == nil {
		return status, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return RollupBackfillStatus{}, err
	}

	var minObserved, maxObserved pgtype.Timestamptz
	if err := tx.QueryRow(ctx, `SELECT MIN(observed_at), MAX(observed_at) FROM observations`).Scan(&minObserved, &maxObserved); err != nil {
		return RollupBackfillStatus{}, err
	}
	if !minObserved.Valid || !maxObserved.Valid {
		if _, err := tx.Exec(ctx, `
INSERT INTO observation_rollup_backfill_state(id, completed, updated_at)
VALUES (1, true, now())`); err != nil {
			return RollupBackfillStatus{}, err
		}
		return scanRollupBackfillStatus(tx.QueryRow(ctx, `
SELECT next_start, upper_bound, completed, updated_at
FROM observation_rollup_backfill_state WHERE id = 1`))
	}

	start := dateBucket(minObserved.Time, "day")
	upper := nextBucket(maxObserved.Time, "day")
	if _, err := tx.Exec(ctx, `
INSERT INTO observation_rollup_backfill_state(id, next_start, upper_bound, completed, updated_at)
VALUES (1, $1, $2, false, now())`, start, upper); err != nil {
		return RollupBackfillStatus{}, err
	}
	return scanRollupBackfillStatus(tx.QueryRow(ctx, `
SELECT next_start, upper_bound, completed, updated_at
FROM observation_rollup_backfill_state WHERE id = 1`))
}

func (s *PostgresStore) ObservationRollupBackfillStatus() (RollupBackfillStatus, error) {
	if s == nil || s.pool == nil {
		return RollupBackfillStatus{}, errors.New("postgres store is not open")
	}
	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RollupBackfillStatus{}, err
	}
	defer tx.Rollback(ctx)
	if err := ensurePostgresRollupBackfillTable(ctx, tx); err != nil {
		return RollupBackfillStatus{}, err
	}
	status, err := scanRollupBackfillStatus(tx.QueryRow(ctx, `
SELECT next_start, upper_bound, completed, updated_at
FROM observation_rollup_backfill_state WHERE id = 1`))
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return RollupBackfillStatus{}, err
		}
		return RollupBackfillStatus{}, nil
	}
	if err != nil {
		return RollupBackfillStatus{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RollupBackfillStatus{}, err
	}
	return status, nil
}

func (s *PostgresStore) RunObservationRollupBackfillBatch(span time.Duration) (RollupBackfillStatus, error) {
	if s == nil || s.pool == nil {
		return RollupBackfillStatus{}, errors.New("postgres store is not open")
	}
	if span <= 0 || span > maxRollupBackfillSpan {
		return RollupBackfillStatus{}, errors.New("invalid rollup backfill span")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RollupBackfillStatus{}, err
	}
	defer tx.Rollback(ctx)

	status, err := initializeRollupBackfillTx(ctx, tx)
	if err != nil {
		return RollupBackfillStatus{}, err
	}
	if status.Completed || status.NextStart == nil || status.UpperBound == nil {
		if err := tx.Commit(ctx); err != nil {
			return RollupBackfillStatus{}, err
		}
		return status, nil
	}

	start := status.NextStart.UTC()
	end := start.Add(span)
	if end.After(status.UpperBound.UTC()) {
		end = status.UpperBound.UTC()
	}
	if err := rebuildPostgresRollupsTx(ctx, tx, start, end); err != nil {
		return RollupBackfillStatus{}, err
	}
	completed := !end.Before(status.UpperBound.UTC())
	if _, err := tx.Exec(ctx, `
UPDATE observation_rollup_backfill_state
SET next_start = $1, completed = $2, updated_at = now()
WHERE id = 1`, end, completed); err != nil {
		return RollupBackfillStatus{}, err
	}
	status, err = scanRollupBackfillStatus(tx.QueryRow(ctx, `
SELECT next_start, upper_bound, completed, updated_at
FROM observation_rollup_backfill_state WHERE id = 1`))
	if err != nil {
		return RollupBackfillStatus{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RollupBackfillStatus{}, err
	}
	return status, nil
}
