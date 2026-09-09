package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func stateObservation(at time.Time, reachable bool) agent.Observation {
	return agent.Observation{
		AgentID:    "state-agent",
		ObservedAt: at,
		Endpoint:   agent.Endpoint{Host: "state.example", Port: "6697", TLS: true},
		Result:     probe.EndpointResult{Reachable: reachable},
	}
}

func TestAgentEndpointStateTracksNewestObservation(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	if err := store.Store(stateObservation(base, true)); err != nil { t.Fatal(err) }
	if err := store.Store(stateObservation(base.Add(time.Minute), false)); err != nil { t.Fatal(err) }

	var observedAt string
	var reachable bool
	if err := store.db.QueryRow(`
SELECT observed_at, reachable
FROM agent_endpoint_state
WHERE agent_id = ? AND endpoint_host = ? AND endpoint_port = ? AND endpoint_tls = ?`,
		"state-agent", "state.example", "6697", true).Scan(&observedAt, &reachable); err != nil {
		t.Fatal(err)
	}
	if observedAt != formatObservationTime(base.Add(time.Minute)) {
		t.Fatalf("observed_at=%q want=%q", observedAt, formatObservationTime(base.Add(time.Minute)))
	}
	if reachable {
		t.Fatal("reachable=true want=false")
	}
}

func TestAgentEndpointStateDoesNotRegressOnOutOfOrderObservation(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	newest := stateObservation(base.Add(2*time.Minute), false)
	older := stateObservation(base.Add(time.Minute), true)
	if err := store.Store(newest); err != nil { t.Fatal(err) }
	if err := store.Store(older); err != nil { t.Fatal(err) }

	var observedAt string
	var reachable bool
	if err := store.db.QueryRow(`SELECT observed_at, reachable FROM agent_endpoint_state WHERE agent_id = 'state-agent' AND endpoint_host = 'state.example'`).Scan(&observedAt, &reachable); err != nil {
		t.Fatal(err)
	}
	if observedAt != formatObservationTime(newest.ObservedAt) {
		t.Fatalf("observed_at=%q want newest=%q", observedAt, formatObservationTime(newest.ObservedAt))
	}
	if reachable {
		t.Fatal("out-of-order observation regressed reachability state")
	}
}

func TestMigrateAgentEndpointStateBackfillsLatestObservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }

	base := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	for _, observation := range []agent.Observation{
		stateObservation(base, true),
		stateObservation(base.Add(time.Minute), false),
	} {
		payload, err := json.Marshal(observation)
		if err != nil { t.Fatal(err) }
		if _, err := store.db.Exec(`
INSERT INTO observations (agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json)
VALUES (?, ?, ?, ?, ?, ?)`, observation.AgentID, formatObservationTime(observation.ObservedAt), observation.Endpoint.Host, observation.Endpoint.Port, observation.Endpoint.TLS, payload); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec(`DROP TABLE agent_endpoint_state`); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`PRAGMA user_version = 4`); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	store, err = OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer store.Close()

	var observedAt string
	var reachable bool
	if err := store.db.QueryRow(`SELECT observed_at, reachable FROM agent_endpoint_state WHERE agent_id = 'state-agent' AND endpoint_host = 'state.example'`).Scan(&observedAt, &reachable); err != nil {
		t.Fatal(err)
	}
	if observedAt != formatObservationTime(base.Add(time.Minute)) {
		t.Fatalf("observed_at=%q want=%q", observedAt, formatObservationTime(base.Add(time.Minute)))
	}
	if reachable {
		t.Fatal("backfill did not select newest reachability state")
	}
	var version int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil { t.Fatal(err) }
	if version != sqliteSchemaVersion { t.Fatalf("user_version=%d want=%d", version, sqliteSchemaVersion) }
}
