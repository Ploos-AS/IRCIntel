package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func transitionObservation(agentID string, at time.Time, reachable bool) agent.Observation {
	return agent.Observation{
		AgentID: agentID,
		ObservedAt: at,
		Endpoint: agent.Endpoint{Host: "events.example", Port: "6697", TLS: true},
		Result: probe.EndpointResult{Reachable: reachable},
	}
}

func TestSteadyObservationDoesNotPersistTransition(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 18, 0, 0, 0, time.UTC)
	if err := store.Store(transitionObservation("agent-a", base, true)); err != nil { t.Fatal(err) }
	if err := store.Store(transitionObservation("agent-a", base.Add(time.Minute), true)); err != nil { t.Fatal(err) }
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM endpoint_transition_events`).Scan(&count); err != nil { t.Fatal(err) }
	if count != 0 { t.Fatalf("transition count=%d want=0", count) }
}

func TestReachabilityChangePersistsTransition(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 18, 0, 0, 0, time.UTC)
	if err := store.Store(transitionObservation("agent-a", base, true)); err != nil { t.Fatal(err) }
	if err := store.Store(transitionObservation("agent-a", base.Add(time.Minute), false)); err != nil { t.Fatal(err) }
	var eventType string
	if err := store.db.QueryRow(`SELECT event_type FROM endpoint_transition_events`).Scan(&eventType); err != nil { t.Fatal(err) }
	if eventType != "down" { t.Fatalf("event_type=%q want=down", eventType) }
}

func TestTwoAgentDownTransitionsCreateIncident(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 18, 0, 0, 0, time.UTC)
	for _, observation := range []agent.Observation{
		transitionObservation("agent-a", base, true),
		transitionObservation("agent-b", base.Add(10*time.Second), true),
		transitionObservation("agent-a", base.Add(time.Minute), false),
		transitionObservation("agent-b", base.Add(2*time.Minute), false),
	} {
		if err := store.Store(observation); err != nil { t.Fatal(err) }
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM incident_records WHERE endpoint_host = 'events.example' AND status = 'open'`).Scan(&count); err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("open incident count=%d want=1", count) }
}

func TestOutOfOrderObservationDoesNotCreateTransition(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 18, 0, 0, 0, time.UTC)
	if err := store.Store(transitionObservation("agent-a", base.Add(2*time.Minute), true)); err != nil { t.Fatal(err) }
	if err := store.Store(transitionObservation("agent-a", base, false)); err != nil { t.Fatal(err) }
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM endpoint_transition_events`).Scan(&count); err != nil { t.Fatal(err) }
	if count != 0 { t.Fatalf("transition count=%d want=0", count) }
}
