package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, errors.New("sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    observed_at TEXT NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT,
    endpoint_tls INTEGER NOT NULL,
    payload_json BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_observations_agent_time
    ON observations(agent_id, observed_at);
CREATE INDEX IF NOT EXISTS idx_observations_endpoint_time
    ON observations(endpoint_host, endpoint_port, observed_at);
`)
	return err
}

func (s *SQLiteStore) Store(observation agent.Observation) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	payload, err := json.Marshal(observation)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO observations (
    agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls, payload_json
) VALUES (?, ?, ?, ?, ?, ?)`,
		observation.AgentID,
		observation.ObservedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		observation.Endpoint.Host,
		observation.Endpoint.Port,
		observation.Endpoint.TLS,
		payload,
	)
	return err
}

func (s *SQLiteStore) Count() (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("sqlite store is not open")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
