package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ploos-AS/IRCIntel/internal/agent"
	"github.com/jackc/pgx/v5"
)

func migratePostgresIncidentState(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS agent_endpoint_state (
    agent_id TEXT NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL,
    endpoint_tls BOOLEAN NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    reachable BOOLEAN NOT NULL,
    PRIMARY KEY(agent_id, endpoint_host, endpoint_port, endpoint_tls)
);
CREATE INDEX IF NOT EXISTS idx_pg_agent_endpoint_state_observed
    ON agent_endpoint_state(observed_at DESC);
CREATE TABLE IF NOT EXISTS endpoint_transition_events (
    agent_id TEXT NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL,
    endpoint_tls BOOLEAN NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    previous_observed_at TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL CHECK(event_type IN ('down','recovered')),
    PRIMARY KEY(agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at)
);
CREATE INDEX IF NOT EXISTS idx_pg_endpoint_transition_events_endpoint_time
    ON endpoint_transition_events(endpoint_host, endpoint_port, endpoint_tls, observed_at DESC);
CREATE TABLE IF NOT EXISTS incident_records (
    id BIGSERIAL PRIMARY KEY,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL,
    endpoint_tls BOOLEAN NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('open','closed')),
    payload_json JSONB NOT NULL,
    UNIQUE(endpoint_host, endpoint_port, endpoint_tls, started_at)
);
CREATE INDEX IF NOT EXISTS idx_pg_incident_records_status_time
    ON incident_records(status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_pg_incident_records_endpoint_time
    ON incident_records(endpoint_host, endpoint_port, endpoint_tls, started_at DESC);
`); err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `SELECT payload_json FROM observations ORDER BY observed_at ASC, id ASC`)
	if err != nil {
		return err
	}
	observations := make([]agent.Observation, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			rows.Close()
			return err
		}
		var observation agent.Observation
		if err := json.Unmarshal(payload, &observation); err != nil {
			rows.Close()
			return err
		}
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, observation := range observations {
		if err := upsertPostgresAgentEndpointStateTx(ctx, tx, observation); err != nil {
			return err
		}
	}
	for _, event := range deriveIncidentEvents(observations) {
		if err := insertPostgresTransitionEventTx(ctx, tx, event); err != nil {
			return err
		}
	}
	lifecycles := pairIncidentLifecycle(correlateIncidentEvents(deriveIncidentEvents(observations), defaultCorrelationWindow))
	for _, lifecycle := range lifecycles {
		if err := upsertPostgresIncidentLifecycleTx(ctx, tx, lifecycle); err != nil {
			return err
		}
	}
	return nil
}

func persistPostgresTransitionForObservationTx(ctx context.Context, tx pgx.Tx, observation agent.Observation) (bool, error) {
	var previousObservedAt time.Time
	var reachable bool
	err := tx.QueryRow(ctx, `
SELECT observed_at, reachable
FROM agent_endpoint_state
WHERE agent_id = $1 AND endpoint_host = $2 AND endpoint_port = $3 AND endpoint_tls = $4`,
		observation.AgentID, observation.Endpoint.Host, observation.Endpoint.Port, observation.Endpoint.TLS,
	).Scan(&previousObservedAt, &reachable)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	previousObservedAt = previousObservedAt.UTC()
	if !observation.ObservedAt.After(previousObservedAt) || reachable == observation.Result.Reachable {
		return false, nil
	}
	eventType := "down"
	if observation.Result.Reachable {
		eventType = "recovered"
	}
	event := IncidentEvent{
		Type: eventType, AgentID: observation.AgentID,
		Host: observation.Endpoint.Host, Port: observation.Endpoint.Port, TLS: observation.Endpoint.TLS,
		ObservedAt: observation.ObservedAt.UTC(), PreviousObservedAt: previousObservedAt,
	}
	if err := insertPostgresTransitionEventTx(ctx, tx, event); err != nil {
		return false, err
	}
	return true, nil
}

func insertPostgresTransitionEventTx(ctx context.Context, tx pgx.Tx, event IncidentEvent) error {
	_, err := tx.Exec(ctx, `
INSERT INTO endpoint_transition_events (
    agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at, previous_observed_at, event_type
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at) DO NOTHING`,
		event.AgentID, event.Host, event.Port, event.TLS,
		event.ObservedAt.UTC(), event.PreviousObservedAt.UTC(), event.Type,
	)
	return err
}

func upsertPostgresAgentEndpointStateTx(ctx context.Context, tx pgx.Tx, observation agent.Observation) error {
	_, err := tx.Exec(ctx, `
INSERT INTO agent_endpoint_state (
    agent_id, endpoint_host, endpoint_port, endpoint_tls, observed_at, reachable
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (agent_id, endpoint_host, endpoint_port, endpoint_tls)
DO UPDATE SET observed_at = excluded.observed_at, reachable = excluded.reachable
WHERE excluded.observed_at > agent_endpoint_state.observed_at`,
		observation.AgentID, observation.Endpoint.Host, observation.Endpoint.Port, observation.Endpoint.TLS,
		observation.ObservedAt.UTC(), observation.Result.Reachable,
	)
	return err
}

func postgresTransitionEventsForEndpointTx(ctx context.Context, tx pgx.Tx, host, port string, tls bool) ([]IncidentEvent, error) {
	var checkpoint time.Time
	err := tx.QueryRow(ctx, `
SELECT started_at
FROM incident_records
WHERE endpoint_host = $1 AND endpoint_port = $2 AND endpoint_tls = $3
ORDER BY started_at DESC, id DESC
LIMIT 1`, host, port, tls).Scan(&checkpoint)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	statement := `
SELECT agent_id, event_type, observed_at, previous_observed_at
FROM endpoint_transition_events
WHERE endpoint_host = $1 AND endpoint_port = $2 AND endpoint_tls = $3`
	args := []any{host, port, tls}
	if err == nil {
		statement += ` AND observed_at >= $4`
		args = append(args, checkpoint.UTC().Add(-defaultCorrelationWindow))
	}
	statement += ` ORDER BY observed_at DESC, agent_id`
	rows, err := tx.Query(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]IncidentEvent, 0)
	for rows.Next() {
		var event IncidentEvent
		event.Host, event.Port, event.TLS = host, port, tls
		if err := rows.Scan(&event.AgentID, &event.Type, &event.ObservedAt, &event.PreviousObservedAt); err != nil {
			return nil, err
		}
		event.ObservedAt = event.ObservedAt.UTC()
		event.PreviousObservedAt = event.PreviousObservedAt.UTC()
		out = append(out, event)
	}
	return out, rows.Err()
}

func refreshPostgresIncidentRecordsForEndpointTx(ctx context.Context, tx pgx.Tx, host, port string, tls bool) error {
	events, err := postgresTransitionEventsForEndpointTx(ctx, tx, host, port, tls)
	if err != nil {
		return err
	}
	lifecycles := pairIncidentLifecycle(correlateIncidentEvents(events, defaultCorrelationWindow))
	for _, lifecycle := range lifecycles {
		if err := upsertPostgresIncidentLifecycleTx(ctx, tx, lifecycle); err != nil {
			return err
		}
	}
	return nil
}

func upsertPostgresIncidentLifecycleTx(ctx context.Context, tx pgx.Tx, lifecycle IncidentLifecycle) error {
	payload, err := json.Marshal(lifecycle)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO incident_records (
    endpoint_host, endpoint_port, endpoint_tls, started_at, status, payload_json
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (endpoint_host, endpoint_port, endpoint_tls, started_at)
DO UPDATE SET status = excluded.status, payload_json = excluded.payload_json`,
		lifecycle.Host, lifecycle.Port, lifecycle.TLS, lifecycle.StartedAt.UTC(), lifecycle.Status, payload,
	)
	return err
}

func (s *PostgresStore) ListIncidentRecords(query IncidentRecordQuery) ([]IncidentLifecycle, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("postgres store is not open")
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
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if query.Status != "" { add("status = $%d", query.Status) }
	if query.Host != "" { add("endpoint_host = $%d", query.Host) }
	if !query.Since.IsZero() { add("started_at >= $%d", query.Since.UTC()) }
	if !query.Until.IsZero() { add("started_at <= $%d", query.Until.UTC()) }
	statement := "SELECT payload_json FROM incident_records"
	if len(where) > 0 { statement += " WHERE " + strings.Join(where, " AND ") }
	args = append(args, query.Limit)
	statement += fmt.Sprintf(" ORDER BY started_at DESC, id DESC LIMIT $%d", len(args))

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	rows, err := s.pool.Query(ctx, statement, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	out := make([]IncidentLifecycle, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil { return nil, err }
		var lifecycle IncidentLifecycle
		if err := json.Unmarshal(payload, &lifecycle); err != nil { return nil, err }
		out = append(out, lifecycle)
	}
	if err := rows.Err(); err != nil { return nil, err }
	return out, nil
}
