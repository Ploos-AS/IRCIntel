package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/Ploos-AS/IRCIntel/internal/probe"
)

func TestSQLiteStorePersistsObservations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	if err := store.Store(validObservation()); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	reopened, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()
	count, err := reopened.Count()
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("count=%d", count) }
}

func TestSQLiteStoreDeduplicatesObservationRetries(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	observation := agent.Observation{
		AgentID: "oslo-1",
		ObservedAt: time.Date(2026, 9, 9, 12, 0, 0, 123_000_000, time.UTC),
		Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
		Result: probe.EndpointResult{Reachable: true},
	}
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	count, err := store.Count()
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("count=%d want=1", count) }
}

func TestFormatObservationTimeIsFixedWidthUTC(t *testing.T) {
	value := time.Date(2026, 9, 9, 8, 0, 0, 100_000_000, time.FixedZone("offset", 2*60*60))
	if got, want := formatObservationTime(value), "2026-09-09T06:00:00.100000000Z"; got != want { t.Fatalf("format=%q want=%q", got, want) }
}

func TestSQLiteStoreOrdersFractionalObservationTimes(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	base := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	times := []time.Time{base, base.Add(100 * time.Millisecond), base.Add(time.Second)}
	for _, observedAt := range times {
		observation := agent.Observation{AgentID: "oslo-1", ObservedAt: observedAt, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}}
		if err := store.Store(observation); err != nil { t.Fatal(err) }
	}
	recent, err := store.RecentObservations(3)
	if err != nil { t.Fatal(err) }
	if len(recent) != 3 { t.Fatalf("len=%d", len(recent)) }
	if !recent[0].ObservedAt.Equal(base.Add(time.Second)) || !recent[1].ObservedAt.Equal(base.Add(100*time.Millisecond)) || !recent[2].ObservedAt.Equal(base) { t.Fatalf("ordering=%v %v %v", recent[0].ObservedAt, recent[1].ObservedAt, recent[2].ObservedAt) }
	filtered, err := store.List(ObservationQuery{Host: "irc.example", Since: base.Add(50 * time.Millisecond), Until: base.Add(500 * time.Millisecond), Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(filtered) != 1 || !filtered[0].ObservedAt.Equal(base.Add(100*time.Millisecond)) { t.Fatalf("filtered=%+v", filtered) }
	latest, err := store.LatestEndpointObservations()
	if err != nil { t.Fatal(err) }
	if len(latest) != 1 || !latest[0].ObservedAt.Equal(base.Add(time.Second)) { t.Fatalf("latest=%+v", latest) }
}

func TestSQLiteStoreNormalizesLegacyRFC3339NanoTimesOnV0Migration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	observedAt := time.Date(2026, 9, 9, 8, 0, 0, 100_000_000, time.UTC)
	observation := agent.Observation{AgentID: "oslo-1", ObservedAt: observedAt, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}}
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`UPDATE observations SET observed_at = ?`, observedAt.Format(time.RFC3339Nano)); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`PRAGMA user_version = 0`); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }
	reopened, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()
	var stored string
	if err := reopened.db.QueryRow(`SELECT observed_at FROM observations LIMIT 1`).Scan(&stored); err != nil { t.Fatal(err) }
	if want := "2026-09-09T08:00:00.100000000Z"; stored != want { t.Fatalf("stored=%q want=%q", stored, want) }
	var version int
	if err := reopened.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil { t.Fatal(err) }
	if version != sqliteSchemaVersion { t.Fatalf("user_version=%d want=%d", version, sqliteSchemaVersion) }
}

func TestSQLiteStoreDoesNotRepeatV1DataMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	observedAt := time.Date(2026, 9, 9, 8, 0, 0, 100_000_000, time.UTC)
	observation := agent.Observation{AgentID: "oslo-1", ObservedAt: observedAt, Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true}, Result: probe.EndpointResult{Reachable: true}}
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	legacy := observedAt.Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`UPDATE observations SET observed_at = ?`, legacy); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }
	reopened, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()
	var stored string
	if err := reopened.db.QueryRow(`SELECT observed_at FROM observations LIMIT 1`).Scan(&stored); err != nil { t.Fatal(err) }
	if stored != legacy { t.Fatalf("v1 database was unexpectedly rescanned: stored=%q want=%q", stored, legacy) }
}

func TestSQLiteStoreV1MigrationCreatesDiscoverySchemas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	for _, table := range []string{"discovery_candidates", "discovery_reviews", "discovery_promotions"} {
		if _, err := store.db.Exec(`DROP TABLE IF EXISTS ` + table); err != nil { t.Fatal(err) }
	}
	if _, err := store.db.Exec(`DROP INDEX IF EXISTS idx_observations_identity`); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`PRAGMA user_version = 1`); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	reopened, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()
	var version int
	if err := reopened.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil { t.Fatal(err) }
	if version != sqliteSchemaVersion { t.Fatalf("user_version=%d want=%d", version, sqliteSchemaVersion) }
	for _, table := range []string{"discovery_candidates", "discovery_reviews", "discovery_promotions"} {
		var count int
		if err := reopened.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil { t.Fatal(err) }
		if count != 1 { t.Fatalf("table %s missing after migration", table) }
	}
}

func TestSQLiteStoreV2ToV3DeduplicatesLegacyObservations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	observation := agent.Observation{
		AgentID: "oslo-1",
		ObservedAt: time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC),
		Endpoint: agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
		Result: probe.EndpointResult{Reachable: true},
	}
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	payload, err := json.Marshal(observation)
	if err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`DROP INDEX idx_observations_identity`); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`INSERT INTO observations (agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json) VALUES (?, ?, ?, ?, ?, ?)`, observation.AgentID, formatObservationTime(observation.ObservedAt), observation.Endpoint.Host, observation.Endpoint.Port, observation.Endpoint.TLS, payload); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`PRAGMA user_version = 2`); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	reopened, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()
	count, err := reopened.Count()
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("count=%d want=1", count) }
	var version int
	if err := reopened.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil { t.Fatal(err) }
	if version != 3 { t.Fatalf("user_version=%d want=3", version) }
	if err := reopened.Store(observation); err != nil { t.Fatal(err) }
	count, err = reopened.Count()
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("count after retry=%d want=1", count) }
}

func TestSQLiteStoreRejectsFutureSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`PRAGMA user_version = 999`); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }
	if _, err := OpenSQLiteStore(path); err == nil { t.Fatal("expected future schema version error") }
}

func TestSQLiteStoreRejectsEmptyPath(t *testing.T) { if _, err := OpenSQLiteStore(""); err == nil { t.Fatal("expected error") } }
func TestSQLiteStoreSatisfiesObservationStore(t *testing.T) { var _ ObservationStore = (*SQLiteStore)(nil); _ = agent.Observation{} }
