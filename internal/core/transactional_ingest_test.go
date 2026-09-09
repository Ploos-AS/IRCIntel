package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreRollsBackObservationWhenDerivedRefreshFails(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	if err := store.UpsertNetwork(Network{ID: "net-1", Name: "Network 1"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv-1", NetworkID: "net-1", Name: "Server 1"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-1", ServerID: "srv-1", Host: "irc.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	// Establish persisted per-agent state first. The second observation below is
	// therefore a real transition and must execute the derived incident path.
	baseline := validObservation()
	baseline.Result.Reachable = true
	if err := store.Store(baseline); err != nil { t.Fatal(err) }

	// Seed a deliberately corrupt incident payload in the affected network.
	// Network derivation must fail and roll back the transition observation,
	// transition event, state change, and every other derived write atomically.
	if _, err := store.db.Exec(`
INSERT INTO incident_records (
    endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json
) VALUES (?, ?, ?, ?, ?, ?)`,
		"irc.example", "6697", true,
		formatObservationTime(time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)),
		"open", []byte("{")); err != nil {
		t.Fatal(err)
	}

	transition := baseline
	transition.ObservedAt = baseline.ObservedAt.Add(time.Minute)
	transition.Result.Reachable = false
	if err := store.Store(transition); err == nil {
		t.Fatal("expected derived refresh failure")
	}
	count, err := store.Count()
	if err != nil { t.Fatal(err) }
	if count != 1 {
		t.Fatalf("observation count=%d want=1 after rollback", count)
	}
	var transitions int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM endpoint_transition_events`).Scan(&transitions); err != nil { t.Fatal(err) }
	if transitions != 0 {
		t.Fatalf("transition count=%d want=0 after rollback", transitions)
	}
}

func TestNetworkIncidentRefreshIgnoresUnrelatedNetworkHistory(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	for _, network := range []Network{{ID: "net-a", Name: "A"}, {ID: "net-b", Name: "B"}} {
		if err := store.UpsertNetwork(network); err != nil { t.Fatal(err) }
	}
	for _, server := range []NetworkServer{{ID: "srv-a", NetworkID: "net-a", Name: "A"}, {ID: "srv-b", NetworkID: "net-b", Name: "B"}} {
		if err := store.UpsertNetworkServer(server); err != nil { t.Fatal(err) }
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-a", ServerID: "srv-a", Host: "irc.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-b", ServerID: "srv-b", Host: "other.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	if _, err := store.db.Exec(`INSERT INTO incident_records (endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json) VALUES (?, ?, ?, ?, ?, ?)`,
		"other.example", "6697", true, formatObservationTime(time.Date(2026, 9, 9, 1, 0, 0, 0, time.UTC)), "open", []byte("{")); err != nil { t.Fatal(err) }

	if err := store.Store(validObservation()); err != nil {
		t.Fatalf("unrelated corrupt history should not block ingest: %v", err)
	}
}

func TestAffectedNetworkIDsForLatestObservationTx(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	if err := store.UpsertNetwork(Network{ID: "net-1", Name: "Network 1"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "srv-1", NetworkID: "net-1", Name: "Server 1"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-1", ServerID: "srv-1", Host: "irc.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	tx, err := store.db.Begin()
	if err != nil { t.Fatal(err) }
	defer tx.Rollback()
	observation := validObservation()
	if _, err := tx.Exec(`INSERT INTO observations (agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json) VALUES (?, ?, ?, ?, ?, '{}')`, observation.AgentID, formatObservationTime(observation.ObservedAt), observation.Endpoint.Host, observation.Endpoint.Port, observation.Endpoint.TLS); err != nil { t.Fatal(err) }
	ids, err := affectedNetworkIDsForLatestObservationTx(tx)
	if err != nil { t.Fatal(err) }
	if len(ids) != 1 || ids[0] != "net-1" { t.Fatalf("network ids=%v want=[net-1]", ids) }
}

func TestIncidentRecordsForNetworksTxReturnsOnlyRequestedNetworks(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	for _, network := range []Network{{ID: "net-a", Name: "A"}, {ID: "net-b", Name: "B"}} { if err := store.UpsertNetwork(network); err != nil { t.Fatal(err) } }
	for _, server := range []NetworkServer{{ID: "srv-a", NetworkID: "net-a", Name: "A"}, {ID: "srv-b", NetworkID: "net-b", Name: "B"}} { if err := store.UpsertNetworkServer(server); err != nil { t.Fatal(err) } }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-a", ServerID: "srv-a", Host: "a.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-b", ServerID: "srv-b", Host: "b.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	payloadA := []byte(`{"host":"a.example","port":"6697","tls":true,"status":"open","started_at":"2026-09-09T01:00:00Z"}`)
	payloadB := []byte(`{"host":"b.example","port":"6697","tls":true,"status":"open","started_at":"2026-09-09T02:00:00Z"}`)
	for _, row := range []struct{ host string; payload []byte }{{"a.example", payloadA}, {"b.example", payloadB}} {
		if _, err := store.db.Exec(`INSERT INTO incident_records (endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json) VALUES (?, '6697', 1, ?, 'open', ?)`, row.host, formatObservationTime(time.Now().UTC()), row.payload); err != nil { t.Fatal(err) }
	}

	tx, err := store.db.Begin()
	if err != nil { t.Fatal(err) }
	defer tx.Rollback()
	items, err := incidentRecordsForNetworksTx(tx, []string{"net-a"})
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].Host != "a.example" { t.Fatalf("items=%+v", items) }
}

func TestSQLiteStoreAtomicIngestStillCommitsSuccessfulObservation(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	observation := validObservation()
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	count, err := store.Count()
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("observation count=%d want=1", count) }
}
