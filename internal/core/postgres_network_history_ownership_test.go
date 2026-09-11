package core

import (
	"context"
	"testing"
	"time"
)

func TestPostgresHistoricalNetworkStatsFollowOwnershipIntervals(t *testing.T) {
	store := openTestPostgresStore(t)
	resetPostgresOwnershipTestState(t, store)
	ctx := context.Background()

	if err := store.UpsertNetwork(Network{ID: "net-old", Name: "Old Network"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetwork(Network{ID: "net-new", Name: "New Network"}); err != nil { t.Fatal(err) }

	tx, err := store.pool.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { t.Fatal(err) }
	if _, err := tx.Exec(ctx, `DELETE FROM endpoint_network_ownership; DELETE FROM endpoint_network_ownership_meta;`); err != nil { t.Fatal(err) }
	if _, err := tx.Exec(ctx, `INSERT INTO endpoint_network_ownership_meta(singleton, bootstrapped_at) VALUES (TRUE, now())`); err != nil { t.Fatal(err) }

	moveAt := time.Date(2026, 9, 11, 3, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `
INSERT INTO endpoint_network_ownership(endpoint_id, server_id, network_id, host, port, tls, valid_from, valid_to)
VALUES
 ('endpoint-a','server-a','net-old','irc.example','6697',true,'-infinity'::timestamptz,$1),
 ('endpoint-a','server-a','net-new','irc.example','6697',true,$1,NULL)`, moveAt); err != nil { t.Fatal(err) }
	if _, err := tx.Exec(ctx, `
INSERT INTO observation_rollups_hourly(bucket_start, endpoint_host, endpoint_port, endpoint_tls, observation_count, reachable_count, dual_stack_count)
VALUES
 ('2026-09-11T01:00:00Z','irc.example','6697',true,10,10,8),
 ('2026-09-11T04:00:00Z','irc.example','6697',true,20,10,20)`); err != nil { t.Fatal(err) }
	if err := tx.Commit(ctx); err != nil { t.Fatal(err) }

	now := time.Date(2026, 9, 11, 6, 0, 0, 0, time.UTC)
	oldSeries, err := store.HistoricalNetworkStats(NetworkHistoricalStatsQuery{NetworkID: "net-old", Period: "24h", Now: now})
	if err != nil { t.Fatal(err) }
	if len(oldSeries.Points) != 1 { t.Fatalf("old points=%d: %+v", len(oldSeries.Points), oldSeries.Points) }
	if oldSeries.Points[0].ObservationCount != 10 || oldSeries.Points[0].ReachableCount != 10 { t.Fatalf("old point=%+v", oldSeries.Points[0]) }

	newSeries, err := store.HistoricalNetworkStats(NetworkHistoricalStatsQuery{NetworkID: "net-new", Period: "24h", Now: now})
	if err != nil { t.Fatal(err) }
	if len(newSeries.Points) != 1 { t.Fatalf("new points=%d: %+v", len(newSeries.Points), newSeries.Points) }
	if newSeries.Points[0].ObservationCount != 20 || newSeries.Points[0].ReachableCount != 10 { t.Fatalf("new point=%+v", newSeries.Points[0]) }

	ranking, err := store.HistoricalNetworkRanking(NetworkHistoryRankingQuery{Period: "24h", Now: now, Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(ranking.Items) != 2 { t.Fatalf("ranking items=%d: %+v", len(ranking.Items), ranking.Items) }
	if ranking.Items[0].NetworkID != "net-old" || ranking.Items[0].ObservationCount != 10 || ranking.Items[0].ReachablePercent != 100 { t.Fatalf("rank 1=%+v", ranking.Items[0]) }
	if ranking.Items[1].NetworkID != "net-new" || ranking.Items[1].ObservationCount != 20 || ranking.Items[1].ReachablePercent != 50 { t.Fatalf("rank 2=%+v", ranking.Items[1]) }
}

func TestPostgresHistoricalNetworkStatsExcludeOwnershipBoundaryBucket(t *testing.T) {
	store := openTestPostgresStore(t)
	resetPostgresOwnershipTestState(t, store)
	ctx := context.Background()
	if err := store.UpsertNetwork(Network{ID: "net-a", Name: "Network A"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetwork(Network{ID: "net-b", Name: "Network B"}); err != nil { t.Fatal(err) }

	tx, err := store.pool.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { t.Fatal(err) }
	if _, err := tx.Exec(ctx, `DELETE FROM endpoint_network_ownership; DELETE FROM endpoint_network_ownership_meta; INSERT INTO endpoint_network_ownership_meta(singleton, bootstrapped_at) VALUES (TRUE, now())`); err != nil { t.Fatal(err) }
	moveAt := time.Date(2026, 9, 11, 3, 30, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `
INSERT INTO endpoint_network_ownership(endpoint_id, server_id, network_id, host, port, tls, valid_from, valid_to)
VALUES
 ('endpoint-a','server-a','net-a','boundary.example','6697',true,'-infinity'::timestamptz,$1),
 ('endpoint-a','server-a','net-b','boundary.example','6697',true,$1,NULL);
INSERT INTO observation_rollups_hourly(bucket_start, endpoint_host, endpoint_port, endpoint_tls, observation_count, reachable_count, dual_stack_count)
VALUES ('2026-09-11T03:00:00Z','boundary.example','6697',true,10,10,10)`, moveAt); err != nil { t.Fatal(err) }
	if err := tx.Commit(ctx); err != nil { t.Fatal(err) }

	now := time.Date(2026, 9, 11, 6, 0, 0, 0, time.UTC)
	for _, id := range []string{"net-a", "net-b"} {
		series, err := store.HistoricalNetworkStats(NetworkHistoricalStatsQuery{NetworkID: id, Period: "24h", Now: now})
		if err != nil { t.Fatal(err) }
		if len(series.Points) != 0 { t.Fatalf("%s boundary bucket attributed: %+v", id, series.Points) }
	}
}
