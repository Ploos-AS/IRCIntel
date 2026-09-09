package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func openTestPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()
	databaseURL := os.Getenv("IRCINTEL_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("IRCINTEL_TEST_POSTGRES_URL not configured")
	}
	store, err := OpenPostgresStore(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(context.Background(), `
TRUNCATE endpoint_transition_events, agent_endpoint_state, incident_records, observations RESTART IDENTITY`); err != nil {
		store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPostgresStoreFoundation(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("postgres ping failed: %v", err)
	}
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != postgresSchemaVersion {
		t.Fatalf("schema version=%d want=%d", version, postgresSchemaVersion)
	}

	for _, table := range []string{
		"observations", "networks", "network_servers", "network_endpoints",
		"agent_endpoint_state", "endpoint_transition_events", "incident_records",
	} {
		var exists bool
		if err := store.pool.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("postgres table %s was not created", table)
		}
	}
}

func TestPostgresObservationStoreParity(t *testing.T) {
	store := openTestPostgresStore(t)
	base := time.Date(2026, 9, 9, 20, 0, 0, 0, time.UTC)
	observations := []agent.Observation{
		{
			AgentID: "agent-a",
			ObservedAt: base,
			Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
			Result: probe.EndpointResult{Reachable: true, DualStackOK: true},
		},
		{
			AgentID: "agent-a",
			ObservedAt: base.Add(time.Minute),
			Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
			Result: probe.EndpointResult{Reachable: false},
		},
		{
			AgentID: "agent-b",
			ObservedAt: base.Add(2 * time.Minute),
			Endpoint: agent.Endpoint{Host: "other.example", Port: "6667", TLS: false},
			Result: probe.EndpointResult{Reachable: true},
		},
	}
	for _, observation := range observations {
		if err := store.Store(observation); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Store(observations[0]); err != nil {
		t.Fatal(err)
	}
	count, err := store.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count=%d want=3", count)
	}

	items, err := store.List(ObservationQuery{AgentID: "agent-a", Host: "irc.example", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("list len=%d want=2", len(items))
	}
	if !items[0].ObservedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("newest observed_at=%s", items[0].ObservedAt)
	}

	window, err := store.List(ObservationQuery{Since: base.Add(30 * time.Second), Until: base.Add(90 * time.Second), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(window) != 1 || !window[0].ObservedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("window=%+v", window)
	}

	recent, err := store.RecentObservations(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].AgentID != "agent-b" || !recent[1].ObservedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("recent=%+v", recent)
	}

	latest, err := store.LatestEndpointObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest len=%d want=2", len(latest))
	}
	var foundAgentA bool
	for _, observation := range latest {
		if observation.AgentID == "agent-a" && observation.Endpoint.Host == "irc.example" {
			foundAgentA = true
			if !observation.ObservedAt.Equal(base.Add(time.Minute)) || observation.Result.Reachable {
				t.Fatalf("agent-a latest=%+v", observation)
			}
		}
	}
	if !foundAgentA {
		t.Fatal("latest endpoint observations missing agent-a")
	}
}

func TestPostgresEndpointIncidentPersistence(t *testing.T) {
	store := openTestPostgresStore(t)
	base := time.Date(2026, 9, 9, 21, 0, 0, 0, time.UTC)
	observation := func(agentID string, at time.Time, reachable bool) agent.Observation {
		return agent.Observation{
			AgentID: agentID,
			ObservedAt: at,
			Endpoint: agent.Endpoint{Host: "incident.example", Port: "6697", TLS: true},
			Result: probe.EndpointResult{Reachable: reachable},
		}
	}
	for _, item := range []agent.Observation{
		observation("agent-a", base, true),
		observation("agent-b", base.Add(10*time.Second), true),
		observation("agent-a", base.Add(time.Minute), false),
		observation("agent-b", base.Add(2*time.Minute), false),
	} {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	var transitionCount int
	if err := store.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM endpoint_transition_events`).Scan(&transitionCount); err != nil { t.Fatal(err) }
	if transitionCount != 2 { t.Fatalf("transition count=%d want=2", transitionCount) }
	var stateCount int
	if err := store.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM agent_endpoint_state`).Scan(&stateCount); err != nil { t.Fatal(err) }
	if stateCount != 2 { t.Fatalf("state count=%d want=2", stateCount) }

	open, err := store.ListIncidentRecords(IncidentRecordQuery{Status: "open", Host: "incident.example", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(open) != 1 { t.Fatalf("open incidents=%d want=1", len(open)) }
	if len(open[0].DownAgents) != 2 { t.Fatalf("down agents=%v", open[0].DownAgents) }

	for _, item := range []agent.Observation{
		observation("agent-a", base.Add(10*time.Minute), true),
		observation("agent-b", base.Add(11*time.Minute), true),
	} {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}
	closed, err := store.ListIncidentRecords(IncidentRecordQuery{Status: "closed", Host: "incident.example", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(closed) != 1 { t.Fatalf("closed incidents=%d want=1", len(closed)) }
	if closed[0].RecoveredAt == nil || closed[0].DurationSeconds == nil {
		t.Fatalf("closed lifecycle missing recovery fields: %+v", closed[0])
	}
	if len(closed[0].RecoveryAgents) != 2 { t.Fatalf("recovery agents=%v", closed[0].RecoveryAgents) }

	// Duplicate retries are strict no-ops for transition/lifecycle state.
	if err := store.Store(observation("agent-b", base.Add(11*time.Minute), true)); err != nil { t.Fatal(err) }
	if err := store.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM endpoint_transition_events`).Scan(&transitionCount); err != nil { t.Fatal(err) }
	if transitionCount != 4 { t.Fatalf("transition count after retry=%d want=4", transitionCount) }

	// An older contradictory observation is retained as telemetry, but cannot
	// create a transition or move persisted agent state backwards.
	if err := store.Store(observation("agent-a", base.Add(5*time.Minute), false)); err != nil { t.Fatal(err) }
	if err := store.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM endpoint_transition_events`).Scan(&transitionCount); err != nil { t.Fatal(err) }
	if transitionCount != 4 { t.Fatalf("transition count after out-of-order=%d want=4", transitionCount) }
	var stateObservedAt time.Time
	var stateReachable bool
	if err := store.pool.QueryRow(context.Background(), `
SELECT observed_at, reachable FROM agent_endpoint_state
WHERE agent_id='agent-a' AND endpoint_host='incident.example' AND endpoint_port='6697' AND endpoint_tls=true`).Scan(&stateObservedAt, &stateReachable); err != nil { t.Fatal(err) }
	if !stateObservedAt.Equal(base.Add(10*time.Minute)) || !stateReachable {
		t.Fatalf("state regressed: observed_at=%s reachable=%v", stateObservedAt, stateReachable)
	}
}

func TestPostgresObservationQueryValidation(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.List(ObservationQuery{Limit: 0}); err == nil {
		t.Fatal("expected invalid limit error")
	}
	if _, err := store.List(ObservationQuery{Since: time.Now(), Until: time.Now().Add(-time.Minute), Limit: 10}); err == nil {
		t.Fatal("expected invalid time range error")
	}
	if _, err := store.RecentObservations(0); err == nil {
		t.Fatal("expected invalid recent limit error")
	}
	if _, err := store.ListIncidentRecords(IncidentRecordQuery{Limit: 0}); err == nil {
		t.Fatal("expected invalid incident limit error")
	}
}

func TestPostgresStoreRejectsEmptyURL(t *testing.T) {
	if _, err := OpenPostgresStore(""); err == nil {
		t.Fatal("expected empty postgres URL to fail")
	}
}
