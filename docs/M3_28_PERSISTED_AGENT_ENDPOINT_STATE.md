# M3.28 — Persisted agent endpoint state

M3.28 adds durable latest-state tracking for every `(agent_id, endpoint_host, endpoint_port, endpoint_tls)` tuple.

## Schema

SQLite schema version advances from v4 to v5.

The new `agent_endpoint_state` table stores:

- agent ID
- endpoint host, port and TLS mode
- latest observation timestamp
- latest reachability state

The primary key is the complete agent+endpoint tuple. `observed_at` is indexed for operational inspection.

## Migration

The v4→v5 migration is transactional and backfills state from the newest persisted observation for every tuple. Observation payloads are decoded using the same `agent.Observation` model used by normal ingestion.

If any historical newest observation cannot be decoded, migration fails and the schema-version transaction rolls back.

## Ingestion semantics

After a non-duplicate observation has successfully completed endpoint- and network-incident derivation, its agent+endpoint state is upserted inside the same ingest transaction.

An older, out-of-order observation is retained in observation history but cannot regress persisted latest state: the state row is updated only when the incoming `observed_at` is newer.

Duplicate observations remain no-ops through the existing observation identity constraint.

## Scope

M3.28 deliberately does **not** switch incident lifecycle derivation to event-driven state consumption yet. M3.27 checkpoint-bounded replay remains the canonical incident derivation path for this milestone.

This separation keeps the schema/backfill change independently qualifiable. A following milestone can consume `agent_endpoint_state` to derive per-agent reachability transitions without historical observation replay.

## Qualification

Regression coverage verifies:

1. normal ingest tracks the newest state;
2. out-of-order observations cannot regress state;
3. a simulated v4 database is backfilled correctly and reaches `PRAGMA user_version = 5`.
