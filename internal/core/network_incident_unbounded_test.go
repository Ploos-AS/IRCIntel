package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestNetworkIncidentRefreshUsesHistoryBeyond500Records(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	if err := store.UpsertNetwork(Network{ID: "n1", Name: "Net"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "s1", NetworkID: "n1", Name: "Server"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "e1", ServerID: "s1", Host: "irc.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// 501 independent closed endpoint incidents. The oldest one must still feed
	// canonical network derivation; the previous Limit:500 path dropped it.
	for i := 0; i < 501; i++ {
		started := base.Add(time.Duration(i) * time.Hour)
		recovered := started.Add(time.Minute)
		duration := int64(60)
		item := IncidentLifecycle{Host: "irc.example", Port: "6697", TLS: true, Status: "closed", StartedAt: started, RecoveredAt: &recovered, DurationSeconds: &duration}
		payload, err := json.Marshal(item)
		if err != nil { t.Fatal(err) }
		_, err = store.db.Exec(`INSERT INTO incident_records(endpoint_host,endpoint_port,endpoint_tls,started_at,status,payload_json) VALUES(?,?,?,?,?,?)`, "irc.example", "6697", true, formatObservationTime(started), "closed", payload)
		if err != nil { t.Fatal(err) }
	}
	all, err := store.listAllIncidentRecords()
	if err != nil { t.Fatal(err) }
	if len(all) != 501 { t.Fatalf("got %d endpoint incidents, want 501", len(all)) }
	if err := store.refreshNetworkIncidentRecords(); err != nil { t.Fatal(err) }
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM network_incident_records WHERE network_id = ?`, "n1").Scan(&count); err != nil { t.Fatal(err) }
	if count != 501 { t.Fatalf("got %d network incidents, want 501", count) }
}
