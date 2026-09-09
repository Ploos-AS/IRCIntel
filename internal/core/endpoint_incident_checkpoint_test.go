package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func checkpointObservation(agentID string, observedAt time.Time, reachable bool) agent.Observation {
	return agent.Observation{
		AgentID:    agentID,
		ObservedAt: observedAt,
		Endpoint:   agent.Endpoint{Host: "checkpoint.example", Port: "6697", TLS: true},
		Result:     probe.EndpointResult{Reachable: reachable},
	}
}

func seedCheckpointIncident(t *testing.T, store *SQLiteStore) (time.Time, time.Time) {
	t.Helper()
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	down := base.Add(time.Hour)
	for _, observation := range []agent.Observation{
		checkpointObservation("agent-a", base, true),
		checkpointObservation("agent-b", base.Add(time.Minute), true),
		checkpointObservation("agent-a", down, false),
		checkpointObservation("agent-b", down.Add(time.Minute), false),
	} {
		if err := store.Store(observation); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM incident_records WHERE endpoint_host = 'checkpoint.example'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("incident count=%d want=1", count)
	}
	return base, down
}

func TestEndpointIncidentCheckpointKeepsBaselineForRecovery(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	_, down := seedCheckpointIncident(t, store)
	if err := store.Store(checkpointObservation("agent-a", down.Add(10*time.Minute), true)); err != nil { t.Fatal(err) }
	if err := store.Store(checkpointObservation("agent-b", down.Add(11*time.Minute), true)); err != nil { t.Fatal(err) }

	var status string
	var payload []byte
	if err := store.db.QueryRow(`SELECT status, payload_json FROM incident_records WHERE endpoint_host = 'checkpoint.example' ORDER BY started_at DESC LIMIT 1`).Scan(&status, &payload); err != nil {
		t.Fatal(err)
	}
	if status != "closed" {
		t.Fatalf("status=%q want=closed", status)
	}
	var lifecycle IncidentLifecycle
	if err := json.Unmarshal(payload, &lifecycle); err != nil { t.Fatal(err) }
	if lifecycle.RecoveredAt == nil {
		t.Fatal("expected recovered_at")
	}
}

func TestEndpointIncidentCheckpointIgnoresAncientInactiveAgentPayload(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base, down := seedCheckpointIncident(t, store)
	if _, err := store.db.Exec(`INSERT INTO observations
(agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json)
VALUES (?, ?, ?, ?, ?, ?)`,
		"ancient-agent", formatObservationTime(base.Add(-time.Hour)),
		"checkpoint.example", "6697", true, []byte("{")); err != nil {
		t.Fatal(err)
	}

	if err := store.Store(checkpointObservation("agent-a", down.Add(2*time.Minute), false)); err != nil {
		t.Fatalf("ancient inactive payload should be outside checkpoint replay: %v", err)
	}
}
