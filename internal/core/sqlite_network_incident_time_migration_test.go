package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreV3ToV4NormalizesNetworkIncidentTimes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }

	startedAt := time.Date(2026, 9, 9, 8, 0, 0, 100_000_000, time.UTC)
	incident := NetworkIncident{
		NetworkID: "net",
		NetworkName: "Network",
		Status: "open",
		Severity: "degraded",
		StartedAt: startedAt,
		AffectedEndpoints: 1,
		TotalEndpoints: 2,
	}
	payload, err := json.Marshal(incident)
	if err != nil { t.Fatal(err) }
	legacy := startedAt.Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO network_incident_records (network_id, started_at, status, severity, payload_json) VALUES (?, ?, ?, ?, ?)`, incident.NetworkID, legacy, incident.Status, incident.Severity, payload); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`PRAGMA user_version = 3`); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	reopened, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()

	var stored string
	if err := reopened.db.QueryRow(`SELECT started_at FROM network_incident_records WHERE network_id = ?`, incident.NetworkID).Scan(&stored); err != nil { t.Fatal(err) }
	if want := "2026-09-09T08:00:00.100000000Z"; stored != want { t.Fatalf("started_at=%q want=%q", stored, want) }

	var version int
	if err := reopened.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil { t.Fatal(err) }
	if version != sqliteSchemaVersion { t.Fatalf("user_version=%d want=%d", version, sqliteSchemaVersion) }
}
