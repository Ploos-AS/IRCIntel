package core

import (
	"context"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestPostgresRetentionMaintenancePreservesLongTermDailyHistory(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.pool.Exec(context.Background(), `TRUNCATE discovery_promotions, discovery_reviews, discovery_candidates, network_incident_records, network_endpoints, network_servers, networks, observation_rollups_hourly, observation_rollups_daily, endpoint_transition_events, agent_endpoint_state, incident_records, observations RESTART IDENTITY CASCADE`); err != nil { t.Fatal(err) }

	now := time.Date(2026, 9, 11, 4, 0, 0, 0, time.UTC)
	for i, age := range []time.Duration{2 * time.Hour, 40 * 24 * time.Hour, 500 * 24 * time.Hour} {
		o := agent.Observation{AgentID: "maintenance-agent", ObservedAt: now.Add(-age), Endpoint: agent.Endpoint{Host: "irc.maintenance.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: i != 1, DualStackOK: i == 0}}
		if err := store.Store(o); err != nil { t.Fatal(err) }
	}

	result, err := store.RunRetentionMaintenance(now, DefaultRetentionPolicy())
	if err != nil { t.Fatal(err) }
	if result.RawObservationsPruned != 2 { t.Fatalf("raw pruned=%d", result.RawObservationsPruned) }
	if result.HourlyRollupsPruned != 1 { t.Fatalf("hourly pruned=%d", result.HourlyRollupsPruned) }

	var rawCount, hourlyCount, dailyCount int
	if err := store.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM observations`).Scan(&rawCount); err != nil { t.Fatal(err) }
	if err := store.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM observation_rollups_hourly`).Scan(&hourlyCount); err != nil { t.Fatal(err) }
	if err := store.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM observation_rollups_daily`).Scan(&dailyCount); err != nil { t.Fatal(err) }
	if rawCount != 1 { t.Fatalf("raw count=%d", rawCount) }
	if hourlyCount != 2 { t.Fatalf("hourly count=%d", hourlyCount) }
	if dailyCount != 3 { t.Fatalf("daily count=%d", dailyCount) }

	again, err := store.RunRetentionMaintenance(now, DefaultRetentionPolicy())
	if err != nil { t.Fatal(err) }
	if again.RawObservationsPruned != 0 || again.HourlyRollupsPruned != 0 { t.Fatalf("second pass=%+v", again) }
}

func TestRetentionPolicyValidation(t *testing.T) {
	bad := []RetentionPolicy{
		{},
		{RawObservations: time.Hour, HourlyRollups: 0},
		{RawObservations: 48 * time.Hour, HourlyRollups: 24 * time.Hour},
	}
	for _, policy := range bad {
		if err := policy.validate(); err == nil { t.Fatalf("expected invalid policy: %+v", policy) }
	}
}
