package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

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
CREATE TABLE IF NOT EXISTS incident_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT,
    endpoint_tls INTEGER NOT NULL,
    started_at TEXT NOT NULL,
    status TEXT NOT NULL,
    payload_json BLOB NOT NULL,
    UNIQUE(endpoint_host, endpoint_port, endpoint_tls, started_at)
);
CREATE INDEX IF NOT EXISTS idx_incident_records_started
    ON incident_records(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_incident_records_status
    ON incident_records(status, started_at DESC);
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
		formatObservationTime(observation.ObservedAt),
		observation.Endpoint.Host,
		observation.Endpoint.Port,
		observation.Endpoint.TLS,
		payload,
	)
	if err != nil {
		return err
	}
	return s.refreshIncidentRecords()
}

func (s *SQLiteStore) List(query ObservationQuery) ([]agent.Observation, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite store is not open")
	}
	if query.Limit < 1 || query.Limit > maxObservationLimit {
		return nil, errors.New("invalid observation limit")
	}
	if !query.Since.IsZero() && !query.Until.IsZero() && query.Since.After(query.Until) {
		return nil, errors.New("invalid observation time range")
	}

	where := make([]string, 0, 4)
	args := make([]any, 0, 5)
	if query.AgentID != "" {
		where = append(where, "agent_id = ?")
		args = append(args, query.AgentID)
	}
	if query.Host != "" {
		where = append(where, "endpoint_host = ?")
		args = append(args, query.Host)
	}
	if !query.Since.IsZero() {
		where = append(where, "observed_at >= ?")
		args = append(args, formatObservationTime(query.Since))
	}
	if !query.Until.IsZero() {
		where = append(where, "observed_at <= ?")
		args = append(args, formatObservationTime(query.Until))
	}

	statement := "SELECT payload_json FROM observations"
	if len(where) > 0 {
		statement += " WHERE " + strings.Join(where, " AND ")
	}
	statement += " ORDER BY observed_at DESC, id DESC LIMIT ?"
	args = append(args, query.Limit)

	rows, err := s.db.Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeObservationRows(rows)
}

func (s *SQLiteStore) LatestEndpointObservations() ([]agent.Observation, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite store is not open")
	}
	rows, err := s.db.Query(`
SELECT payload_json
FROM (
    SELECT payload_json,
           ROW_NUMBER() OVER (
               PARTITION BY agent_id, endpoint_host, endpoint_port, endpoint_tls
               ORDER BY observed_at DESC, id DESC
           ) AS row_number
    FROM observations
)
WHERE row_number = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeObservationRows(rows)
}

func (s *SQLiteStore) RecentObservations(limit int) ([]agent.Observation, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite store is not open")
	}
	if limit < 1 {
		return nil, errors.New("invalid recent observation limit")
	}
	rows, err := s.db.Query(`
SELECT payload_json
FROM observations
ORDER BY observed_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeObservationRows(rows)
}

func decodeObservationRows(rows *sql.Rows) ([]agent.Observation, error) {
	observations := make([]agent.Observation, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var observation agent.Observation
		if err := json.Unmarshal(payload, &observation); err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return observations, nil
}

func formatObservationTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
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
