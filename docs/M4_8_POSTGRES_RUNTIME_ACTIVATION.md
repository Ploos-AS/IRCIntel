# M4.8 — PostgreSQL runtime activation

M4.8 makes PostgreSQL an actual IRCIntel Core runtime backend.

## Contract

`core.RuntimeStore` is the complete storage capability contract required by the Core process. Both `SQLiteStore` and `PostgresStore` have compile-time assertions against this interface. A future backend must implement the entire contract before it can be selected at runtime.

## Configuration

SQLite remains the default:

```text
IRCINTEL_STORAGE_BACKEND=sqlite
IRCINTEL_DB_PATH=/data/ircintel.db
```

PostgreSQL is selected explicitly:

```text
IRCINTEL_STORAGE_BACKEND=postgres
IRCINTEL_DATABASE_URL=postgres://user:password@db/ircintel
```

There is no fallback between backends. Invalid configuration, connection failure, ping failure, or migration failure prevents Core startup.

## Runtime parity

The PostgreSQL backend now satisfies the Core runtime capabilities used by observation ingest/read, endpoint and network status, incident lifecycle, network incident statistics/reliability, registry reads/writes, discovery review/promotion, and registry-driven probe plans.

## Qualification boundary

M4.8 activates the backend but is not the end of production hardening. Long-running public-service work still includes scoped/incremental network incident refresh, historical rollups and retention, backup/restore qualification, operational metrics, public API/web hardening, and production deployment qualification.
