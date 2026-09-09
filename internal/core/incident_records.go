package core

import (
	"encoding/json"
	"errors"
)

type IncidentRecordReader interface {
	ListIncidentRecords(limit int) ([]IncidentLifecycle, error)
}

func (s *SQLiteStore) refreshIncidentRecords() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite store is not open")
	}
	observations, err := s.RecentObservations(incidentScanLimit)
	if err != nil {
		return err
	}
	events := deriveIncidentEvents(observations)
	correlated := correlateIncidentEvents(events, defaultCorrelationWindow)
	lifecycles := pairIncidentLifecycle(correlated)
	if len(lifecycles) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, lifecycle := range lifecycles {
		payload, err := json.Marshal(lifecycle)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
INSERT INTO incident_records (
    endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json
) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(endpoint_host, endpoint_port, endpoint_tls, started_at)
DO UPDATE SET status = excluded.status, payload_json = excluded.payload_json`,
			lifecycle.Host,
			lifecycle.Port,
			lifecycle.TLS,
			formatObservationTime(lifecycle.StartedAt),
			lifecycle.Status,
			payload,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListIncidentRecords(limit int) ([]IncidentLifecycle, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite store is not open")
	}
	if limit < 1 || limit > maxIncidentLimit {
		return nil, errors.New("invalid incident record limit")
	}
	rows, err := s.db.Query(`
SELECT payload_json
FROM incident_records
ORDER BY started_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	incidents := make([]IncidentLifecycle, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var incident IncidentLifecycle
		if err := json.Unmarshal(payload, &incident); err != nil {
			return nil, err
		}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return incidents, nil
}
