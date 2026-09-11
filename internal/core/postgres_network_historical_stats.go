package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) HistoricalNetworkStats(query NetworkHistoricalStatsQuery) (NetworkHistoricalStatsSeries, error) {
	if s == nil || s.pool == nil { return NetworkHistoricalStatsSeries{}, errors.New("postgres store is not open") }
	query.NetworkID = strings.TrimSpace(query.NetworkID)
	if query.NetworkID == "" { return NetworkHistoricalStatsSeries{}, errors.New("network id is required") }
	if query.Now.IsZero() { query.Now = time.Now().UTC() }
	granularity, since, err := historicalWindow(query.Period, query.Now)
	if err != nil { return NetworkHistoricalStatsSeries{}, err }

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil { return NetworkHistoricalStatsSeries{}, err }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { return NetworkHistoricalStatsSeries{}, err }

	var networkName string
	if err := tx.QueryRow(ctx, `SELECT name FROM networks WHERE id = $1`, query.NetworkID).Scan(&networkName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return NetworkHistoricalStatsSeries{}, ErrRegistryNetworkNotFound }
		return NetworkHistoricalStatsSeries{}, err
	}

	table := "observation_rollups_hourly"
	bucketWidth := "1 hour"
	if granularity == "day" { table = "observation_rollups_daily"; bucketWidth = "1 day" }
	args := []any{query.NetworkID}
	where := make([]string, 0, 2)
	if !since.IsZero() {
		args = append(args, since.UTC())
		where = append(where, fmt.Sprintf("r.bucket_start >= $%d", len(args)))
	}
	args = append(args, query.Now.UTC())
	where = append(where, fmt.Sprintf("r.bucket_start <= $%d", len(args)))

	statement := fmt.Sprintf(`WITH attributed_rollups AS (
    SELECT r.bucket_start, r.endpoint_host, r.endpoint_port, r.endpoint_tls,
           r.observation_count, r.reachable_count, r.dual_stack_count,
           MIN(o.network_id) AS network_id,
           COUNT(DISTINCT o.network_id) AS owner_count
    FROM %s r
    JOIN endpoint_network_ownership o
      ON o.host = r.endpoint_host
     AND o.port = r.endpoint_port
     AND o.tls = r.endpoint_tls
     AND o.valid_from <= r.bucket_start
     AND (o.valid_to IS NULL OR o.valid_to >= r.bucket_start + INTERVAL '%s')
    WHERE %s
    GROUP BY r.bucket_start, r.endpoint_host, r.endpoint_port, r.endpoint_tls,
             r.observation_count, r.reachable_count, r.dual_stack_count
)
SELECT bucket_start,
       SUM(observation_count), SUM(reachable_count), SUM(dual_stack_count)
FROM attributed_rollups
WHERE owner_count = 1 AND network_id = $1
GROUP BY bucket_start
ORDER BY bucket_start ASC`, table, bucketWidth, strings.Join(where, " AND "))

	rows, err := tx.Query(ctx, statement, args...)
	if err != nil { return NetworkHistoricalStatsSeries{}, err }
	defer rows.Close()
	points := make([]HistoricalStatsPoint, 0)
	for rows.Next() {
		var point HistoricalStatsPoint
		if err := rows.Scan(&point.BucketStart, &point.ObservationCount, &point.ReachableCount, &point.DualStackCount); err != nil { return NetworkHistoricalStatsSeries{}, err }
		if point.ObservationCount > 0 {
			point.ReachablePercent = float64(point.ReachableCount) * 100 / float64(point.ObservationCount)
			point.DualStackPercent = float64(point.DualStackCount) * 100 / float64(point.ObservationCount)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil { return NetworkHistoricalStatsSeries{}, err }
	if err := tx.Commit(ctx); err != nil { return NetworkHistoricalStatsSeries{}, err }

	series := NetworkHistoricalStatsSeries{
		NetworkID: query.NetworkID,
		NetworkName: networkName,
		Period: query.Period,
		Granularity: granularity,
		Until: query.Now.UTC(),
		Points: points,
	}
	if !since.IsZero() { v := since.UTC(); series.Since = &v }
	return series, nil
}

var _ NetworkHistoricalStatsReader = (*PostgresStore)(nil)
