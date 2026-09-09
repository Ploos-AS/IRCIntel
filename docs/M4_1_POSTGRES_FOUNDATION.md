# M4.1 — PostgreSQL storage foundation

M4.1 establishes the first production PostgreSQL storage layer without claiming full application-backend parity yet.

## Driver and pool

IRCIntel now includes `pgx/v5` and opens PostgreSQL through `pgxpool`.

`OpenPostgresStore`:

- parses the configured database URL;
- creates a bounded connection pool;
- performs a startup `Ping` with a 10-second context;
- fails closed when connectivity is unavailable;
- runs PostgreSQL migrations before returning a usable foundation store.

Initial pool defaults are deliberately conservative: maximum 8 connections, minimum 1 connection, 1-hour maximum connection lifetime, 15-minute idle lifetime, and 1-minute health checks.

## Migration v1

PostgreSQL schema versioning is explicit through `schema_migrations`.

Version 1 creates the production foundations for:

- `observations` using `TIMESTAMPTZ` and `JSONB`;
- observation identity uniqueness for retry idempotency;
- agent/time and endpoint/time indexes;
- `networks`;
- `network_servers` with foreign-key integrity;
- `network_endpoints` with foreign-key integrity and endpoint uniqueness.

Future PostgreSQL migrations must be additive, ordered, transactional, and recorded in `schema_migrations`.

## CI qualification

The normal CI test job now starts a real `postgres:17` service and provides `IRCINTEL_TEST_POSTGRES_URL`.

The PostgreSQL foundation test verifies:

- successful pool creation and ping;
- migration execution;
- expected schema version;
- existence of the initial production tables.

The test skips only outside environments where `IRCINTEL_TEST_POSTGRES_URL` is intentionally absent.

## Runtime status

`IRCINTEL_STORAGE_BACKEND=postgres` remains reserved at the application runtime boundary until the PostgreSQL store implements the complete set of Core reader/writer interfaces. M4.1 therefore qualifies connectivity, pooling, and migration mechanics, but does **not** claim PostgreSQL feature parity with SQLite.

That parity is the next storage milestone.
