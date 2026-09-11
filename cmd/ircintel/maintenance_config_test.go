package main

import (
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/core"
)

func TestMaintenanceConfigDefaultsDisabled(t *testing.T) {
	for _, key := range []string{"IRCINTEL_MAINTENANCE_ENABLED","IRCINTEL_MAINTENANCE_INTERVAL","IRCINTEL_RAW_OBSERVATION_RETENTION","IRCINTEL_HOURLY_ROLLUP_RETENTION"} { t.Setenv(key, "") }
	cfg, err := maintenanceConfigFromEnv()
	if err != nil { t.Fatal(err) }
	if cfg.Enabled { t.Fatal("maintenance unexpectedly enabled") }
	if cfg.Interval != defaultMaintenanceInterval { t.Fatalf("interval=%s", cfg.Interval) }
	if cfg.RawRetention != core.DefaultRawObservationRetention { t.Fatalf("raw=%s", cfg.RawRetention) }
	if cfg.HourlyRetention != core.DefaultHourlyRollupRetention { t.Fatalf("hourly=%s", cfg.HourlyRetention) }
}

func TestMaintenanceConfigAcceptsExplicitValues(t *testing.T) {
	t.Setenv("IRCINTEL_MAINTENANCE_ENABLED", "true")
	t.Setenv("IRCINTEL_MAINTENANCE_INTERVAL", "12h")
	t.Setenv("IRCINTEL_RAW_OBSERVATION_RETENTION", "720h")
	t.Setenv("IRCINTEL_HOURLY_ROLLUP_RETENTION", "9600h")
	cfg, err := maintenanceConfigFromEnv()
	if err != nil { t.Fatal(err) }
	if !cfg.Enabled || cfg.Interval != 12*time.Hour || cfg.RawRetention != 720*time.Hour || cfg.HourlyRetention != 9600*time.Hour { t.Fatalf("cfg=%+v", cfg) }
}

func TestMaintenanceConfigRejectsInvalidBool(t *testing.T) {
	t.Setenv("IRCINTEL_MAINTENANCE_ENABLED", "maybe")
	if _, err := maintenanceConfigFromEnv(); err == nil { t.Fatal("expected invalid boolean error") }
}

func TestMaintenanceConfigRejectsHourlyShorterThanRaw(t *testing.T) {
	t.Setenv("IRCINTEL_MAINTENANCE_ENABLED", "true")
	t.Setenv("IRCINTEL_RAW_OBSERVATION_RETENTION", "48h")
	t.Setenv("IRCINTEL_HOURLY_ROLLUP_RETENTION", "24h")
	if _, err := maintenanceConfigFromEnv(); err == nil { t.Fatal("expected retention ordering error") }
}
