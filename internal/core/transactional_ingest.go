package core

import (
	"database/sql"
	"encoding/json"
	"sort"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

// refreshIncidentRecordsTx derives and persists endpoint incidents using the
// caller's transaction. This lets observation ingestion and every derived write
// commit or roll back as one unit.
func refreshIncidentRecordsTx(tx *sql.Tx) error {
	observations, err := recentObservationsTx(tx, incidentScanLimit)
	if err != nil {
		return err
	}
	events := deriveIncidentEvents(observations)
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

func recentObservationsTx(tx *sql.Tx, limit int) ([]agent.Observation, error) {
	rows, err := tx.Query(`SELECT payload_json FROM observations ORDER BY observed_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
// persistence.
func refreshNetworkIncidentRecordsTx(tx *sql.Tx) error {
	snapshot, err := registrySnapshotTx(tx)
	if err != nil {
		return err
	}
	endpoints, err := allIncidentRecordsTx(tx)
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

func allIncidentRecordsTx(tx *sql.Tx) ([]IncidentLifecycle, error) {
	rows, err := tx.Query(`SELECT payload_json FROM incident_records ORDER BY started_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
