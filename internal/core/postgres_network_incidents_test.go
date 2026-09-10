package core

import (
	"context"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestPostgresNetworkIncidentPersistenceParity(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, `
TRUNCATE network_incident_records, endpoint_transition_events, agent_endpoint_state,
         observations, network_endpoints, network_servers, networks
RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}

	if err := store.UpsertNetwork(Network{ID: "net-a", Name: "Network A"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv-a", NetworkID: "net-a", Name: "irc-a"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-a", ServerID: "srv-a", Host: "irc-a.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-b", ServerID: "srv-a", Host: "irc-b.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	base := time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC)
	obs := func(agentID, host string, at time.Time, reachable bool) agent.Observation {
		return agent.Observation{
			AgentID: agentID,
			ObservedAt: at,
			Endpoint: agent.Endpoint{Host: host, Port: "6697", TLS: true},
			Result: probe.EndpointResult{Reachable: reachable},
		}
	}

	// Establish healthy baselines from two independent agents on both endpoints.
	for _, item := range []agent.Observation{
		obs("agent-a", "irc-a.example", base, true),
		obs("agent-b", "irc-a.example", base.Add(10*time.Second), true),
		obs("agent-a", "irc-b.example", base.Add(20*time.Second), true),
		obs("agent-b", "irc-b.example", base.Add(30*time.Second), true),
	} {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}

	// First endpoint incident makes the network degraded.
	if err := store.Store(obs("agent-a", "irc-a.example", base.Add(time.Minute), false)); err != nil { t.Fatal(err) }
	if err := store.Store(obs("agent-b", "irc-a.example", base.Add(2*time.Minute), false)); err != nil { t.Fatal(err) }
	items, err := store.ListNetworkIncidentRecords(NetworkIncidentRecordQuery{NetworkID: "net-a", Status: "open", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(items) != 1 { t.Fatalf("open incidents=%d want=1", len(items)) }
	if items[0].Severity != "degraded" || items[0].AffectedEndpoints != 1 || items[0].TotalEndpoints != 2 {
		t.Fatalf("degraded incident=%+v", items[0])
	}

	// Second endpoint incident raises the persisted network severity to down.
	if err := store.Store(obs("agent-a", "irc-b.example", base.Add(3*time.Minute), false)); err != nil { t.Fatal(err) }
	if err := store.Store(obs("agent-b", "irc-b.example", base.Add(4*time.Minute), false)); err != nil { t.Fatal(err) }
	items, err = store.ListNetworkIncidentRecords(NetworkIncidentRecordQuery{NetworkID: "net-a", Status: "open", Severity: "down", Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].AffectedEndpoints != 2 { t.Fatalf("down incidents=%+v", items) }

	// Recover both endpoint incidents and require one closed network lifecycle.
	for _, item := range []agent.Observation{
		obs("agent-a", "irc-a.example", base.Add(5*time.Minute), true),
		obs("agent-b", "irc-a.example", base.Add(6*time.Minute), true),
		obs("agent-a", "irc-b.example", base.Add(7*time.Minute), true),
		obs("agent-b", "irc-b.example", base.Add(8*time.Minute), true),
	} {
		if err := store.Store(item); err != nil { t.Fatal(err) }
	}
	items, err = store.ListNetworkIncidentRecords(NetworkIncidentRecordQuery{
		NetworkID: "net-a", Status: "closed", Since: base, Until: base.Add(9*time.Minute), Limit: 10,
	})
	if err != nil { t.Fatal(err) }
	if len(items) != 1 { t.Fatalf("closed incidents=%d want=1: %+v", len(items), items) }
	if items[0].RecoveredAt == nil || items[0].DurationSeconds == nil { t.Fatalf("closed lifecycle missing recovery: %+v", items[0]) }

	var persisted int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM network_incident_records WHERE network_id = 'net-a'`).Scan(&persisted); err != nil { t.Fatal(err) }
	if persisted != 1 { t.Fatalf("persisted network incidents=%d want=1", persisted) }
}

func TestPostgresNetworkIncidentQueryValidation(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.ListNetworkIncidentRecords(NetworkIncidentRecordQuery{Limit: 0}); err == nil { t.Fatal("expected invalid limit") }
	if _, err := store.ListNetworkIncidentRecords(NetworkIncidentRecordQuery{Status: "bad", Limit: 10}); err == nil { t.Fatal("expected invalid status") }
	if _, err := store.ListNetworkIncidentRecords(NetworkIncidentRecordQuery{Severity: "bad", Limit: 10}); err == nil { t.Fatal("expected invalid severity") }
	if _, err := store.ListNetworkIncidentRecords(NetworkIncidentRecordQuery{Since: time.Now(), Until: time.Now().Add(-time.Minute), Limit: 10}); err == nil { t.Fatal("expected invalid time range") }
}
