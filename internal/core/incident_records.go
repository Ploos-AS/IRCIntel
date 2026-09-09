package core

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type IncidentRecordQuery struct {
	Status string
	Host   string
	Since  time.Time
	Until  time.Time
	Limit  int
}

type IncidentRecordReader interface {
	ListIncidentRecords(IncidentRecordQuery) ([]IncidentLifecycle, error)
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

func (s *SQLiteStore) ListIncidentRecords(query IncidentRecordQuery) ([]IncidentLifecycle, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite store is not open")
	}
	if query.Limit < 1 || query.Limit > maxIncidentLimit {
		return nil, errors.New("invalid incident record limit")
	}
	if query.Status != "" && query.Status != "open" && query.Status != "closed" {
		return nil, errors.New("invalid incident status")
	}
	if !query.Since.IsZero() && !query.Until.IsZero() && query.Since.After(query.Until) {
		return nil, errors.New("invalid incident time range")
	}

	where := make([]string, 0, 4)
	args := make([]any, 0, 5)
	if query.Status != "" {
		where = append(where, "status = ?")
		args = append(args, query.Status)
	}
	if query.Host != "" {
		where = append(where, "endpoint_host = ?")
		args = append(args, query.Host)
	}
	if !query.Since.IsZero() {
		where = append(where, "started_at >= ?")
		args = append(args, formatObservationTime(query.Since))
	}
	if !query.Until.IsZero() {
		where = append(where, "started_at <= ?")
		args = append(args, formatObservationTime(query.Until))
	}

	statement := "SELECT payload_json FROM incident_records"
	if len(where) > 0 {
		statement += " WHERE " + strings.Join(where, " AND ")
	}
	statement += " ORDER BY started_at DESC, id DESC LIMIT ?"
	args = append(args, query.Limit)

	rows, err := s.db.Query(statement, args...)
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
