package core

import (
	"encoding/json"
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

func TestTransitionReplayUsesLatestLifecycleCheckpoint(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	for _, event := range []IncidentEvent{
		{Type: "down", AgentID: "old-a", Host: "events.example", Port: "6697", TLS: true, ObservedAt: base, PreviousObservedAt: base.Add(-time.Minute)},
		{Type: "down", AgentID: "old-b", Host: "events.example", Port: "6697", TLS: true, ObservedAt: base.Add(time.Minute), PreviousObservedAt: base.Add(-time.Minute)},
		{Type: "down", AgentID: "new-a", Host: "events.example", Port: "6697", TLS: true, ObservedAt: base.Add(2*time.Hour + 30*time.Second), PreviousObservedAt: base.Add(2*time.Hour)},
	} {
		if _, err := store.db.Exec(`INSERT INTO endpoint_transition_events (agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at, previous_observed_at, event_type) VALUES (?, ?, ?, ?, ?, ?, ?)`, event.AgentID, event.Host, event.Port, event.TLS, formatObservationTime(event.ObservedAt), formatObservationTime(event.PreviousObservedAt), event.Type); err != nil { t.Fatal(err) }
	}
	checkpoint := IncidentLifecycle{Host: "events.example", Port: "6697", TLS: true, Status: "closed", StartedAt: base.Add(2*time.Hour), DownAgents: []string{"x", "y"}}
	payload, err := json.Marshal(checkpoint)
	if err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`INSERT INTO incident_records (endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json) VALUES (?, ?, ?, ?, ?, ?)`, checkpoint.Host, checkpoint.Port, checkpoint.TLS, formatObservationTime(checkpoint.StartedAt), checkpoint.Status, payload); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`INSERT INTO observations (agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json) VALUES ('latest', ?, 'events.example', '6697', 1, '{}')`, formatObservationTime(base.Add(3*time.Hour))); err != nil { t.Fatal(err) }

	tx, err := store.db.Begin()
	if err != nil { t.Fatal(err) }
	defer tx.Rollback()
	events, err := transitionEventsForLatestEndpointTx(tx)
	if err != nil { t.Fatal(err) }
	if len(events) != 1 { t.Fatalf("events=%d want=1", len(events)) }
	if events[0].AgentID != "new-a" { t.Fatalf("agent=%q want=new-a", events[0].AgentID) }
}

func TestTransitionReplayKeepsCorrelationBoundary(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	checkpointAt := base.Add(time.Hour)
	boundary := checkpointAt.Add(-defaultCorrelationWindow)

	for _, event := range []IncidentEvent{
		{Type: "down", AgentID: "at-boundary", Host: "events.example", Port: "6697", TLS: true, ObservedAt: boundary, PreviousObservedAt: boundary.Add(-time.Minute)},
		{Type: "down", AgentID: "too-old", Host: "events.example", Port: "6697", TLS: true, ObservedAt: boundary.Add(-time.Nanosecond), PreviousObservedAt: boundary.Add(-time.Minute)},
	} {
		if _, err := store.db.Exec(`INSERT INTO endpoint_transition_events (agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at, previous_observed_at, event_type) VALUES (?, ?, ?, ?, ?, ?, ?)`, event.AgentID, event.Host, event.Port, event.TLS, formatObservationTime(event.ObservedAt), formatObservationTime(event.PreviousObservedAt), event.Type); err != nil { t.Fatal(err) }
	}
	checkpoint := IncidentLifecycle{Host: "events.example", Port: "6697", TLS: true, Status: "open", StartedAt: checkpointAt, DownAgents: []string{"a", "b"}}
	payload, err := json.Marshal(checkpoint)
	if err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`INSERT INTO incident_records (endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json) VALUES (?, ?, ?, ?, ?, ?)`, checkpoint.Host, checkpoint.Port, checkpoint.TLS, formatObservationTime(checkpoint.StartedAt), checkpoint.Status, payload); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`INSERT INTO observations (agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json) VALUES ('latest', ?, 'events.example', '6697', 1, '{}')`, formatObservationTime(checkpointAt.Add(time.Minute))); err != nil { t.Fatal(err) }

	tx, err := store.db.Begin()
	if err != nil { t.Fatal(err) }
	defer tx.Rollback()
	events, err := transitionEventsForLatestEndpointTx(tx)
	if err != nil { t.Fatal(err) }
	if len(events) != 1 || events[0].AgentID != "at-boundary" { t.Fatalf("events=%+v", events) }
}
