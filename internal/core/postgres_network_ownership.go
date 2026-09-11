package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// EndpointNetworkOwnership records the registry identity that owned one endpoint
// during a stable time interval. The journal is append-only apart from closing
// the currently active interval when registry ownership changes.
type EndpointNetworkOwnership struct {
	EndpointID string     `json:"endpoint_id"`
	ServerID   string     `json:"server_id"`
	NetworkID  string     `json:"network_id"`
	Host       string     `json:"host"`
	Port       string     `json:"port"`
	TLS        bool       `json:"tls"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidTo    *time.Time `json:"valid_to,omitempty"`
}

func ensurePostgresNetworkOwnershipTx(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS endpoint_network_ownership (
    ownership_id BIGSERIAL PRIMARY KEY,
    endpoint_id TEXT NOT NULL,
    server_id TEXT NOT NULL,
    network_id TEXT NOT NULL,
    host TEXT NOT NULL,
    port TEXT NOT NULL,
    tls BOOLEAN NOT NULL,
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to TIMESTAMPTZ,
    CHECK(valid_to IS NULL OR valid_to > valid_from)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_endpoint_network_ownership_active
    ON endpoint_network_ownership(endpoint_id) WHERE valid_to IS NULL;
CREATE INDEX IF NOT EXISTS idx_endpoint_network_ownership_tuple_time
    ON endpoint_network_ownership(host, port, tls, valid_from, valid_to);
CREATE INDEX IF NOT EXISTS idx_endpoint_network_ownership_network_time
    ON endpoint_network_ownership(network_id, valid_from, valid_to);
CREATE TABLE IF NOT EXISTS endpoint_network_ownership_meta (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(singleton),
    bootstrapped_at TIMESTAMPTZ NOT NULL
);`); err != nil {
		return err
	}

	var bootstrapped bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM endpoint_network_ownership_meta WHERE singleton)`).Scan(&bootstrapped); err != nil {
		return err
	}
	if bootstrapped {
		return nil
	}

	// Existing registry state is the baseline for installations upgraded to
	// M4.18. Using -infinity makes pre-M4.18 observations attributable to that
	// baseline without pretending to know earlier ownership transitions.
	if _, err := tx.Exec(ctx, `
INSERT INTO endpoint_network_ownership (
    endpoint_id, server_id, network_id, host, port, tls, valid_from
)
SELECT e.id, e.server_id, s.network_id, e.host, e.port, e.tls, '-infinity'::timestamptz
FROM network_endpoints e
JOIN network_servers s ON s.id = e.server_id
ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO endpoint_network_ownership_meta(singleton, bootstrapped_at) VALUES (TRUE, now())`)
	return err
}

func syncPostgresEndpointOwnershipTx(ctx context.Context, tx pgx.Tx, endpointID string, changedAt time.Time) error {
	if endpointID == "" { return errors.New("endpoint id is required") }
	if changedAt.IsZero() { changedAt = time.Now().UTC() }
	changedAt = changedAt.UTC()

	var current EndpointNetworkOwnership
	if err := tx.QueryRow(ctx, `
SELECT e.id, e.server_id, s.network_id, e.host, e.port, e.tls
FROM network_endpoints e
JOIN network_servers s ON s.id = e.server_id
WHERE e.id = $1`, endpointID).Scan(
		&current.EndpointID, &current.ServerID, &current.NetworkID,
		&current.Host, &current.Port, &current.TLS,
	); err != nil {
		return err
	}

	var active EndpointNetworkOwnership
	err := tx.QueryRow(ctx, `
SELECT endpoint_id, server_id, network_id, host, port, tls, valid_from
FROM endpoint_network_ownership
WHERE endpoint_id = $1 AND valid_to IS NULL`, endpointID).Scan(
		&active.EndpointID, &active.ServerID, &active.NetworkID,
		&active.Host, &active.Port, &active.TLS, &active.ValidFrom,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) { return err }
	if err == nil && active.ServerID == current.ServerID && active.NetworkID == current.NetworkID && active.Host == current.Host && active.Port == current.Port && active.TLS == current.TLS {
		return nil
	}
	if err == nil {
		if !changedAt.After(active.ValidFrom) {
			changedAt = active.ValidFrom.Add(time.Microsecond)
		}
		if _, err := tx.Exec(ctx, `UPDATE endpoint_network_ownership SET valid_to = $2 WHERE endpoint_id = $1 AND valid_to IS NULL`, endpointID, changedAt); err != nil { return err }
	}
	_, err = tx.Exec(ctx, `
INSERT INTO endpoint_network_ownership (
    endpoint_id, server_id, network_id, host, port, tls, valid_from
) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		current.EndpointID, current.ServerID, current.NetworkID,
		current.Host, current.Port, current.TLS, changedAt,
	)
	return err
}

func (s *PostgresStore) EndpointNetworkOwnershipHistory(endpointID string) ([]EndpointNetworkOwnership, error) {
	if s == nil || s.pool == nil { return nil, errors.New("postgres store is not open") }
	if endpointID == "" { return nil, errors.New("endpoint id is required") }
	ctx, cancel := context.WithTimeout(context.Background(), postgresRegistryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { return nil, err }
	rows, err := tx.Query(ctx, `
SELECT endpoint_id, server_id, network_id, host, port, tls, valid_from, valid_to
FROM endpoint_network_ownership
WHERE endpoint_id = $1
ORDER BY valid_from`, endpointID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := make([]EndpointNetworkOwnership, 0)
	for rows.Next() {
		var item EndpointNetworkOwnership
		if err := rows.Scan(&item.EndpointID, &item.ServerID, &item.NetworkID, &item.Host, &item.Port, &item.TLS, &item.ValidFrom, &item.ValidTo); err != nil { return nil, err }
		out = append(out, item)
	}
	if err := rows.Err(); err != nil { return nil, err }
	if err := tx.Commit(ctx); err != nil { return nil, err }
	return out, nil
}
