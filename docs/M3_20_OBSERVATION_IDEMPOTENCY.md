# M3.20 — Observation idempotency

M3.20 makes Core observation ingestion idempotent for agent retries.

## Identity

An observation is identified by:

- `agent_id`
- `observed_at`
- `endpoint_host`
- `endpoint_port` (NULL treated as empty for identity)
- `endpoint_tls`

The identity intentionally does not include result payload fields. A retry of the same measured observation therefore cannot create a second stored observation or trigger duplicate incident refresh work.

## SQLite migration

Schema version 3 adds the unique expression index `idx_observations_identity`.

The v2 -> v3 migration first removes historical duplicate observation rows by logical identity, keeping the newest row (`MAX(id)`), then creates the unique index. This allows existing databases with prior retry duplicates to upgrade safely.

## Ingestion behavior

`SQLiteStore.Store` uses `INSERT OR IGNORE`. If the observation already exists, the call succeeds without inserting another row and without running endpoint/network incident refresh again.

This keeps agent retry transport semantics simple: an agent may safely retry a submission after an uncertain response without multiplying observations.

## Scope

This milestone provides storage-level idempotency. It does not yet add an explicit public observation ID or transport idempotency key, and it does not make observation insertion plus incident refresh one atomic transaction. Those remain separate follow-up concerns.
