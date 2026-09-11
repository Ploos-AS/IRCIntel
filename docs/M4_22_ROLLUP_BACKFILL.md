# M4.22 — Bounded, resumable PostgreSQL rollup backfill

M4.22 removes the last unbounded historical rollup scan from PostgreSQL schema migration/startup and replaces it with an explicit, checkpointed backfill operation.

## Problem closed

The original v5 rollup migration created the hourly/daily rollup tables and immediately called a full-history `rebuildPostgresRollupsTx` in the same migration transaction. That is acceptable for small databases but unsafe for a multi-year public deployment because application startup could become proportional to all retained raw observations.

M4.22 changes the migration contract:

- schema migration creates rollup tables and indexes only;
- it does **not** scan historical observations;
- newly ingested observations still update hourly/daily rollups transactionally as before;
- pre-existing historical observations are backfilled explicitly in bounded ranges.

## Checkpoint contract

`observation_rollup_backfill_state` stores one durable checkpoint with:

- `next_start` — next UTC historical boundary to process;
- `upper_bound` — fixed end of the initial historical snapshot;
- `completed` — whether the snapshot has been fully processed;
- `updated_at` — last committed progress time.

The state row is locked with `FOR UPDATE` while a batch runs, preventing two workers from advancing the same checkpoint concurrently.

The initial range is derived once from `MIN(observed_at)` / `MAX(observed_at)` and normalized to complete UTC day boundaries. New observations arriving after that snapshot continue to be represented by the normal incremental rollup path.

## Batch operation

`RunObservationRollupBackfillBatch(span)`:

1. validates a positive bounded span (maximum 90 days);
2. initializes and locks the checkpoint if necessary;
3. rebuilds only `[next_start, min(next_start + span, upper_bound))`;
4. rebuilds complete UTC hour/day buckets through the M4.20 correctness path;
5. advances the checkpoint in the **same transaction** as the rebuilt rollups;
6. commits only when both rollup data and checkpoint are durable.

A crash or cancellation before commit therefore leaves both aggregates and checkpoint unchanged. Re-running the same batch is safe because rebuild uses delete/recompute semantics for the bounded range.

## Restart/resume semantics

The checkpoint persists independently of the Core process. A later process can call `ObservationRollupBackfillStatus()` and continue with the next batch. Once complete, repeated batch calls are no-ops and do not inflate aggregates.

## Operational policy

M4.22 intentionally does not run historical backfill automatically on Core startup. Operators can choose a conservative batch size based on database load and run batches during controlled maintenance windows. A future operational milestone may expose scheduling/status through the public operations surface, but startup remains bounded regardless.

## Qualification

The dedicated `Rollup Backfill` workflow runs against PostgreSQL 17 and verifies:

- historical rows can exist without rollups;
- one 24-hour batch processes only one bounded historical window;
- checkpoint progress is persisted;
- Core/store restart resumes from the persisted boundary;
- subsequent batches complete the snapshot exactly;
- hourly and daily totals are correct;
- an additional run after completion is idempotent;
- invalid/oversized spans are rejected.
