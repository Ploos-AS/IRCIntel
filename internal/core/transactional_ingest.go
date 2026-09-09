package core

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

// refreshIncidentRecordsTx derives and persists endpoint incidents from the
// persisted transition-event stream. M3.29 removes raw observation replay from
// the normal incident-maintenance path; observations are now only the source of
// a transition when persisted agent state actually changes.
func refreshIncidentRecordsTx(tx *sql.Tx) error {
	events, err := transitionEventsForLatestEndpointTx(tx)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	correlated := correlateIncidentEvents(events, defaultCorrelationWindow)
	lifecycles := pairIncidentLifecycle(correlated)
	for _, lifecycle := range lifecycles {
		payload, err := json.Marshal(lifecycle)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`
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
		); err != nil {
			return err
		}
	}
	return nil
}

// M3.27 observation replay helpers remain available for regression comparison
// and explicit reconstruction paths, but normal ingest no longer calls them.
func observationsForLatestEndpointTx(tx *sql.Tx) ([]agent.Observation, error) {
	var host, port string
	var tls bool
	if err := tx.QueryRow(`
SELECT endpoint_host, COALESCE(endpoint_port, ''), endpoint_tls
FROM observations
ORDER BY id DESC
LIMIT 1`).Scan(&host, &port, &tls); err != nil {
		if err == sql.ErrNoRows {
			return []agent.Observation{}, nil
		}
		return nil, err
	}

	var checkpointPayload []byte
	err := tx.QueryRow(`
SELECT payload_json
FROM incident_records
WHERE endpoint_host = ?
  AND COALESCE(endpoint_port, '') = ?
  AND endpoint_tls = ?
ORDER BY started_at DESC, id DESC
LIMIT 1`, host, port, tls).Scan(&checkpointPayload)
	if err == sql.ErrNoRows {
		return observationsForEndpointTx(tx, host, port, tls)
	}
	if err != nil {
		return nil, err
	}

	var checkpoint IncidentLifecycle
	if err := json.Unmarshal(checkpointPayload, &checkpoint); err != nil {
		return nil, err
	}
	anchor := checkpoint.StartedAt.Add(-defaultCorrelationWindow)
	return observationsForEndpointSinceTx(tx, host, port, tls, anchor)
}

func observationsForEndpointTx(tx *sql.Tx, host, port string, tls bool) ([]agent.Observation, error) {
	rows, err := tx.Query(`
SELECT payload_json
FROM observations
WHERE endpoint_host = ?
  AND COALESCE(endpoint_port, '') = ?
  AND endpoint_tls = ?
ORDER BY observed_at DESC, id DESC`, host, port, tls)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeObservationRowsTx(rows)
}

func observationsForEndpointSinceTx(tx *sql.Tx, host, port string, tls bool, anchor time.Time) ([]agent.Observation, error) {
	anchorText := formatObservationTime(anchor)
	rows, err := tx.Query(`
WITH recent_agents AS (
    SELECT DISTINCT agent_id
    FROM observations
    WHERE endpoint_host = ?
      AND COALESCE(endpoint_port, '') = ?
      AND endpoint_tls = ?
      AND observed_at >= ?
),
baseline_ranked AS (
    SELECT payload_json, observed_at, id,
           ROW_NUMBER() OVER (
               PARTITION BY agent_id
               ORDER BY observed_at DESC, id DESC
           ) AS row_number
    FROM observations
    WHERE endpoint_host = ?
      AND COALESCE(endpoint_port, '') = ?
      AND endpoint_tls = ?
      AND observed_at < ?
      AND agent_id IN (SELECT agent_id FROM recent_agents)
),
replay AS (
    SELECT payload_json, observed_at, id
    FROM baseline_ranked
    WHERE row_number = 1
    UNION ALL
    SELECT payload_json, observed_at, id
    FROM observations
    WHERE endpoint_host = ?
      AND COALESCE(endpoint_port, '') = ?
      AND endpoint_tls = ?
      AND observed_at >= ?
)
SELECT payload_json
FROM replay
ORDER BY observed_at DESC, id DESC`,
		host, port, tls, anchorText,
		host, port, tls, anchorText,
		host, port, tls, anchorText,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeObservationRowsTx(rows)
}

func recentObservationsTx(tx *sql.Tx, limit int) ([]agent.Observation, error) {
	rows, err := tx.Query(`SELECT payload_json FROM observations ORDER BY observed_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeObservationRowsTx(rows)
}

func decodeObservationRowsTx(rows *sql.Rows) ([]agent.Observation, error) {
	out := make([]agent.Observation, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var observation agent.Observation
		if err := json.Unmarshal(payload, &observation); err != nil {
			return nil, err
		}
		out = append(out, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// refreshNetworkIncidentRecordsTx performs the canonical network-level
// derivation inside the same transaction as observation and endpoint-incident
// persistence. M3.25 scopes the expensive incident-history read to networks
// containing the observation that triggered this ingest instead of rescanning
// every network's endpoint incidents on every write.
func refreshNetworkIncidentRecordsTx(tx *sql.Tx) error {
	networkIDs, err := affectedNetworkIDsForLatestObservationTx(tx)
	if err != nil {
		return err
	}
	if len(networkIDs) == 0 {
		return nil
	}

	snapshot, err := registrySnapshotTx(tx)
	if err != nil {
		return err
	}
	endpoints, err := incidentRecordsForNetworksTx(tx, networkIDs)
	if err != nil {
		return err
	}
	items := deriveNetworkIncidents(snapshot, endpoints)
	for _, item := range items {
		payload, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO network_incident_records
(network_id, started_at, status, severity, payload_json) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(network_id, started_at) DO UPDATE SET
status=excluded.status, severity=excluded.severity, payload_json=excluded.payload_json`,
			item.NetworkID, formatObservationTime(item.StartedAt), item.Status, item.Severity, payload); err != nil {
			return err
		}
	}
	return nil
}

func affectedNetworkIDsForLatestObservationTx(tx *sql.Tx) ([]string, error) {
	var host, port string
	var tls bool
	if err := tx.QueryRow(`SELECT endpoint_host, COALESCE(endpoint_port, ''), endpoint_tls FROM observations ORDER BY id DESC LIMIT 1`).Scan(&host, &port, &tls); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	rows, err := tx.Query(`
SELECT DISTINCT ns.network_id
FROM network_endpoints ne
JOIN network_servers ns ON ns.id = ne.server_id
WHERE ne.host = ? AND ne.port = ? AND ne.tls = ?
ORDER BY ns.network_id`, host, port, tls)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func incidentRecordsForNetworksTx(tx *sql.Tx, networkIDs []string) ([]IncidentLifecycle, error) {
	if len(networkIDs) == 0 {
		return []IncidentLifecycle{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(networkIDs)), ",")
	args := make([]any, len(networkIDs))
	for i, id := range networkIDs {
		args[i] = id
	}
	statement := `
SELECT ir.payload_json
FROM incident_records ir
WHERE EXISTS (
    SELECT 1
    FROM network_endpoints ne
    JOIN network_servers ns ON ns.id = ne.server_id
    WHERE ne.host = ir.endpoint_host
      AND ne.port = COALESCE(ir.endpoint_port, '')
      AND ne.tls = ir.endpoint_tls
      AND ns.network_id IN (` + placeholders + `)
)
ORDER BY ir.started_at DESC, ir.id DESC`
	rows, err := tx.Query(statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeIncidentLifecycleRows(rows)
}

func allIncidentRecordsTx(tx *sql.Tx) ([]IncidentLifecycle, error) {
	rows, err := tx.Query(`SELECT payload_json FROM incident_records ORDER BY started_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return decodeIncidentLifecycleRows(rows)
}

func decodeIncidentLifecycleRows(rows *sql.Rows) ([]IncidentLifecycle, error) {
	out := make([]IncidentLifecycle, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var item IncidentLifecycle
		if err := json.Unmarshal(payload, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func registrySnapshotTx(tx *sql.Tx) (RegistrySnapshot, error) {
	snapshot := RegistrySnapshot{
		Networks:  make([]Network, 0),
		Servers:   make([]NetworkServer, 0),
		Endpoints: make([]NetworkEndpoint, 0),
	}

	rows, err := tx.Query(`SELECT id, name, website, description FROM networks ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item Network
		if err := rows.Scan(&item.ID, &item.Name, &item.Website, &item.Description); err != nil {
			_ = rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Networks = append(snapshot.Networks, item)
	}
	if err := rows.Close(); err != nil {
		return RegistrySnapshot{}, err
	}

	rows, err = tx.Query(`SELECT id, network_id, name FROM network_servers ORDER BY network_id, name COLLATE NOCASE, id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item NetworkServer
		if err := rows.Scan(&item.ID, &item.NetworkID, &item.Name); err != nil {
			_ = rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Servers = append(snapshot.Servers, item)
	}
	if err := rows.Close(); err != nil {
		return RegistrySnapshot{}, err
	}

	rows, err = tx.Query(`SELECT id, server_id, host, port, tls FROM network_endpoints ORDER BY server_id, host, port, tls, id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item NetworkEndpoint
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Host, &item.Port, &item.TLS); err != nil {
			_ = rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Endpoints = append(snapshot.Endpoints, item)
	}
	if err := rows.Close(); err != nil {
		return RegistrySnapshot{}, err
	}

	sort.SliceStable(snapshot.Endpoints, func(i, j int) bool {
		if snapshot.Endpoints[i].ServerID != snapshot.Endpoints[j].ServerID {
			return snapshot.Endpoints[i].ServerID < snapshot.Endpoints[j].ServerID
		}
		if snapshot.Endpoints[i].Host != snapshot.Endpoints[j].Host {
			return snapshot.Endpoints[i].Host < snapshot.Endpoints[j].Host
		}
		if snapshot.Endpoints[i].Port != snapshot.Endpoints[j].Port {
			return snapshot.Endpoints[i].Port < snapshot.Endpoints[j].Port
		}
		return !snapshot.Endpoints[i].TLS && snapshot.Endpoints[j].TLS
	})
	return snapshot, nil
}
