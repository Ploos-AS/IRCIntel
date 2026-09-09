package core

import (
	"errors"
	"os"
	"testing"
)

func openPostgresRegistryTestStore(t *testing.T) *PostgresStore {
	t.Helper()
	databaseURL := os.Getenv("IRCINTEL_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("IRCINTEL_TEST_POSTGRES_URL not configured")
	}
	store, err := OpenPostgresStore(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(t.Context(), `TRUNCATE network_endpoints, network_servers, networks RESTART IDENTITY CASCADE`); err != nil {
		store.Close()
		t.Fatal(err)
	}
	return store
}

func TestPostgresRegistryParity(t *testing.T) {
	store := openPostgresRegistryTestStore(t)
	defer store.Close()

	if err := store.UpsertNetwork(Network{ID: "libera", Name: " Libera.Chat ", Website: " https://libera.chat ", Description: " public IRC "}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkServer(NetworkServer{ID: "server-a", NetworkID: "libera", Name: " Server A "}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-b", ServerID: "server-a", Host: " IRC2.EXAMPLE ", Port: "6697", TLS: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-a", ServerID: "server-a", Host: " IRC1.EXAMPLE ", Port: "6667", TLS: false}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := store.RegistrySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Networks) != 1 || len(snapshot.Servers) != 1 || len(snapshot.Endpoints) != 2 {
		t.Fatalf("unexpected snapshot sizes: %+v", snapshot)
	}
	if snapshot.Networks[0].Name != "Libera.Chat" || snapshot.Networks[0].Website != "https://libera.chat" || snapshot.Networks[0].Description != "public IRC" {
		t.Fatalf("network normalization mismatch: %+v", snapshot.Networks[0])
	}
	if snapshot.Servers[0].Name != "Server A" {
		t.Fatalf("server normalization mismatch: %+v", snapshot.Servers[0])
	}
	if snapshot.Endpoints[0].ID != "ep-a" || snapshot.Endpoints[0].Host != "irc1.example" || snapshot.Endpoints[1].ID != "ep-b" || snapshot.Endpoints[1].Host != "irc2.example" {
		t.Fatalf("endpoint order/normalization mismatch: %+v", snapshot.Endpoints)
	}

	if err := store.UpsertNetwork(Network{ID: "libera", Name: "Libera IRC", Website: "https://libera.chat/new"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkServer(NetworkServer{ID: "server-a", NetworkID: "libera", Name: "Server A2"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "ep-a", ServerID: "server-a", Host: "new.example", Port: "7000", TLS: true}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.RegistrySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Networks[0].Name != "Libera IRC" || snapshot.Servers[0].Name != "Server A2" {
		t.Fatalf("upsert did not update registry: %+v", snapshot)
	}
	var updated NetworkEndpoint
	for _, endpoint := range snapshot.Endpoints {
		if endpoint.ID == "ep-a" {
			updated = endpoint
		}
	}
	if updated.Host != "new.example" || updated.Port != "7000" || !updated.TLS {
		t.Fatalf("endpoint update mismatch: %+v", updated)
	}
}

func TestPostgresRegistryTypedParentErrors(t *testing.T) {
	store := openPostgresRegistryTestStore(t)
	defer store.Close()

	if err := store.UpsertNetworkServer(NetworkServer{ID: "server", NetworkID: "missing", Name: "Missing parent"}); !errors.Is(err, ErrRegistryNetworkNotFound) {
		t.Fatalf("server parent error=%v", err)
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "endpoint", ServerID: "missing", Host: "example.net", Port: "6697", TLS: true}); !errors.Is(err, ErrRegistryServerNotFound) {
		t.Fatalf("endpoint parent error=%v", err)
	}
}

func TestPostgresRegistryRejectsInvalidInput(t *testing.T) {
	store := openPostgresRegistryTestStore(t)
	defer store.Close()

	if err := store.UpsertNetwork(Network{}); err == nil {
		t.Fatal("expected invalid network to fail")
	}
	if err := store.UpsertNetworkServer(NetworkServer{}); err == nil {
		t.Fatal("expected invalid server to fail")
	}
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{}); err == nil {
		t.Fatal("expected invalid endpoint to fail")
	}
}
