package main

import (
	"strings"
	"testing"
	"time"
)

func TestGetenvFallback(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_VALUE", "")
	if got := getenv("IRCINTEL_TEST_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback", got)
	}
}

func TestGetenvValue(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_VALUE", "configured")
	if got := getenv("IRCINTEL_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("got %q, want configured", got)
	}
}

func TestGetenvDurationFallback(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_DURATION", "")
	got, err := getenvDuration("IRCINTEL_TEST_DURATION", 15*time.Minute)
	if err != nil { t.Fatal(err) }
	if got != 15*time.Minute { t.Fatalf("got %s, want 15m", got) }
}

func TestGetenvDurationValue(t *testing.T) {
	t.Setenv("IRCINTEL_TEST_DURATION", "7m30s")
	got, err := getenvDuration("IRCINTEL_TEST_DURATION", 15*time.Minute)
	if err != nil { t.Fatal(err) }
	if got != 7*time.Minute+30*time.Second { t.Fatalf("got %s, want 7m30s", got) }
}

func TestGetenvDurationRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"not-a-duration", "0s", "-1m"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("IRCINTEL_TEST_DURATION", raw)
			if _, err := getenvDuration("IRCINTEL_TEST_DURATION", 15*time.Minute); err == nil {
				t.Fatalf("expected error for %q", raw)
			}
		})
	}
}

func TestStorageConfigDefaultsToSQLite(t *testing.T) {
	t.Setenv("IRCINTEL_STORAGE_BACKEND", "")
	t.Setenv("IRCINTEL_DB_PATH", "")
	t.Setenv("IRCINTEL_DATABASE_URL", "")
	cfg, err := storageConfigFromEnv()
	if err != nil { t.Fatal(err) }
	if cfg.Backend != "sqlite" { t.Fatalf("backend=%q want=sqlite", cfg.Backend) }
	if cfg.SQLitePath != "/data/ircintel.db" { t.Fatalf("path=%q", cfg.SQLitePath) }
}

func TestStorageConfigAcceptsPostgresContract(t *testing.T) {
	t.Setenv("IRCINTEL_STORAGE_BACKEND", "POSTGRES")
	t.Setenv("IRCINTEL_DATABASE_URL", "postgres://ircintel:secret@db/ircintel")
	cfg, err := storageConfigFromEnv()
	if err != nil { t.Fatal(err) }
	if cfg.Backend != "postgres" { t.Fatalf("backend=%q want=postgres", cfg.Backend) }
	if cfg.DatabaseURL == "" { t.Fatal("database URL missing") }
}

func TestStorageConfigRejectsPostgresWithoutURL(t *testing.T) {
	t.Setenv("IRCINTEL_STORAGE_BACKEND", "postgres")
	t.Setenv("IRCINTEL_DATABASE_URL", "")
	if _, err := storageConfigFromEnv(); err == nil || !strings.Contains(err.Error(), "IRCINTEL_DATABASE_URL") {
		t.Fatalf("err=%v", err)
	}
}

func TestStorageConfigRejectsDatabaseURLForSQLite(t *testing.T) {
	t.Setenv("IRCINTEL_STORAGE_BACKEND", "sqlite")
	t.Setenv("IRCINTEL_DATABASE_URL", "postgres://db/ircintel")
	if _, err := storageConfigFromEnv(); err == nil {
		t.Fatal("expected sqlite/database-url conflict")
	}
}

func TestStorageConfigRejectsUnknownBackend(t *testing.T) {
	t.Setenv("IRCINTEL_STORAGE_BACKEND", "mysql")
	t.Setenv("IRCINTEL_DATABASE_URL", "")
	if _, err := storageConfigFromEnv(); err == nil {
		t.Fatal("expected unsupported backend error")
	}
}

func TestOpenStorageFailsClosedForReservedPostgres(t *testing.T) {
	_, err := openStorage(storageConfig{Backend: "postgres", DatabaseURL: "postgres://db/ircintel"})
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("err=%v", err)
	}
}
