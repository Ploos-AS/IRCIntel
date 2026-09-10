package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func migratePostgresNetworkIncidents(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS network_incident_records (
    id BIGSERIAL PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE RESTRICT,
    started_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('open','closed')),
    severity TEXT NOT NULL CHECK(severity IN ('degraded','down')),
    payload_json JSONB NOT NULL,
    UNIQUE(network_id, started_at)
);
CREATE INDEX IF NOT EXISTS idx_pg_network_incident_records_status_time
    ON network_incident_records(status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_pg_network_incident_records_network_time
    ON network_incident_records(network_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_pg_network_incident_records_severity_time
    ON network_incident_records(severity, started_at DESC);`)
	if err != nil {
		return err
	}

	snapshot, err := postgresRegistrySnapshotTx(ctx, tx)
	if err != nil {
		return err
	}
	endpoints, err := postgresAllIncidentRecordsTx(ctx, tx)
	if err != nil {
		return err
	}
	for _, item := range deriveNetworkIncidents(snapshot, endpoints) {
		if err := upsertPostgresNetworkIncidentTx(ctx, tx, item); err != nil {
			return err
		}
	}
	return nil
}

func refreshPostgresNetworkIncidentRecordsTx(ctx context.Context, tx pgx.Tx) error {
	snapshot, err := postgresRegistrySnapshotTx(ctx, tx)
	if err != nil {
		return err
	}
	endpoints, err := postgresAllIncidentRecordsTx(ctx, tx)
	if err != nil {
		return err
	}
	for _, item := range deriveNetworkIncidents(snapshot, endpoints) {
		if err := upsertPostgresNetworkIncidentTx(ctx, tx, item); err != nil {
			return err
		}
	}
	return nil
}

func postgresRegistrySnapshotTx(ctx context.Context, tx pgx.Tx) (RegistrySnapshot, error) {
	snapshot := RegistrySnapshot{
		Networks: make([]Network, 0), Servers: make([]NetworkServer, 0), Endpoints: make([]NetworkEndpoint, 0),
	}

	rows, err := tx.Query(ctx, `SELECT id, name, website, description FROM networks ORDER BY lower(name), id`)
	if err != nil { return RegistrySnapshot{}, err }
	for rows.Next() {
		var item Network
		if err := rows.Scan(&item.ID, &item.Name, &item.Website, &item.Description); err != nil { rows.Close(); return RegistrySnapshot{}, err }
		snapshot.Networks = append(snapshot.Networks, item)
	}
	if err := rows.Err(); err != nil { rows.Close(); return RegistrySnapshot{}, err }
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id, network_id, name FROM network_servers ORDER BY network_id, lower(name), id`)
	if err != nil { return RegistrySnapshot{}, err }
	for rows.Next() {
		var item NetworkServer
		if err := rows.Scan(&item.ID, &item.NetworkID, &item.Name); err != nil { rows.Close(); return RegistrySnapshot{}, err }
		snapshot.Servers = append(snapshot.Servers, item)
	}
	if err := rows.Err(); err != nil { rows.Close(); return RegistrySnapshot{}, err }
	rows.Close()

	rows, err = tx.Query(ctx, `SELECT id, server_id, host, port, tls FROM network_endpoints ORDER BY server_id, host, port, tls, id`)
	if err != nil { return RegistrySnapshot{}, err }
	for rows.Next() {
		var item NetworkEndpoint
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Host, &item.Port, &item.TLS); err != nil { rows.Close(); return RegistrySnapshot{}, err }
		snapshot.Endpoints = append(snapshot.Endpoints, item)
	}
	if err := rows.Err(); err != nil { rows.Close(); return RegistrySnapshot{}, err }
	rows.Close()
	return snapshot, nil
}

func postgresAllIncidentRecordsTx(ctx context.Context, tx pgx.Tx) ([]IncidentLifecycle, error) {
	rows, err := tx.Query(ctx, `SELECT payload_json FROM incident_records ORDER BY started_at DESC, id DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	out := make([]IncidentLifecycle, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil { return nil, err }
		var item IncidentLifecycle
		if err := json.Unmarshal(payload, &item); err != nil { return nil, err }
		out = append(out, item)
	}
	if err := rows.Err(); err != nil { return nil, err }
	return out, nil
}

func upsertPostgresNetworkIncidentTx(ctx context.Context, tx pgx.Tx, item NetworkIncident) error {
	payload, err := json.Marshal(item)
	if err != nil { return err }
	_, err = tx.Exec(ctx, `
INSERT INTO network_incident_records (network_id, started_at, status, severity, payload_json)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (network_id, started_at) DO UPDATE SET
    status = excluded.status,
    severity = excluded.severity,
    payload_json = excluded.payload_json`,
		item.NetworkID, item.StartedAt.UTC(), item.Status, item.Severity, payload)
	return err
}

func (s *PostgresStore) ListNetworkIncidentRecords(query NetworkIncidentRecordQuery) ([]NetworkIncident, error) {
	if s == nil || s.pool == nil { return nil, errors.New("postgres store is not open") }
	if query.Limit < 1 || query.Limit > 500 { return nil, errors.New("invalid network incident limit") }
	if query.Status != "" && query.Status != "open" && query.Status != "closed" { return nil, errors.New("invalid network incident status") }
	if query.Severity != "" && query.Severity != "degraded" && query.Severity != "down" { return nil, errors.New("invalid network incident severity") }
	if !query.Since.IsZero() && !query.Until.IsZero() && query.Since.After(query.Until) { return nil, errors.New("invalid network incident time range") }

	where := make([]string, 0, 5)
	args := make([]any, 0, 6)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if query.Status != "" { add("status = $%d", query.Status) }
	if query.Severity != "" { add("severity = $%d", query.Severity) }
	if query.NetworkID != "" { add("network_id = $%d", query.NetworkID) }
	if !query.Since.IsZero() { add("started_at >= $%d", query.Since.UTC()) }
	if !query.Until.IsZero() { add("started_at <= $%d", query.Until.UTC()) }
	statement := "SELECT payload_json FROM network_incident_records"
	if len(where) > 0 { statement += " WHERE " + strings.Join(where, " AND ") }
	args = append(args, query.Limit)
	statement += fmt.Sprintf(" ORDER BY started_at DESC, id DESC LIMIT $%d", len(args))

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	rows, err := s.pool.Query(ctx, statement, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	out := make([]NetworkIncident, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil { return nil, err }
		var item NetworkIncident
		if err := json.Unmarshal(payload, &item); err != nil { return nil, err }
		out = append(out, item)
	}
	if err := rows.Err(); err != nil { return nil, err }
	return out, nil
}

var _ NetworkIncidentRecordReader = (*PostgresStore)(nil)
var _ = time.Time{}
