package core

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPostgresStoreFoundation(t *testing.T) {
	databaseURL := os.Getenv("IRCINTEL_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("IRCINTEL_TEST_POSTGRES_URL not configured")
	}
	store, err := OpenPostgresStore(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

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

func TestPostgresStoreRejectsEmptyURL(t *testing.T) {
	if _, err := OpenPostgresStore(""); err == nil {
		t.Fatal("expected empty postgres URL to fail")
	}
}
