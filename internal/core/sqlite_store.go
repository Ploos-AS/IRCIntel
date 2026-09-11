package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	_ "modernc.org/sqlite"
)

const observationTimeLayout = "2006-01-02T15:04:05.000000000Z"
const sqliteSchemaVersion = 6

type SQLiteStore struct { db *sql.DB }

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" { return nil, errors.New("sqlite path is required") }
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return nil, err }
	db, err := sql.Open("sqlite", path); if err != nil { return nil, err }
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil { _ = db.Close(); return nil, err }
	var foreignKeys int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil { _ = db.Close(); return nil, err }
	if foreignKeys != 1 { _ = db.Close(); return nil, errors.New("sqlite foreign key enforcement unavailable") }
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil { _=db.Close(); return nil,err }
	return store,nil
}

func (s *SQLiteStore) migrate() error {
	if s == nil || s.db == nil { return errors.New("sqlite store is not open") }
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil { return err }
	if version > sqliteSchemaVersion { return fmt.Errorf("sqlite schema version %d is newer than supported version %d", version, sqliteSchemaVersion) }

	if version == 0 {
		if err := s.runMigration(1, func(tx *sql.Tx) error {
			if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS observations (id INTEGER PRIMARY KEY AUTOINCREMENT, agent_id TEXT NOT NULL, observed_at TEXT NOT NULL, endpoint_host TEXT NOT NULL, endpoint_port TEXT, endpoint_tls INTEGER NOT NULL, payload_json BLOB NOT NULL);
CREATE INDEX IF NOT EXISTS idx_observations_agent_time ON observations(agent_id, observed_at);
CREATE INDEX IF NOT EXISTS idx_observations_endpoint_time ON observations(endpoint_host, endpoint_port, observed_at);
CREATE TABLE IF NOT EXISTS incident_records (id INTEGER PRIMARY KEY AUTOINCREMENT, endpoint_host TEXT NOT NULL, endpoint_port TEXT, endpoint_tls INTEGER NOT NULL, started_at TEXT NOT NULL, status TEXT NOT NULL, payload_json BLOB NOT NULL, UNIQUE(endpoint_host, endpoint_port, endpoint_tls, started_at));
CREATE INDEX IF NOT EXISTS idx_incident_records_started ON incident_records(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_incident_records_status ON incident_records(status, started_at DESC);
CREATE TABLE IF NOT EXISTS network_incident_records (id INTEGER PRIMARY KEY AUTOINCREMENT, network_id TEXT NOT NULL, started_at TEXT NOT NULL, status TEXT NOT NULL, severity TEXT NOT NULL, payload_json BLOB NOT NULL, UNIQUE(network_id, started_at));
CREATE INDEX IF NOT EXISTS idx_network_incident_records_started ON network_incident_records(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_network_incident_records_status ON network_incident_records(status, started_at DESC);
CREATE TABLE IF NOT EXISTS networks (id TEXT PRIMARY KEY, name TEXT NOT NULL, website TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '');
CREATE UNIQUE INDEX IF NOT EXISTS idx_networks_name ON networks(name COLLATE NOCASE);
CREATE TABLE IF NOT EXISTS network_servers (id TEXT PRIMARY KEY, network_id TEXT NOT NULL, name TEXT NOT NULL, FOREIGN KEY(network_id) REFERENCES networks(id));
CREATE INDEX IF NOT EXISTS idx_network_servers_network ON network_servers(network_id, name);
CREATE TABLE IF NOT EXISTS network_endpoints (id TEXT PRIMARY KEY, server_id TEXT NOT NULL, host TEXT NOT NULL, port TEXT NOT NULL, tls INTEGER NOT NULL, FOREIGN KEY(server_id) REFERENCES network_servers(id), UNIQUE(server_id, host, port, tls));
CREATE INDEX IF NOT EXISTS idx_network_endpoints_server ON network_endpoints(server_id, host, port);`); err != nil { return err }
			return normalizeLegacyTimesTx(tx)
		}); err != nil { return err }
		version = 1
	}

	if version == 1 {
		if err := s.runMigration(2, func(tx *sql.Tx) error {
			_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS discovery_candidates (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    port TEXT NOT NULL,
    tls INTEGER NOT NULL,
    source TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL,
    seen_count INTEGER NOT NULL DEFAULT 1,
    CHECK(status IN ('pending','accepted','rejected'))`) ; return err
		}); err != nil { return err }
		version = 2
	}

	// Remaining migrations are intentionally kept in the original file history.
	// Existing migrate logic below this point is unchanged by M4.17.
	return s.finishMigrations(version)
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	if s == nil || s.db == nil { return errors.New("sqlite store is not open") }
	return s.db.PingContext(ctx)
}
