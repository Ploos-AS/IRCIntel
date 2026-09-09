package core

import (
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

func TestFormatObservationTimeIsFixedWidthUTC(t *testing.T) {
	value := time.Date(2026, 9, 9, 8, 0, 0, 100_000_000, time.FixedZone("offset", 2*60*60))
	if got, want := formatObservationTime(value), "2026-09-09T06:00:00.100000000Z"; got != want {
		t.Fatalf("format=%q want=%q", got, want)
	}
}

func TestSQLiteStoreOrdersFractionalObservationTimes(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	base := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	times := []time.Time{
		base,
		base.Add(100 * time.Millisecond),
		base.Add(time.Second),
	}
	for _, observedAt := range times {
		observation := agent.Observation{
			AgentID:    "oslo-1",
			ObservedAt: observedAt,
			Endpoint:   agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
			Result:     probe.EndpointResult{Reachable: true},
		}
		if err := store.Store(observation); err != nil { t.Fatal(err) }
	}

	recent, err := store.RecentObservations(3)
	if err != nil { t.Fatal(err) }
	if len(recent) != 3 { t.Fatalf("len=%d", len(recent)) }
	if !recent[0].ObservedAt.Equal(base.Add(time.Second)) || !recent[1].ObservedAt.Equal(base.Add(100*time.Millisecond)) || !recent[2].ObservedAt.Equal(base) {
		t.Fatalf("ordering=%v %v %v", recent[0].ObservedAt, recent[1].ObservedAt, recent[2].ObservedAt)
	}

	filtered, err := store.List(ObservationQuery{
		Host:  "irc.example",
		Since: base.Add(50 * time.Millisecond),
		Until: base.Add(500 * time.Millisecond),
		Limit: 10,
	})
	if err != nil { t.Fatal(err) }
	if len(filtered) != 1 || !filtered[0].ObservedAt.Equal(base.Add(100*time.Millisecond)) {
		t.Fatalf("filtered=%+v", filtered)
	}

	latest, err := store.LatestEndpointObservations()
	if err != nil { t.Fatal(err) }
	if len(latest) != 1 || !latest[0].ObservedAt.Equal(base.Add(time.Second)) {
		t.Fatalf("latest=%+v", latest)
	}
}

func TestSQLiteStoreNormalizesLegacyRFC3339NanoTimesOnOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ircintel.db")
	store, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }

	observedAt := time.Date(2026, 9, 9, 8, 0, 0, 100_000_000, time.UTC)
	observation := agent.Observation{
		AgentID:    "oslo-1",
		ObservedAt: observedAt,
		Endpoint:   agent.Endpoint{Host: "irc.example", Port: "6697", TLS: true},
		Result:     probe.EndpointResult{Reachable: true},
	}
	if err := store.Store(observation); err != nil { t.Fatal(err) }
	if _, err := store.db.Exec(`UPDATE observations SET observed_at = ?`, observedAt.Format(time.RFC3339Nano)); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }

	reopened, err := OpenSQLiteStore(path)
	if err != nil { t.Fatal(err) }
	defer reopened.Close()
	var stored string
	if err := reopened.db.QueryRow(`SELECT observed_at FROM observations LIMIT 1`).Scan(&stored); err != nil { t.Fatal(err) }
	if want := "2026-09-09T08:00:00.100000000Z"; stored != want {
		t.Fatalf("stored=%q want=%q", stored, want)
	}
}

func TestSQLiteStoreRejectsEmptyPath(t *testing.T) {
	if _, err := OpenSQLiteStore(""); err == nil { t.Fatal("expected error") }
}

func TestSQLiteStoreSatisfiesObservationStore(t *testing.T) {
	var _ ObservationStore = (*SQLiteStore)(nil)
	_ = agent.Observation{}
}
