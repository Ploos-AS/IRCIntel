package core

import (
	"context"
	"testing"
)

func resetPostgresOwnershipTestState(t *testing.T, store *PostgresStore) {
	t.Helper()
	ctx := context.Background()
	tx, err := store.pool.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { t.Fatal(err) }
	if _, err := tx.Exec(ctx, `
TRUNCATE endpoint_network_ownership, endpoint_network_ownership_meta,
         discovery_promotions, discovery_reviews, discovery_candidates,
         network_incident_records, network_endpoints, network_servers, networks,
         observation_rollups_hourly, observation_rollups_daily,
         endpoint_transition_events, agent_endpoint_state, incident_records, observations
RESTART IDENTITY CASCADE`); err != nil { t.Fatal(err) }
	if err := tx.Commit(ctx); err != nil { t.Fatal(err) }
}

func TestPostgresEndpointOwnershipTracksServerNetworkMove(t *testing.T) {
	store := openTestPostgresStore(t)
	resetPostgresOwnershipTestState(t, store)

	if err := store.UpsertNetwork(Network{ID: "net-a", Name: "Network A"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetwork(Network{ID: "net-b", Name: "Network B"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "server-a", NetworkID: "net-a", Name: "Server A"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "endpoint-a", ServerID: "server-a", Host: "irc.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }

	before, err := store.EndpointNetworkOwnershipHistory("endpoint-a")
	if err != nil { t.Fatal(err) }
	if len(before) != 1 { t.Fatalf("before intervals=%d", len(before)) }
	if before[0].NetworkID != "net-a" || before[0].ValidTo != nil { t.Fatalf("before=%+v", before[0]) }

	if err := store.UpsertNetworkServer(NetworkServer{ID: "server-a", NetworkID: "net-b", Name: "Server A"}); err != nil { t.Fatal(err) }
	after, err := store.EndpointNetworkOwnershipHistory("endpoint-a")
	if err != nil { t.Fatal(err) }
	if len(after) != 2 { t.Fatalf("after intervals=%d", len(after)) }
	if after[0].NetworkID != "net-a" || after[0].ValidTo == nil { t.Fatalf("first interval=%+v", after[0]) }
	if after[1].NetworkID != "net-b" || after[1].ValidTo != nil { t.Fatalf("second interval=%+v", after[1]) }
	if !after[1].ValidFrom.After(after[0].ValidFrom) { t.Fatalf("interval order=%+v", after) }
}

func TestPostgresEndpointOwnershipTracksEndpointIdentityChange(t *testing.T) {
	store := openTestPostgresStore(t)
	resetPostgresOwnershipTestState(t, store)

	if err := store.UpsertNetwork(Network{ID: "net-a", Name: "Network A"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkServer(NetworkServer{ID: "server-a", NetworkID: "net-a", Name: "Server A"}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "endpoint-a", ServerID: "server-a", Host: "old.example", Port: "6697", TLS: true}); err != nil { t.Fatal(err) }
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "endpoint-a", ServerID: "server-a", Host: "new.example", Port: "6667", TLS: false}); err != nil { t.Fatal(err) }

	history, err := store.EndpointNetworkOwnershipHistory("endpoint-a")
	if err != nil { t.Fatal(err) }
	if len(history) != 2 { t.Fatalf("intervals=%d", len(history)) }
	if history[0].Host != "old.example" || history[0].Port != "6697" || !history[0].TLS || history[0].ValidTo == nil { t.Fatalf("old=%+v", history[0]) }
	if history[1].Host != "new.example" || history[1].Port != "6667" || history[1].TLS || history[1].ValidTo != nil { t.Fatalf("new=%+v", history[1]) }

	// Repeating the same registry state must not create another ownership interval.
	if err := store.UpsertNetworkEndpoint(NetworkEndpoint{ID: "endpoint-a", ServerID: "server-a", Host: "new.example", Port: "6667", TLS: false}); err != nil { t.Fatal(err) }
	history, err = store.EndpointNetworkOwnershipHistory("endpoint-a")
	if err != nil { t.Fatal(err) }
	if len(history) != 2 { t.Fatalf("no-op upsert created interval: %d", len(history)) }
}

func TestPostgresEndpointOwnershipBootstrapFreezesExistingRegistry(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx := context.Background()
	if _, err := store.pool.Exec(ctx, `
TRUNCATE endpoint_network_ownership, endpoint_network_ownership_meta,
         discovery_promotions, discovery_reviews, discovery_candidates,
         network_incident_records, network_endpoints, network_servers, networks
RESTART IDENTITY CASCADE`); err != nil {
		// Ownership tables may not exist until this milestone is exercised.
		tx, beginErr := store.pool.Begin(ctx)
		if beginErr != nil { t.Fatal(beginErr) }
		if ensureErr := ensurePostgresNetworkOwnershipTx(ctx, tx); ensureErr != nil { _ = tx.Rollback(ctx); t.Fatal(ensureErr) }
		if commitErr := tx.Commit(ctx); commitErr != nil { t.Fatal(commitErr) }
		if _, err = store.pool.Exec(ctx, `TRUNCATE endpoint_network_ownership, endpoint_network_ownership_meta, network_endpoints, network_servers, networks RESTART IDENTITY CASCADE`); err != nil { t.Fatal(err) }
	}

	// Simulate a pre-M4.18 registry populated before the ownership journal exists.
	if _, err := store.pool.Exec(ctx, `
INSERT INTO networks(id,name) VALUES ('legacy-net','Legacy Network');
INSERT INTO network_servers(id,network_id,name) VALUES ('legacy-server','legacy-net','Legacy Server');
INSERT INTO network_endpoints(id,server_id,host,port,tls) VALUES ('legacy-endpoint','legacy-server','legacy.example','6697',true);`); err != nil { t.Fatal(err) }

	history, err := store.EndpointNetworkOwnershipHistory("legacy-endpoint")
	if err != nil { t.Fatal(err) }
	if len(history) != 1 { t.Fatalf("bootstrap intervals=%d", len(history)) }
	if history[0].NetworkID != "legacy-net" || history[0].ValidTo != nil { t.Fatalf("bootstrap=%+v", history[0]) }

	// The baseline must be stable across repeated reads/bootstrap attempts.
	again, err := store.EndpointNetworkOwnershipHistory("legacy-endpoint")
	if err != nil { t.Fatal(err) }
	if len(again) != 1 { t.Fatalf("repeat bootstrap intervals=%d", len(again)) }
}
