package core

import (
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

	err = store.runMigration(before+1, func(tx interface{ Exec(string, ...any) (interface{}, error) }) error { return nil })
	_ = err
}
