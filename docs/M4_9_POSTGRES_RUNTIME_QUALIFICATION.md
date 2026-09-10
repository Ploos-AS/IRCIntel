# M4.9 — PostgreSQL production runtime qualification

M4.9 qualifies PostgreSQL as a real IRCIntel Core runtime backend through the public HTTP surface rather than only through store-level Go tests.

## CI qualification

The `postgres-runtime` CI job starts a PostgreSQL 17 service and builds the real `cmd/ircintel` binary with:

- `IRCINTEL_STORAGE_BACKEND=postgres`
- `IRCINTEL_DATABASE_URL` pointing at the CI PostgreSQL service
- a Core bearer token
- a dedicated local listen address

The job then verifies:

1. Core starts and `/healthz` becomes ready.
2. Registry network/server/endpoint writes succeed over HTTP.
3. Observation ingestion succeeds over HTTP.
4. Registry, observation read, and endpoint-status APIs can read data from PostgreSQL.
5. The Core process is stopped.
6. The same binary is restarted against the same PostgreSQL database.
7. Registry and observation data remain available after restart.

This is deliberately a process-level persistence qualification. It catches configuration, migration, runtime wiring, handler/store-interface, and restart persistence failures that unit/store tests alone cannot detect.

## Scope

M4.9 does not yet claim full production readiness for a multi-year public service. Remaining production work includes retention/rollups, backup/restore qualification, operational metrics/readiness, concurrency/load qualification, public API protection/caching, and production deployment documentation.

SQLite remains the default backend for development and small installations. PostgreSQL is now the intended production backend and is exercised as such in CI.
