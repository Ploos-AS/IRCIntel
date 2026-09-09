package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

func TestEndpointIncidentRefreshIgnoresUnrelatedObservationPayloads(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	if _, err := store.db.Exec(`INSERT INTO observations
(agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json)
VALUES (?, ?, ?, ?, ?, ?)`,
		"other-agent",
		formatObservationTime(time.Date(2026, 9, 9, 1, 0, 0, 0, time.UTC)),
		"other.example", "6697", true, []byte("{")); err != nil {
		t.Fatal(err)
	}

	if err := store.Store(validObservation()); err != nil {
		t.Fatalf("unrelated corrupt observation should not block ingest: %v", err)
	}
	count, err := store.Count()
	if err != nil { t.Fatal(err) }
	if count != 2 { t.Fatalf("count=%d want=2", count) }
}

func TestEndpointIncidentRefreshRejectsCorruptPayloadOnAffectedEndpoint(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	observation := validObservation()
	if _, err := store.db.Exec(`INSERT INTO observations
(agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json)
VALUES (?, ?, ?, ?, ?, ?)`,
		"older-agent",
		formatObservationTime(observation.ObservedAt.Add(-time.Minute)),
		observation.Endpoint.Host, observation.Endpoint.Port, observation.Endpoint.TLS, []byte("{")); err != nil {
		t.Fatal(err)
	}

	if err := store.Store(observation); err == nil {
		t.Fatal("expected affected endpoint refresh to reject corrupt payload")
	}
	count, err := store.Count()
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("count=%d want=1 after rollback", count) }
}

func TestObservationsForLatestEndpointTxReturnsOnlyAffectedEndpoint(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	other := agent.Observation{
		AgentID: "other",
		ObservedAt: time.Date(2026, 9, 9, 1, 0, 0, 0, time.UTC),
		Endpoint: agent.Endpoint{Host: "other.example", Port: "6697", TLS: true},
	}
	if err := store.Store(other); err != nil { t.Fatal(err) }
	wanted := validObservation()
	if err := store.Store(wanted); err != nil { t.Fatal(err) }

	tx, err := store.db.Begin()
	if err != nil { t.Fatal(err) }
	defer tx.Rollback()
	items, err := observationsForLatestEndpointTx(tx)
	if err != nil { t.Fatal(err) }
	if len(items) != 1 { t.Fatalf("len=%d want=1", len(items)) }
	if items[0].Endpoint.Host != wanted.Endpoint.Host { t.Fatalf("host=%q want=%q", items[0].Endpoint.Host, wanted.Endpoint.Host) }
}
