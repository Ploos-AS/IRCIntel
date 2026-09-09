package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
)

func migrateEndpointTransitionEventsTx(tx *sql.Tx) error {
	if tx == nil {
		return errors.New("sqlite transaction is required")
	}
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS endpoint_transition_events (
    agent_id TEXT NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL,
    endpoint_tls INTEGER NOT NULL,
    observed_at TEXT NOT NULL,
    previous_observed_at TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK(event_type IN ('down','recovered')),
    PRIMARY KEY(agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at)
);
CREATE INDEX IF NOT EXISTS idx_endpoint_transition_events_endpoint_time
    ON endpoint_transition_events(endpoint_host, endpoint_port, endpoint_tls, observed_at DESC);`); err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT payload_json FROM observations ORDER BY observed_at DESC, id DESC`)
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
	for _, event := range deriveIncidentEvents(observations) {
		if err := insertEndpointTransitionEventTx(tx, event); err != nil {
			return err
		}
	}
	return nil
}

func persistTransitionForObservationTx(tx *sql.Tx, observation agent.Observation) (bool, error) {
	if tx == nil {
		return false, errors.New("sqlite transaction is required")
	}
	var observedAt string
	var reachable bool
	err := tx.QueryRow(`
SELECT observed_at, reachable
FROM agent_endpoint_state
WHERE agent_id = ? AND endpoint_host = ? AND endpoint_port = ? AND endpoint_tls = ?`,
		observation.AgentID,
		observation.Endpoint.Host,
		observation.Endpoint.Port,
		observation.Endpoint.TLS,
	).Scan(&observedAt, &reachable)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	previousObservedAt, err := time.Parse(observationTimeLayout, observedAt)
	if err != nil {
		return false, err
	}
	if !observation.ObservedAt.After(previousObservedAt) || reachable == observation.Result.Reachable {
		return false, nil
	}
	eventType := "down"
	if observation.Result.Reachable {
		eventType = "recovered"
	}
	event := IncidentEvent{
		Type:               eventType,
		AgentID:            observation.AgentID,
		Host:               observation.Endpoint.Host,
		Port:               observation.Endpoint.Port,
		TLS:                observation.Endpoint.TLS,
		ObservedAt:         observation.ObservedAt,
		PreviousObservedAt: previousObservedAt,
	}
	if err := insertEndpointTransitionEventTx(tx, event); err != nil {
		return false, err
	}
	return true, nil
}

func insertEndpointTransitionEventTx(tx *sql.Tx, event IncidentEvent) error {
	_, err := tx.Exec(`
INSERT OR IGNORE INTO endpoint_transition_events (
    agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at, previous_observed_at, event_type
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.AgentID,
		event.Host,
		event.Port,
		event.TLS,
		formatObservationTime(event.ObservedAt),
		formatObservationTime(event.PreviousObservedAt),
		event.Type,
	)
	return err
}

func transitionEventsForLatestEndpointTx(tx *sql.Tx) ([]IncidentEvent, error) {
	var host, port string
	var tls bool
	if err := tx.QueryRow(`
SELECT endpoint_host, COALESCE(endpoint_port, ''), endpoint_tls
FROM observations
ORDER BY id DESC
LIMIT 1`).Scan(&host, &port, &tls); err != nil {
		if err == sql.ErrNoRows {
			return []IncidentEvent{}, nil
		}
		return nil, err
	}
	rows, err := tx.Query(`
SELECT agent_id, event_type, observed_at, previous_observed_at
FROM endpoint_transition_events
WHERE endpoint_host = ? AND endpoint_port = ? AND endpoint_tls = ?
ORDER BY observed_at DESC, agent_id`, host, port, tls)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]IncidentEvent, 0)
	for rows.Next() {
		var event IncidentEvent
		var observedAt, previousObservedAt string
		event.Host = host
		event.Port = port
		event.TLS = tls
		if err := rows.Scan(&event.AgentID, &event.Type, &observedAt, &previousObservedAt); err != nil {
			return nil, err
		}
		var err error
		event.ObservedAt, err = time.Parse(observationTimeLayout, observedAt)
		if err != nil {
			return nil, err
		}
		event.PreviousObservedAt, err = time.Parse(observationTimeLayout, previousObservedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
