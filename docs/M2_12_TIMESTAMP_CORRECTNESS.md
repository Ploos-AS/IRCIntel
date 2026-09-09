# M2.12 — timestamp correctness hardening

M2.12 removes a subtle ordering hazard from SQLite-backed history.

Earlier releases stored timestamps using `time.RFC3339Nano`, whose fractional-second component is variable width. Lexicographic SQLite ordering can therefore disagree with chronological ordering around values such as `...00Z` and `...00.1Z`.

Core now persists all observation and incident-key timestamps as fixed-width UTC nanosecond strings using:

`2006-01-02T15:04:05.000000000Z`

This preserves chronological ordering under ordinary SQLite TEXT comparison while keeping timestamps human-readable.

## Existing databases

Opening an existing database now normalizes legacy `observed_at` and incident `started_at` values from each row's persisted JSON payload. This means pre-M2.12 development databases do not need to be discarded merely to obtain correct ordering.

## Qualification

Regression tests cover the ordering sequence:

- `...00.000000000Z`
- `...00.100000000Z`
- `...01.000000000Z`

The suite verifies newest-first reads, inclusive time-range filtering, latest-per-endpoint selection, UTC normalization, and automatic rewriting of legacy RFC3339Nano timestamp text when a database is reopened.
