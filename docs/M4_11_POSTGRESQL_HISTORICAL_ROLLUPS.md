# M4.11 — PostgreSQL historical rollup foundation

M4.11 adds the first storage layer intended for multi-year IRCIntel history without requiring every historical chart or statistic to scan raw observations.

## Schema v5

PostgreSQL schema version 5 adds:

- `observation_rollups_hourly`
- `observation_rollups_daily`

Each bucket is keyed by UTC bucket start plus endpoint host/port/TLS and stores:

- observation count
- reachable observation count
- dual-stack observation count
- first observed timestamp
- last observed timestamp

The v5 migration backfills both rollup tables from existing raw observations.

## Ingest semantics

A newly inserted observation updates its hourly and daily bucket inside the same PostgreSQL transaction as the raw observation and incident/state updates. Duplicate observations remain no-ops and therefore cannot double-count rollups.

## Rebuild/backfill

`PostgresStore.RebuildObservationRollups(since, until)` replaces the selected bucket range from canonical raw observations. It is intended for repair/backfill and is idempotent.

## Retention safety

M4.11 deliberately does **not** enable automatic raw-data retention.

`PostgresStore.PruneObservationsBefore(cutoff)` is explicit/manual. In one transaction it:

1. rebuilds hourly and daily rollups for raw history before the cutoff;
2. deletes raw observations older than the cutoff;
3. commits both operations together.

This provides a safe primitive for a later scheduled retention policy while avoiding accidental startup-time data loss.

Incident lifecycles and network incident records remain separately persisted and are not pruned by this operation.

## Qualification

PostgreSQL tests verify:

- schema v5 migration;
- atomic incremental rollup updates;
- duplicate ingest does not inflate aggregates;
- hourly and daily aggregation;
- rebuild idempotence;
- raw-data pruning after rollup rebuild;
- historical rollups remain queryable after raw observations are pruned;
- query/cutoff validation.

## Deliberate scope

M4.11 is the storage foundation only. It does not yet expose a public historical API, automate retention scheduling, or normalize network ownership into the rollup key. Network-level historical presentation and retention operations/metrics belong in subsequent milestones.
