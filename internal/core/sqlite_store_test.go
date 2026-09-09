package core

import (
	"path/filepath"
	"testing"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
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

func TestSQLiteStoreRejectsEmptyPath(t *testing.T) {
	if _, err := OpenSQLiteStore(""); err == nil { t.Fatal("expected error") }
}

func TestSQLiteStoreSatisfiesObservationStore(t *testing.T) {
	var _ ObservationStore = (*SQLiteStore)(nil)
	_ = agent.Observation{}
}
