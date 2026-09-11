# M4.23 — Rollup disaster recovery qualification

## Goal

Close the remaining disaster-recovery gap for long-running historical data by proving that PostgreSQL logical backup/restore preserves IRCIntel rollups and resumable rollup-backfill state exactly.

M4.10 qualified registry, raw observations and incident recovery. M4.11 introduced historical rollups. M4.22 introduced persistent resumable backfill state. M4.23 explicitly qualifies those newer historical-storage structures through the production logical backup and restore scripts.

## Qualification contract

The dedicated `Rollup Disaster Recovery` workflow runs against PostgreSQL 17 and:

1. Starts the real IRCIntel Core so the current PostgreSQL schema is migrated normally.
2. Seeds observations through the Core API so hourly and daily rollups are produced by normal runtime code.
3. Seeds a deliberately incomplete `observation_rollup_backfill_state` checkpoint representing resumable historical work.
4. Verifies source hourly and daily rollup counts before backup.
5. Creates a logical backup using `ops/postgres/backup-logical.sh` and verifies its checksum and table inventory.
6. Restores into a fresh database using `ops/postgres/restore-logical.sh` and the destructive-restore confirmation guardrail.
7. Compares restored hourly rollups, daily rollups, and the backfill checkpoint byte-for-value at the SQL field level against the source database.
8. Starts the real Core on the restored database and verifies historical statistics remain queryable through the HTTP API.

## Data covered

The qualification asserts preservation of:

- `observation_rollups_hourly`
- `observation_rollups_daily`
- `observation_rollup_backfill_state`
- bucket timestamps
- observation, reachable, and dual-stack counts
- first/last observation timestamps
- backfill `next_start`
- backfill `upper_bound`
- backfill completion state
- backfill update timestamp

## Production meaning

A successful M4.23 qualification means a normal IRCIntel PostgreSQL logical backup contains enough historical-storage state to restore both already-computed public statistics and the exact checkpoint required to resume an interrupted historical rollup backfill.

The restore does not need to recompute all historical rollups before serving the restored data, and an unfinished backfill does not silently restart from the beginning merely because the database was restored.

## Scope boundary

M4.23 qualifies logical backup/restore durability. It does not yet implement continuous WAL archiving, point-in-time recovery, multi-region offsite replication, or automated restore drills against production snapshots. Those remain later production-operations milestones.
