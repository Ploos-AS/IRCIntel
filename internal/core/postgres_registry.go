package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

const postgresRegistryTimeout = 5 * time.Second

func (s *PostgresStore) UpsertNetwork(network Network) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres store is not open")
	}
	network.ID = strings.TrimSpace(network.ID)
	network.Name = strings.TrimSpace(network.Name)
	if network.ID == "" || network.Name == "" {
		return errors.New("network id and name are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), postgresRegistryTimeout)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
INSERT INTO networks (id, name, website, description)
VALUES ($1, $2, $3, $4)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    website = excluded.website,
    description = excluded.description`,
		network.ID, network.Name, strings.TrimSpace(network.Website), strings.TrimSpace(network.Description))
	return err
}

func (s *PostgresStore) UpsertNetworkServer(server NetworkServer) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres store is not open")
	}
	server.ID = strings.TrimSpace(server.ID)
	server.NetworkID = strings.TrimSpace(server.NetworkID)
	server.Name = strings.TrimSpace(server.Name)
	if server.ID == "" || server.NetworkID == "" || server.Name == "" {
		return errors.New("server id, network id, and name are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), postgresRegistryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { return err }

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM networks WHERE id = $1)`, server.NetworkID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrRegistryNetworkNotFound
	}
	changedAt := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
INSERT INTO network_servers (id, network_id, name)
VALUES ($1, $2, $3)
ON CONFLICT(id) DO UPDATE SET
    network_id = excluded.network_id,
    name = excluded.name`, server.ID, server.NetworkID, server.Name); err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `SELECT id FROM network_endpoints WHERE server_id = $1 ORDER BY id`, server.ID)
	if err != nil { return err }
	endpointIDs := make([]string, 0)
	for rows.Next() {
		var endpointID string
		if err := rows.Scan(&endpointID); err != nil { rows.Close(); return err }
		endpointIDs = append(endpointIDs, endpointID)
	}
	if err := rows.Err(); err != nil { rows.Close(); return err }
	rows.Close()
	for _, endpointID := range endpointIDs {
		if err := syncPostgresEndpointOwnershipTx(ctx, tx, endpointID, changedAt); err != nil { return err }
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpsertNetworkEndpoint(endpoint NetworkEndpoint) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres store is not open")
	}
	endpoint.ID = strings.TrimSpace(endpoint.ID)
	endpoint.ServerID = strings.TrimSpace(endpoint.ServerID)
	endpoint.Host = strings.ToLower(strings.TrimSpace(endpoint.Host))
	endpoint.Port = strings.TrimSpace(endpoint.Port)
	if endpoint.ID == "" || endpoint.ServerID == "" || endpoint.Host == "" || endpoint.Port == "" {
		return errors.New("endpoint id, server id, host, and port are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), postgresRegistryTimeout)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	if err := ensurePostgresNetworkOwnershipTx(ctx, tx); err != nil { return err }

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM network_servers WHERE id = $1)`, endpoint.ServerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrRegistryServerNotFound
	}
	changedAt := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
INSERT INTO network_endpoints (id, server_id, host, port, tls)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT(id) DO UPDATE SET
    server_id = excluded.server_id,
    host = excluded.host,
    port = excluded.port,
    tls = excluded.tls`, endpoint.ID, endpoint.ServerID, endpoint.Host, endpoint.Port, endpoint.TLS); err != nil {
		return err
	}
	if err := syncPostgresEndpointOwnershipTx(ctx, tx, endpoint.ID, changedAt); err != nil { return err }
	return tx.Commit(ctx)
}

func (s *PostgresStore) RegistrySnapshot() (RegistrySnapshot, error) {
	if s == nil || s.pool == nil {
		return RegistrySnapshot{}, errors.New("postgres store is not open")
	}
	ctx, cancel := context.WithTimeout(context.Background(), postgresRegistryTimeout)
	defer cancel()
	snapshot := RegistrySnapshot{
		Networks:  make([]Network, 0),
		Servers:   make([]NetworkServer, 0),
		Endpoints: make([]NetworkEndpoint, 0),
	}

	rows, err := s.pool.Query(ctx, `SELECT id, name, website, description FROM networks ORDER BY lower(name), id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item Network
		if err := rows.Scan(&item.ID, &item.Name, &item.Website, &item.Description); err != nil {
			rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Networks = append(snapshot.Networks, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return RegistrySnapshot{}, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT id, network_id, name FROM network_servers ORDER BY network_id, lower(name), id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item NetworkServer
		if err := rows.Scan(&item.ID, &item.NetworkID, &item.Name); err != nil {
			rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Servers = append(snapshot.Servers, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return RegistrySnapshot{}, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT id, server_id, host, port, tls FROM network_endpoints ORDER BY server_id, host, port, tls, id`)
	if err != nil {
		return RegistrySnapshot{}, err
	}
	for rows.Next() {
		var item NetworkEndpoint
		if err := rows.Scan(&item.ID, &item.ServerID, &item.Host, &item.Port, &item.TLS); err != nil {
			rows.Close()
			return RegistrySnapshot{}, err
		}
		snapshot.Endpoints = append(snapshot.Endpoints, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return RegistrySnapshot{}, err
	}
	rows.Close()
	return snapshot, nil
}
