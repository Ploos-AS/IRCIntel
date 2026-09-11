package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ObservationRollup struct {
	BucketStart      time.Time `json:"bucket_start"`
	Host             string    `json:"host"`
	Port             string    `json:"port"`
	TLS              bool      `json:"tls"`
	ObservationCount int64     `json:"observation_count"`
	ReachableCount   int64     `json:"reachable_count"`
	DualStackCount   int64     `json:"dual_stack_count"`
	FirstObservedAt  time.Time `json:"first_observed_at"`
	LastObservedAt   time.Time `json:"last_observed_at"`
}

type ObservationRollupQuery struct {
	Granularity string
	Host        string
	Since       time.Time
	Until       time.Time
	Limit       int
}

func migratePostgresRollups(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS observation_rollups_hourly (
    bucket_start TIMESTAMPTZ NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL,
    endpoint_tls BOOLEAN NOT NULL,
    observation_count BIGINT NOT NULL,
    reachable_count BIGINT NOT NULL,
    dual_stack_count BIGINT NOT NULL,
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(bucket_start, endpoint_host, endpoint_port, endpoint_tls)
);
CREATE INDEX IF NOT EXISTS idx_pg_rollups_hourly_endpoint_time
    ON observation_rollups_hourly(endpoint_host, endpoint_port, endpoint_tls, bucket_start DESC);
CREATE TABLE IF NOT EXISTS observation_rollups_daily (
    bucket_start TIMESTAMPTZ NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL,
    endpoint_tls BOOLEAN NOT NULL,
    observation_count BIGINT NOT NULL,
    reachable_count BIGINT NOT NULL,
    dual_stack_count BIGINT NOT NULL,
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(bucket_start, endpoint_host, endpoint_port, endpoint_tls)
);
CREATE INDEX IF NOT EXISTS idx_pg_rollups_daily_endpoint_time
    ON observation_rollups_daily(endpoint_host, endpoint_port, endpoint_tls, bucket_start DESC);`); err != nil {
		return err
	}
	return rebuildPostgresRollupsTx(ctx, tx, time.Time{}, time.Time{})
}

func refreshPostgresRollupsForObservationTx(ctx context.Context, tx pgx.Tx, observedAt time.Time, host, port string, tls, reachable, dualStack bool) error {
	for _, spec := range []struct{ table, trunc string }{{"observation_rollups_hourly", "hour"}, {"observation_rollups_daily", "day"}} {
		statement := fmt.Sprintf(`
INSERT INTO %s (
    bucket_start, endpoint_host, endpoint_port, endpoint_tls,
    observation_count, reachable_count, dual_stack_count,
    first_observed_at, last_observed_at
)
VALUES (date_trunc('%s', $1::timestamptz, 'UTC'), $2, $3, $4, 1, $5, $6, $1, $1)
ON CONFLICT (bucket_start, endpoint_host, endpoint_port, endpoint_tls) DO UPDATE SET
    observation_count = %s.observation_count + 1,
    reachable_count = %s.reachable_count + excluded.reachable_count,
    dual_stack_count = %s.dual_stack_count + excluded.dual_stack_count,
    first_observed_at = LEAST(%s.first_observed_at, excluded.first_observed_at),
    last_observed_at = GREATEST(%s.last_observed_at, excluded.last_observed_at)`, spec.table, spec.trunc, spec.table, spec.table, spec.table, spec.table, spec.table)
		if _, err := tx.Exec(ctx, statement, observedAt.UTC(), host, port, tls, boolToInt64(reachable), boolToInt64(dualStack)); err != nil {
			return err
		}
	}
	return nil
}

func boolToInt64(v bool) int64 { if v { return 1 }; return 0 }

// rebuildPostgresRollupsTx always rebuilds complete UTC buckets. Callers may
// pass arbitrary timestamps; each granularity expands them to the containing
// bucket boundaries before deleting/recomputing data. This prevents a partial
// rebuild from replacing a complete rollup with only one side of a retention
// or repair cutoff.
func rebuildPostgresRollupsTx(ctx context.Context, tx pgx.Tx, since, until time.Time) error {
	for _, spec := range []struct{ table, trunc string }{{"observation_rollups_hourly", "hour"}, {"observation_rollups_daily", "day"}} {
		bucketSince, bucketUntil := completeBucketRange(since, until, spec.trunc)

		where := make([]string, 0, 2)
		args := make([]any, 0, 2)
		if !bucketSince.IsZero() {
			args = append(args, bucketSince)
			where = append(where, fmt.Sprintf("observed_at >= $%d", len(args)))
		}
		if !bucketUntil.IsZero() {
			args = append(args, bucketUntil)
			where = append(where, fmt.Sprintf("observed_at < $%d", len(args)))
		}
		filter := ""
		if len(where) > 0 { filter = " WHERE " + strings.Join(where, " AND ") }

		deleteSQL := "DELETE FROM " + spec.table
		deleteArgs := make([]any, 0, 2)
		deleteWhere := make([]string, 0, 2)
		if !bucketSince.IsZero() {
			deleteArgs = append(deleteArgs, bucketSince)
			deleteWhere = append(deleteWhere, fmt.Sprintf("bucket_start >= $%d", len(deleteArgs)))
		}
		if !bucketUntil.IsZero() {
			deleteArgs = append(deleteArgs, bucketUntil)
			deleteWhere = append(deleteWhere, fmt.Sprintf("bucket_start < $%d", len(deleteArgs)))
		}
		if len(deleteWhere) > 0 { deleteSQL += " WHERE " + strings.Join(deleteWhere, " AND ") }
		if _, err := tx.Exec(ctx, deleteSQL, deleteArgs...); err != nil { return err }

		insertSQL := fmt.Sprintf(`
INSERT INTO %s (
    bucket_start, endpoint_host, endpoint_port, endpoint_tls,
    observation_count, reachable_count, dual_stack_count,
    first_observed_at, last_observed_at
)
SELECT date_trunc('%s', observed_at, 'UTC'), endpoint_host, endpoint_port, endpoint_tls,
       COUNT(*),
       COUNT(*) FILTER (WHERE COALESCE((payload_json #>> '{result,reachable}')::boolean, false)),
       COUNT(*) FILTER (WHERE COALESCE((payload_json #>> '{result,dual_stack_ok}')::boolean, false)),
       MIN(observed_at), MAX(observed_at)
FROM observations%s
GROUP BY 1, 2, 3, 4`, spec.table, spec.trunc, filter)
		if _, err := tx.Exec(ctx, insertSQL, args...); err != nil { return err }
	}
	return nil
}

func dateBucket(t time.Time, granularity string) time.Time {
	u := t.UTC()
	if granularity == "day" { return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC) }
	return u.Truncate(time.Hour)
}

func nextBucket(t time.Time, granularity string) time.Time {
	start := dateBucket(t, granularity)
	if granularity == "day" { return start.AddDate(0, 0, 1) }
	return start.Add(time.Hour)
}

func completeBucketRange(since, until time.Time, granularity string) (time.Time, time.Time) {
	var start, end time.Time
	if !since.IsZero() { start = dateBucket(since, granularity) }
	if !until.IsZero() {
		end = dateBucket(until, granularity)
		if !until.UTC().Equal(end) { end = nextBucket(until, granularity) }
	}
	return start, end
}

func (s *PostgresStore) RebuildObservationRollups(since, until time.Time) error {
	if s == nil || s.pool == nil { return errors.New("postgres store is not open") }
	if !since.IsZero() && !until.IsZero() && since.After(until) { return errors.New("invalid rollup time range") }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tx, err := s.pool.Begin(ctx); if err != nil { return err }; defer tx.Rollback(ctx)
	if err := rebuildPostgresRollupsTx(ctx, tx, since, until); err != nil { return err }
	return tx.Commit(ctx)
}

func (s *PostgresStore) PruneObservationsBefore(cutoff time.Time) (int64, error) {
	if s == nil || s.pool == nil { return 0, errors.New("postgres store is not open") }
	if cutoff.IsZero() { return 0, errors.New("retention cutoff is required") }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tx, err := s.pool.Begin(ctx); if err != nil { return 0, err }; defer tx.Rollback(ctx)
	if err := rebuildPostgresRollupsTx(ctx, tx, time.Time{}, cutoff); err != nil { return 0, err }
	result, err := tx.Exec(ctx, `DELETE FROM observations WHERE observed_at < $1`, cutoff.UTC()); if err != nil { return 0, err }
	if err := tx.Commit(ctx); err != nil { return 0, err }
	return result.RowsAffected(), nil
}

func (s *PostgresStore) ListObservationRollups(query ObservationRollupQuery) ([]ObservationRollup, error) {
	if s == nil || s.pool == nil { return nil, errors.New("postgres store is not open") }
	if query.Granularity != "hour" && query.Granularity != "day" { return nil, errors.New("invalid rollup granularity") }
	if query.Limit < 1 || query.Limit > 5000 { return nil, errors.New("invalid rollup limit") }
	if !query.Since.IsZero() && !query.Until.IsZero() && query.Since.After(query.Until) { return nil, errors.New("invalid rollup time range") }
	table := "observation_rollups_hourly"; if query.Granularity == "day" { table = "observation_rollups_daily" }
	where := make([]string, 0, 3); args := make([]any, 0, 4)
	if query.Host != "" { args = append(args, query.Host); where = append(where, fmt.Sprintf("endpoint_host = $%d", len(args))) }
	if !query.Since.IsZero() { args = append(args, query.Since.UTC()); where = append(where, fmt.Sprintf("bucket_start >= $%d", len(args))) }
	if !query.Until.IsZero() { args = append(args, query.Until.UTC()); where = append(where, fmt.Sprintf("bucket_start <= $%d", len(args))) }
	statement := `SELECT bucket_start, endpoint_host, endpoint_port, endpoint_tls, observation_count, reachable_count, dual_stack_count, first_observed_at, last_observed_at FROM ` + table
	if len(where) > 0 { statement += " WHERE " + strings.Join(where, " AND ") }
	args = append(args, query.Limit); statement += fmt.Sprintf(" ORDER BY bucket_start DESC, endpoint_host, endpoint_port, endpoint_tls LIMIT $%d", len(args))
	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout); defer cancel()
	rows, err := s.pool.Query(ctx, statement, args...); if err != nil { return nil, err }; defer rows.Close()
	out := make([]ObservationRollup, 0)
	for rows.Next() { var item ObservationRollup; if err := rows.Scan(&item.BucketStart,&item.Host,&item.Port,&item.TLS,&item.ObservationCount,&item.ReachableCount,&item.DualStackCount,&item.FirstObservedAt,&item.LastObservedAt); err != nil { return nil, err }; out = append(out,item) }
	if err := rows.Err(); err != nil { return nil, err }
	return out, nil
}
