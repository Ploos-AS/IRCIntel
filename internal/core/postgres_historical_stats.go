package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *PostgresStore) HistoricalObservationStats(query HistoricalStatsQuery) (HistoricalStatsSeries, error) {
	if s == nil || s.pool == nil { return HistoricalStatsSeries{}, errors.New("postgres store is not open") }
	if query.Now.IsZero() { query.Now = time.Now().UTC() }
	granularity, since, err := historicalWindow(query.Period, query.Now)
	if err != nil { return HistoricalStatsSeries{}, err }
	table := "observation_rollups_hourly"
	if granularity == "day" { table = "observation_rollups_daily" }

	where := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if query.Host != "" {
		args = append(args, strings.ToLower(strings.TrimSpace(query.Host)))
		where = append(where, fmt.Sprintf("endpoint_host = $%d", len(args)))
	}
	if !since.IsZero() {
		args = append(args, since.UTC())
		where = append(where, fmt.Sprintf("bucket_start >= $%d", len(args)))
	}
	args = append(args, query.Now.UTC())
	where = append(where, fmt.Sprintf("bucket_start <= $%d", len(args)))

	statement := `SELECT bucket_start,
       SUM(observation_count), SUM(reachable_count), SUM(dual_stack_count)
FROM ` + table + ` WHERE ` + strings.Join(where, " AND ") + `
GROUP BY bucket_start
ORDER BY bucket_start ASC`
	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	rows, err := s.pool.Query(ctx, statement, args...)
	if err != nil { return HistoricalStatsSeries{}, err }
	defer rows.Close()
	points := make([]HistoricalStatsPoint, 0)
	for rows.Next() {
		var point HistoricalStatsPoint
		if err := rows.Scan(&point.BucketStart, &point.ObservationCount, &point.ReachableCount, &point.DualStackCount); err != nil { return HistoricalStatsSeries{}, err }
		if point.ObservationCount > 0 {
			point.ReachablePercent = float64(point.ReachableCount) * 100 / float64(point.ObservationCount)
			point.DualStackPercent = float64(point.DualStackCount) * 100 / float64(point.ObservationCount)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil { return HistoricalStatsSeries{}, err }

	series := HistoricalStatsSeries{
		Period: query.Period,
		Granularity: granularity,
		Until: query.Now.UTC(),
		Host: strings.ToLower(strings.TrimSpace(query.Host)),
		Points: points,
	}
	if !since.IsZero() { v := since.UTC(); series.Since = &v }
	return series, nil
}

var _ HistoricalStatsReader = (*PostgresStore)(nil)
