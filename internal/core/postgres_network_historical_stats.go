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
	var networkName string
	if err := s.pool.QueryRow(ctx, `SELECT name FROM networks WHERE id = $1`, query.NetworkID).Scan(&networkName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return NetworkHistoricalStatsSeries{}, ErrRegistryNetworkNotFound }
		return NetworkHistoricalStatsSeries{}, err
	}

	table := "observation_rollups_hourly"
	if granularity == "day" { table = "observation_rollups_daily" }
	args := []any{query.NetworkID}
	where := make([]string, 0, 2)
	if !since.IsZero() {
		args = append(args, since.UTC())
		where = append(where, fmt.Sprintf("r.bucket_start >= $%d", len(args)))
	}
	args = append(args, query.Now.UTC())
	where = append(where, fmt.Sprintf("r.bucket_start <= $%d", len(args)))

	statement := `WITH endpoint_owners AS (
    SELECT e.host, e.port, e.tls,
           MIN(s.network_id) AS network_id,
           COUNT(DISTINCT s.network_id) AS owner_count
    FROM network_endpoints e
    JOIN network_servers s ON s.id = e.server_id
    GROUP BY e.host, e.port, e.tls
), network_endpoints_unambiguous AS (
    SELECT host, port, tls
    FROM endpoint_owners
    WHERE owner_count = 1 AND network_id = $1
)
SELECT r.bucket_start,
       SUM(r.observation_count), SUM(r.reachable_count), SUM(r.dual_stack_count)
FROM ` + table + ` r
JOIN network_endpoints_unambiguous e
  ON e.host = r.endpoint_host AND e.port = r.endpoint_port AND e.tls = r.endpoint_tls
WHERE ` + strings.Join(where, " AND ") + `
GROUP BY r.bucket_start
ORDER BY r.bucket_start ASC`

	rows, err := s.pool.Query(ctx, statement, args...)
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
