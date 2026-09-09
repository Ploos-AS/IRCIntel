package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const postgresSchemaVersion = 1

type PostgresStore struct {
	pool *pgxpool.Pool
}

func OpenPostgresStore(databaseURL string) (*PostgresStore, error) {
	if databaseURL == "" {
		return nil, errors.New("postgres database URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	config.MaxConns = 8
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	store := &PostgresStore{pool: pool}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres store is not open")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		return fmt.Errorf("create postgres migration table: %w", err)
	}

	var version int
	err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return fmt.Errorf("read postgres schema version: %w", err)
	}
	if version > postgresSchemaVersion {
		return fmt.Errorf("postgres schema version %d is newer than supported version %d", version, postgresSchemaVersion)
	}
	if version == 0 {
		if _, err := tx.Exec(ctx, `
CREATE TABLE IF NOT EXISTS observations (
    id BIGSERIAL PRIMARY KEY,
    agent_id TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    endpoint_host TEXT NOT NULL,
    endpoint_port TEXT NOT NULL DEFAULT '',
    endpoint_tls BOOLEAN NOT NULL,
    payload_json JSONB NOT NULL,
    UNIQUE(agent_id, observed_at, endpoint_host, endpoint_port, endpoint_tls)
);
CREATE INDEX IF NOT EXISTS idx_pg_observations_agent_time ON observations(agent_id, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_pg_observations_endpoint_time ON observations(endpoint_host, endpoint_port, endpoint_tls, observed_at DESC);
CREATE TABLE IF NOT EXISTS networks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    website TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pg_networks_name ON networks(lower(name));
CREATE TABLE IF NOT EXISTS network_servers (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE RESTRICT,
    name TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pg_network_servers_network ON network_servers(network_id, name);
CREATE TABLE IF NOT EXISTS network_endpoints (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES network_servers(id) ON DELETE RESTRICT,
    host TEXT NOT NULL,
    port TEXT NOT NULL,
    tls BOOLEAN NOT NULL,
    UNIQUE(server_id, host, port, tls)
);
CREATE INDEX IF NOT EXISTS idx_pg_network_endpoints_server ON network_endpoints(server_id, host, port, tls);
`); err != nil {
			return fmt.Errorf("apply postgres migration v1: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES (1)`); err != nil {
			return fmt.Errorf("record postgres migration v1: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("postgres store is not open")
	}
	return s.pool.Ping(ctx)
}

func (s *PostgresStore) SchemaVersion(ctx context.Context) (int, error) {
	if s == nil || s.pool == nil {
		return 0, errors.New("postgres store is not open")
	}
	var version int
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}
