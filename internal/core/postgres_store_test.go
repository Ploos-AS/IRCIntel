package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func openTestPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()
	databaseURL := os.Getenv("IRCINTEL_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("IRCINTEL_TEST_POSTGRES_URL not configured")
	}
	store, err := OpenPostgresStore(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(context.Background(), `TRUNCATE observations RESTART IDENTITY CASCADE`); err != nil {
		store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPostgresStoreFoundation(t *testing.T) {
	store := openTestPostgresStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("postgres ping failed: %v", err)
	}
	version, err := store.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != postgresSchemaVersion {
		t.Fatalf("schema version=%d want=%d", version, postgresSchemaVersion)
	}

	for _, table := range []string{"observations", "networks", "network_servers", "network_endpoints"} {
		var exists bool
		if err := store.pool.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("postgres table %s was not created", table)
		}
	}
}

func TestPostgresObservationStoreParity(t *testing.T) {
	store := openTestPostgresStore(t)
	base := time.Date(2026, 9, 9, 20, 0, 0, 0, time.UTC)
	observations := []agent.Observation{
		{
			AgentID: "agent-a",
			ObservedAt: base,
			Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
			Result: probe.EndpointResult{Reachable: true, DualStackOK: true},
		},
		{
			AgentID: "agent-a",
			ObservedAt: base.Add(time.Minute),
			Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
			Result: probe.EndpointResult{Reachable: false},
		},
		{
			AgentID: "agent-b",
			ObservedAt: base.Add(2 * time.Minute),
			Endpoint: agent.Endpoint{Host: "other.example", Port: "6667", TLS: false},
			Result: probe.EndpointResult{Reachable: true},
		},
	}
	for _, observation := range observations {
		if err := store.Store(observation); err != nil {
			t.Fatal(err)
		}
	}
	// Idempotent retry must not duplicate an observation.
	if err := store.Store(observations[0]); err != nil {
		t.Fatal(err)
	}
	count, err := store.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count=%d want=3", count)
	}

	items, err := store.List(ObservationQuery{AgentID: "agent-a", Host: "irc.example", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("list len=%d want=2", len(items))
	}
	if !items[0].ObservedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("newest observed_at=%s", items[0].ObservedAt)
	}

	window, err := store.List(ObservationQuery{Since: base.Add(30 * time.Second), Until: base.Add(90 * time.Second), Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(window) != 1 || !window[0].ObservedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("window=%+v", window)
	}

	recent, err := store.RecentObservations(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].AgentID != "agent-b" || !recent[1].ObservedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("recent=%+v", recent)
	}

	latest, err := store.LatestEndpointObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest len=%d want=2", len(latest))
	}
	var foundAgentA bool
	for _, observation := range latest {
		if observation.AgentID == "agent-a" && observation.Endpoint.Host == "irc.example" {
			foundAgentA = true
			if !observation.ObservedAt.Equal(base.Add(time.Minute)) || observation.Result.Reachable {
				t.Fatalf("agent-a latest=%+v", observation)
			}
		}
	}
	if !foundAgentA {
		t.Fatal("latest endpoint observations missing agent-a")
	}
}

func TestPostgresObservationQueryValidation(t *testing.T) {
	store := openTestPostgresStore(t)
	if _, err := store.List(ObservationQuery{Limit: 0}); err == nil {
		t.Fatal("expected invalid limit error")
	}
	if _, err := store.List(ObservationQuery{Since: time.Now(), Until: time.Now().Add(-time.Minute), Limit: 10}); err == nil {
		t.Fatal("expected invalid time range error")
	}
	if _, err := store.RecentObservations(0); err == nil {
		t.Fatal("expected invalid recent limit error")
	}
}

func TestPostgresStoreRejectsEmptyURL(t *testing.T) {
	if _, err := OpenPostgresStore(""); err == nil {
		t.Fatal("expected empty postgres URL to fail")
	}
}
