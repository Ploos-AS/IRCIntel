package core

import (
	"context"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestPostgresObservationRollupsAndRetention(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, `TRUNCATE observation_rollups_hourly, observation_rollups_daily, observations RESTART IDENTITY`); err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 9, 1, 10, 15, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "a", ObservedAt: base, Endpoint: agent.Endpoint{Host: "irc.rollup.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true, DualStackOK: true}},
		{AgentID: "b", ObservedAt: base.Add(20 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.rollup.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "a", ObservedAt: base.Add(70 * time.Minute), Endpoint: agent.Endpoint{Host: "irc.rollup.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
		{AgentID: "a", ObservedAt: base.Add(25 * time.Hour), Endpoint: agent.Endpoint{Host: "irc.rollup.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true, DualStackOK: true}},
	}
	for _, item := range items {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}
	// Duplicate ingest must not increment rollups.
	if err := store.Store(items[0]); err != nil { t.Fatal(err) }

	hourly, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "hour", Host: "irc.rollup.example", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(hourly) != 3 { t.Fatalf("hourly len=%d want=3", len(hourly)) }
	var total, reachable, dual int64
	for _, item := range hourly { total += item.ObservationCount; reachable += item.ReachableCount; dual += item.DualStackCount }
	if total != 4 || reachable != 3 || dual != 2 { t.Fatalf("hourly totals=%d reachable=%d dual=%d", total, reachable, dual) }

	daily, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "day", Host: "irc.rollup.example", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(daily) != 2 { t.Fatalf("daily len=%d want=2", len(daily)) }

	// Rebuild must be idempotent and preserve the same totals.
	if err := store.RebuildObservationRollups(time.Time{}, time.Time{}); err != nil { t.Fatal(err) }
	hourly, err = store.ListObservationRollups(ObservationRollupQuery{Granularity: "hour", Host: "irc.rollup.example", Limit: 10})
	if err != nil { t.Fatal(err) }
	total = 0
	for _, item := range hourly { total += item.ObservationCount }
	if total != 4 { t.Fatalf("rebuilt total=%d want=4", total) }

	cutoff := base.Add(24 * time.Hour)
	deleted, err := store.PruneObservationsBefore(cutoff)
	if err != nil { t.Fatal(err) }
	if deleted != 3 { t.Fatalf("deleted=%d want=3", deleted) }
	count, err := store.Count(); if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("raw count=%d want=1", count) }

	// Historical buckets older than the raw-retention cutoff must remain queryable.
	hourly, err = store.ListObservationRollups(ObservationRollupQuery{Granularity: "hour", Host: "irc.rollup.example", Until: cutoff, Limit: 10})
	if err != nil { t.Fatal(err) }
	var retained int64
	for _, item := range hourly { retained += item.ObservationCount }
	if retained != 3 { t.Fatalf("retained rollup count=%d want=3", retained) }
}

func TestCompleteBucketRangeExpandsNonAlignedBounds(t *testing.T) {
	since := time.Date(2026, 9, 11, 10, 17, 0, 0, time.FixedZone("offset", 2*60*60))
	until := time.Date(2026, 9, 11, 12, 43, 0, 0, time.FixedZone("offset", 2*60*60))

	hourStart, hourEnd := completeBucketRange(since, until, "hour")
	if want := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC); !hourStart.Equal(want) { t.Fatalf("hour start=%s want=%s", hourStart, want) }
	if want := time.Date(2026, 9, 11, 11, 0, 0, 0, time.UTC); !hourEnd.Equal(want) { t.Fatalf("hour end=%s want=%s", hourEnd, want) }

	dayStart, dayEnd := completeBucketRange(since, until, "day")
	if want := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC); !dayStart.Equal(want) { t.Fatalf("day start=%s want=%s", dayStart, want) }
	if want := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC); !dayEnd.Equal(want) { t.Fatalf("day end=%s want=%s", dayEnd, want) }
}

func TestPostgresRollupRebuildKeepsCompleteBoundaryBucket(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, `TRUNCATE observation_rollups_hourly, observation_rollups_daily, observations RESTART IDENTITY`); err != nil { t.Fatal(err) }

	bucket := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	items := []agent.Observation{
		{AgentID: "before", ObservedAt: bucket.Add(10 * time.Minute), Endpoint: agent.Endpoint{Host: "boundary.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}},
		{AgentID: "after", ObservedAt: bucket.Add(50 * time.Minute), Endpoint: agent.Endpoint{Host: "boundary.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: false}},
	}
	for _, item := range items { if err := store.Store(item); err != nil { t.Fatal(err) } }

	// Simulate a damaged rollup, then rebuild only up to a non-aligned cutoff
	// inside the same hour. The whole 10:00 UTC bucket must be reconstructed.
	if _, err := store.pool.Exec(ctx, `UPDATE observation_rollups_hourly SET observation_count=99 WHERE endpoint_host='boundary.example'`); err != nil { t.Fatal(err) }
	cutoff := bucket.Add(30 * time.Minute)
	if err := store.RebuildObservationRollups(time.Time{}, cutoff); err != nil { t.Fatal(err) }

	rollups, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "hour", Host: "boundary.example", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(rollups) != 1 { t.Fatalf("rollups=%d want=1", len(rollups)) }
	if rollups[0].ObservationCount != 2 || rollups[0].ReachableCount != 1 { t.Fatalf("boundary rollup=%+v", rollups[0]) }

	deleted, err := store.PruneObservationsBefore(cutoff)
	if err != nil { t.Fatal(err) }
	if deleted != 1 { t.Fatalf("deleted=%d want=1", deleted) }
	rollups, err = store.ListObservationRollups(ObservationRollupQuery{Granularity: "hour", Host: "boundary.example", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(rollups) != 1 || rollups[0].ObservationCount != 2 { t.Fatalf("retained boundary rollup=%+v", rollups) }
}

func TestPostgresRollupBucketingIgnoresSessionTimezone(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, `TRUNCATE observation_rollups_hourly, observation_rollups_daily, observations RESTART IDENTITY`); err != nil { t.Fatal(err) }

	tx, err := store.pool.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'Pacific/Honolulu'`); err != nil { t.Fatal(err) }
	observedAt := time.Date(2026, 9, 11, 0, 30, 0, 0, time.UTC)
	if err := refreshPostgresRollupsForObservationTx(ctx, tx, observedAt, "utc.example", "6697", true, true, false); err != nil { t.Fatal(err) }
	var bucketStart time.Time
	if err := tx.QueryRow(ctx, `SELECT bucket_start FROM observation_rollups_hourly WHERE endpoint_host='utc.example'`).Scan(&bucketStart); err != nil { t.Fatal(err) }
	want := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	if !bucketStart.UTC().Equal(want) { t.Fatalf("bucket=%s want=%s", bucketStart.UTC(), want) }
}

func TestPostgresRollupQueryValidation(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "week", Limit: 10}); err == nil { t.Fatal("expected granularity error") }
	if _, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "hour", Limit: 0}); err == nil { t.Fatal("expected limit error") }
	now := time.Now().UTC()
	if _, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "day", Since: now, Until: now.Add(-time.Hour), Limit: 10}); err == nil { t.Fatal("expected range error") }
	if _, err := store.PruneObservationsBefore(time.Time{}); err == nil { t.Fatal("expected cutoff error") }
}
