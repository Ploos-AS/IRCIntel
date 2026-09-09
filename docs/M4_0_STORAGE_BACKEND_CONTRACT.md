# M4.0 — Storage backend contract

M4.0 starts the production-storage phase by making the runtime storage backend explicit.

## Configuration

`IRCINTEL_STORAGE_BACKEND` selects the backend:

- `sqlite` — current implemented backend and default;
- `postgres` — reserved production backend, configuration contract accepted but startup fails closed until the backend is implemented.

SQLite uses `IRCINTEL_DB_PATH`, defaulting to `/data/ircintel.db`.

PostgreSQL uses `IRCINTEL_DATABASE_URL`.

Configuration is intentionally strict:

- unknown backends are rejected;
- PostgreSQL without `IRCINTEL_DATABASE_URL` is rejected;
- `IRCINTEL_DATABASE_URL` with the SQLite backend is rejected to prevent accidental misconfiguration;
- selecting PostgreSQL currently returns an explicit not-implemented startup error rather than silently falling back to SQLite.

## Why this milestone exists

IRCIntel v0.1.0 is intended to be a long-running public production service with multi-year statistics. SQLite remains useful for development, testing, small installations, and migration qualification, but the production path needs a database backend designed for concurrent ingest, long retention, operational backups, and future rollups.

M4.0 establishes the operator-facing configuration contract before adding PostgreSQL implementation code. This keeps the next milestone focused on storage semantics rather than also changing runtime configuration.

## Compatibility

Existing installations require no changes: the default remains SQLite at `/data/ircintel.db`.

No HTTP, JSON, or SQLite schema changes are introduced by M4.0.

## Next

M4.1 will implement the first PostgreSQL storage foundation, including connection/ping handling and schema migration ownership. Feature parity with the complete SQLite store will be added and qualified incrementally rather than claimed prematurely.
