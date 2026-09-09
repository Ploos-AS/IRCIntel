package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreRollsBackObservationWhenDerivedRefreshFails(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Seed a deliberately corrupt historical incident payload. Endpoint incident
	// refresh can still run, but canonical network derivation must reject this
	// payload after the new observation has been inserted into the transaction.
	if _, err := store.db.Exec(`
INSERT INTO incident_records (
    endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json
) VALUES (?, ?, ?, ?, ?, ?)`,
		"broken.example", "6697", true,
		formatObservationTime(time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)),
		"open", []byte("{")); err != nil {
		t.Fatal(err)
	}

	if err := store.Store(validObservation()); err == nil {
		t.Fatal("expected derived refresh failure")
	}
	count, err := store.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("observation count=%d want=0 after rollback", count)
	}
}

func TestSQLiteStoreAtomicIngestStillCommitsSuccessfulObservation(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	observation := validObservation()
	if err := store.Store(observation); err != nil {
		t.Fatal(err)
	}
	if err := store.Store(observation); err != nil {
		t.Fatal(err)
	}
	count, err := store.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("observation count=%d want=1", count)
	}
}
