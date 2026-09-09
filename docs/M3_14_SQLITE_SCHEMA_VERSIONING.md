# M3.14 — SQLite schema versioning

M3.14 introduces explicit SQLite schema versioning with `PRAGMA user_version`.

## Motivation

M2.12 fixed timestamp ordering by normalizing legacy RFC3339Nano timestamps to a fixed-width UTC representation. Until M3.14 that normalization scanned and rewrote observation and endpoint-incident history every time IRCIntel Core started. That is unacceptable for a long-running public service as history grows.

## Version 1

`sqliteSchemaVersion` is now `1`.

When Core opens a version-0 database it:

1. creates the baseline tables and indexes idempotently;
2. runs the legacy timestamp normalization once;
3. records `PRAGMA user_version = 1`.

Subsequent opens of a version-1 database skip the O(N) data normalization entirely.

A database whose `user_version` is newer than the running IRCIntel binary supports is rejected rather than being opened with unknown schema semantics.

## Compatibility

Existing pre-M3.14 databases report version 0 and are upgraded automatically. Fresh databases also start at version 0 and reach version 1 during initial creation. Public HTTP APIs and stored JSON payload formats are unchanged.

## Qualification

Tests cover:

- legacy v0 timestamp normalization and upgrade to v1;
- no repeated data migration after v1 is recorded;
- rejection of a future/unsupported schema version.

## Follow-up

Discovery/review/promotion tables still have historical lazy `CREATE TABLE IF NOT EXISTS` helpers. A later migration should fold those tables into numbered migrations and make schema ownership fully centralized.
