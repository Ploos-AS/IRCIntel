package core

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

type AgentEndpointState struct {
	AgentID    string
	Host       string
	Port       string
	TLS        bool
	ObservedAt string
	Reachable  bool
}

func migrateAgentEndpointStateTx(tx *sql.Tx) error {
	if tx == nil {
		return errors.New("sqlite transaction is required")
	}
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS agent_endpoint_state (
    agent_id TEXT NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL,
    endpoint_tls INTEGER NOT NULL,
    observed_at TEXT NOT NULL,
    reachable INTEGER NOT NULL,
    PRIMARY KEY(agent_id, endpoint_host, endpoint_port, endpoint_tls)
);
CREATE INDEX IF NOT EXISTS idx_agent_endpoint_state_observed
    ON agent_endpoint_state(observed_at DESC);`); err != nil {
		return err
	}

	rows, err := tx.Query(`
SELECT payload_json
FROM (
    SELECT payload_json,
           ROW_NUMBER() OVER (
               PARTITION BY agent_id, endpoint_host, COALESCE(endpoint_port, ''), endpoint_tls
               ORDER BY observed_at DESC, id DESC
           ) AS row_number
    FROM observations
)
WHERE row_number = 1`)
	if err != nil {
		return err
	}
	observations := make([]agent.Observation, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			_ = rows.Close()
			return err
		}
		var observation agent.Observation
		if err := json.Unmarshal(payload, &observation); err != nil {
			_ = rows.Close()
			return err
		}
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, observation := range observations {
		if err := upsertAgentEndpointStateTx(tx, observation); err != nil {
			return err
		}
	}
	return nil
}

func upsertAgentEndpointStateTx(tx *sql.Tx, observation agent.Observation) error {
	if tx == nil {
		return errors.New("sqlite transaction is required")
	}
	_, err := tx.Exec(`
INSERT INTO agent_endpoint_state (
    agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at, reachable
) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(agent_id, endpoint_host, endpoint_port, endpoint_tls)
DO UPDATE SET observed_at = excluded.observed_at, reachable = excluded.reachable
WHERE excluded.observed_at > agent_endpoint_state.observed_at`,
		observation.AgentID,
		observation.Endpoint.Host,
		observation.Endpoint.Port,
		observation.Endpoint.TLS,
		formatObservationTime(observation.ObservedAt),
		observation.Result.Reachable,
	)
	return err
}
