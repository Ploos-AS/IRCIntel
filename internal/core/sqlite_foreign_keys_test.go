package core

import (
	"path/filepath"
	"testing"
)

func TestSQLiteStoreEnforcesForeignKeys(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var enabled int
	if err := store.db.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys=%d want=1", enabled)
	}

	if _, err := store.db.Exec(`INSERT INTO network_servers (id, network_id, name) VALUES (?, ?, ?)`, "orphan-server", "missing-network", "Orphan"); err == nil {
		t.Fatal("expected foreign key violation for orphan network server")
	}

	if err := store.UpsertNetwork(Network{ID: "net-1", Name: "Network"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkServer(NetworkServer{ID: "server-1", NetworkID: "net-1", Name: "Server"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO network_endpoints (id, server_id, host, port, tls) VALUES (?, ?, ?, ?, ?)`, "orphan-endpoint", "missing-server", "irc.example", "6697", true); err == nil {
		t.Fatal("expected foreign key violation for orphan endpoint")
	}
}
