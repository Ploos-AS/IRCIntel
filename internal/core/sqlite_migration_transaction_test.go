package core

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestSQLiteMigrationRollbackPreservesSchemaVersion(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ircintel.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	var before int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&before); err != nil { t.Fatal(err) }

	boom := errors.New("migration failed")
	err = store.runMigration(before+1, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE migration_should_rollback (id INTEGER PRIMARY KEY)`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) { t.Fatalf("error=%v want=%v", err, boom) }

	var after int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&after); err != nil { t.Fatal(err) }
	if after != before { t.Fatalf("user_version=%d want=%d", after, before) }

	var tables int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migration_should_rollback'`).Scan(&tables); err != nil { t.Fatal(err) }
	if tables != 0 { t.Fatalf("rolled-back table count=%d want=0", tables) }
}
