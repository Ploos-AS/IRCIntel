package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (s *PostgresStore) HistoricalNetworkRanking(query NetworkHistoryRankingQuery) (NetworkHistoryRanking, error) {
	if s == nil || s.pool == nil { return NetworkHistoryRanking{}, errors.New("postgres store is not open") }
	if query.Now.IsZero() { query.Now = time.Now().UTC() }
	if query.Limit == 0 { query.Limit = 100 }
	if query.Limit < 1 || query.Limit > 500 { return NetworkHistoryRanking{}, errors.New("invalid ranking limit") }
	granularity, since, err := historicalWindow(query.Period, query.Now)
	if err != nil { return NetworkHistoryRanking{}, err }
	table := "observation_rollups_hourly"
	bucketWidth := "1 hour"
	if granularity == "day" { table = "observation_rollups_daily"; bucketWidth = "1 day" }

	args := []any{query.Now.UTC(), query.Limit}
	currentPredicate := "a.bucket_start <= $1"
	previousCTE := "SELECT NULL::text AS network_id, 0::bigint AS observation_count, 0::bigint AS reachable_count WHERE false"
	previousJoin := "LEFT JOIN previous p ON p.network_id = c.network_id"
	if !since.IsZero() {
		args = append(args, since.UTC())
		currentPredicate += fmt.Sprintf(" AND a.bucket_start >= $%d", len(args))
		window := query.Now.UTC().Sub(since.UTC())
		previousSince := since.UTC().Add(-window)
		args = append(args, previousSince)
		previousCTE = fmt.Sprintf(`SELECT a.network_id,
       SUM(a.observation_count) AS observation_count,
       SUM(a.reachable_count) AS reachable_count
FROM attributed_rollups a
WHERE a.owner_count = 1
  AND a.bucket_start >= $%d
  AND a.bucket_start < $%d
GROUP BY a.network_id`, len(args), len(args)-1)
	} else {
		previousJoin = "LEFT JOIN previous p ON false"
	}

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
    GROUP BY r.bucket_start, r.endpoint_host, r.endpoint_port, r.endpoint_tls,
             r.observation_count, r.reachable_count, r.dual_stack_count
), current_window AS (
    SELECT a.network_id,
           SUM(a.observation_count) AS observation_count,
           SUM(a.reachable_count) AS reachable_count,
           SUM(a.dual_stack_count) AS dual_stack_count
    FROM attributed_rollups a
    WHERE a.owner_count = 1 AND %s
    GROUP BY a.network_id
), previous AS (
    %s
)
SELECT n.id, n.name,
       c.observation_count, c.reachable_count, c.dual_stack_count,
       COALESCE(p.observation_count, 0), COALESCE(p.reachable_count, 0)
FROM current_window c
JOIN networks n ON n.id = c.network_id
%s
WHERE c.observation_count > 0
ORDER BY (c.reachable_count::double precision / c.observation_count) DESC,
         c.observation_count DESC,
         lower(n.name), n.id
LIMIT $2`, table, bucketWidth, currentPredicate, previousCTE, previousJoin)

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil { return NetworkHistoryRanking{}, err }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { return NetworkHistoryRanking{}, err }
	rows, err := tx.Query(ctx, statement, args...)
	if err != nil { return NetworkHistoryRanking{}, err }
	defer rows.Close()

	items := make([]NetworkHistoryRankingItem, 0)
	for rows.Next() {
		var item NetworkHistoryRankingItem
		var previousReachable int64
		if err := rows.Scan(&item.NetworkID, &item.NetworkName, &item.ObservationCount, &item.ReachableCount, &item.DualStackCount, &item.PreviousObservationCount, &previousReachable); err != nil { return NetworkHistoryRanking{}, err }
		if item.ObservationCount > 0 {
			item.ReachablePercent = float64(item.ReachableCount) * 100 / float64(item.ObservationCount)
			item.DualStackPercent = float64(item.DualStackCount) * 100 / float64(item.ObservationCount)
		}
		if item.PreviousObservationCount > 0 {
			previous := float64(previousReachable) * 100 / float64(item.PreviousObservationCount)
			trend := item.ReachablePercent - previous
			item.PreviousReachablePercent = &previous
			item.ReachableTrendPoints = &trend
		}
		item.Rank = len(items) + 1
		items = append(items, item)
	}
	if err := rows.Err(); err != nil { return NetworkHistoryRanking{}, err }
	if err := tx.Commit(ctx); err != nil { return NetworkHistoryRanking{}, err }

	ranking := NetworkHistoryRanking{Period: query.Period, Granularity: granularity, Until: query.Now.UTC(), Items: items}
	if !since.IsZero() { v := since.UTC(); ranking.Since = &v }
	return ranking, nil
}

var _ NetworkHistoryRankingReader = (*PostgresStore)(nil)
