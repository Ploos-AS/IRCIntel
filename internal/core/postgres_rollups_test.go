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

func TestPostgresRollupQueryValidation(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "week", Limit: 10}); err == nil { t.Fatal("expected granularity error") }
	if _, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "hour", Limit: 0}); err == nil { t.Fatal("expected limit error") }
	now := time.Now().UTC()
	if _, err := store.ListObservationRollups(ObservationRollupQuery{Granularity: "day", Since: now, Until: now.Add(-time.Hour), Limit: 10}); err == nil { t.Fatal("expected range error") }
	if _, err := store.PruneObservationsBefore(time.Time{}); err == nil { t.Fatal("expected cutoff error") }
}
