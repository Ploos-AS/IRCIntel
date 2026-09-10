package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *PostgresStore) NetworkIncidentStatsSnapshot(since, until time.Time) (RegistrySnapshot, []NetworkIncident, error) {
	if s == nil || s.pool == nil {
		return RegistrySnapshot{}, nil, errors.New("postgres store is not open")
	}
	if !since.IsZero() && !until.IsZero() && since.After(until) {
		return RegistrySnapshot{}, nil, errors.New("invalid network incident time range")
	}

	snapshot, err := s.RegistrySnapshot()
	if err != nil {
		return RegistrySnapshot{}, nil, err
	}

	where := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if !since.IsZero() {
		args = append(args, since.UTC())
		where = append(where, "started_at >= $1")
	}
	if !until.IsZero() {
		args = append(args, until.UTC())
		where = append(where, "started_at <= $"+string(rune('0'+len(args))))
	}
	statement := "SELECT payload_json FROM network_incident_records"
	if len(where) > 0 {
		statement += " WHERE " + strings.Join(where, " AND ")
	}
	statement += " ORDER BY started_at DESC, id DESC"

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	rows, err := s.pool.Query(ctx, statement, args...)
	if err != nil {
		return RegistrySnapshot{}, nil, err
	}
	defer rows.Close()

	incidents := make([]NetworkIncident, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return RegistrySnapshot{}, nil, err
		}
		var incident NetworkIncident
		if err := json.Unmarshal(payload, &incident); err != nil {
			return RegistrySnapshot{}, nil, err
		}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return RegistrySnapshot{}, nil, err
	}
	return snapshot, incidents, nil
}

var _ NetworkIncidentStatsSnapshotReader = (*PostgresStore)(nil)
