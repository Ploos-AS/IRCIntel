# M4.20 — PostgreSQL rollup correctness hardening

M4.20 hardens the long-term observation rollups used by retention maintenance and historical public statistics.

## Goals

- make hourly and daily bucket boundaries explicitly UTC, independent of the PostgreSQL session timezone;
- make partial-window rebuild requests safe when `since` or `until` is not aligned to an hour/day boundary;
- preserve complete historical buckets across raw-observation retention cutoffs;
- retain the existing transactional maintenance behavior.

## UTC bucket semantics

Incremental rollup updates and rebuilds now use PostgreSQL `date_trunc(..., 'UTC')` for `timestamptz` values. Bucket identity therefore no longer depends on the database/session timezone.

The Go-side range helpers also normalize to UTC before computing hour/day boundaries.

## Complete-bucket rebuild semantics

`rebuildPostgresRollupsTx` no longer filters raw observations using the exact caller timestamps while deleting whole rollup buckets.

For each granularity independently it expands requested bounds to complete UTC buckets:

- `since` is rounded down to the containing bucket start;
- a non-aligned `until` is rounded up to the next bucket boundary;
- an already aligned `until` remains exclusive at that exact boundary.

The delete range and raw-observation scan use the same expanded interval. A rebuild therefore cannot replace a complete bucket with only the observations on one side of an arbitrary cutoff.

This is especially important for retention. Before old raw observations are deleted, the boundary hour/day is rebuilt from all raw observations still present in that complete bucket. Deleting the older raw rows afterwards does not change the preserved rollup totals.

## Qualification coverage

M4.20 adds regression coverage for:

1. non-aligned hour/day range expansion, including input timestamps with a non-UTC offset;
2. rebuilding through a cutoff in the middle of an hour without losing observations after the cutoff in that same hour;
3. pruning raw observations at a non-aligned cutoff while preserving the complete historical hourly rollup;
4. generating an hourly rollup under a non-UTC PostgreSQL session timezone and verifying that the stored bucket remains the expected UTC hour.

Existing CI continues to cover ordinary ingest, historical APIs, PostgreSQL runtime, disaster recovery, ownership history, observability, and OCI runtime.

## Operational consequence

The partial-boundary corruption risk identified in M4.11/M4.15 is closed by this milestone. The maintenance scheduler remains opt-in as designed in M4.16; M4.20 makes its rollup/rebuild path suitable for arbitrary real-world run times rather than only hour-aligned maintenance timestamps.
