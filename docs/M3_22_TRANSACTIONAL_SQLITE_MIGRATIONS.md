# M3.22 — Transactional SQLite migrations

M3.22 makes every numbered SQLite migration atomic.

## Behaviour

`SQLiteStore.migrate()` now runs each schema step through one internal `runMigration` helper. The migration body and its `PRAGMA user_version` update execute in the same `sql.Tx` and are committed together.

The existing migration chain remains unchanged semantically:

- v0 -> v1 creates the baseline Core/registry schema and performs the one-time legacy timestamp normalization.
- v1 -> v2 creates discovery candidate, review, and promotion storage.
- v2 -> v3 deduplicates historical observation retries and creates the observation identity index.

The current schema version remains 3. No public API or payload format changes.

## Failure semantics

If any statement or data transformation in a numbered migration fails, the transaction is rolled back. The database therefore keeps both its previous schema/data state and previous `user_version`, instead of exposing a partially applied migration with an old version marker.

The v0 -> v1 timestamp normalization was refactored to support execution inside the migration transaction, avoiding nested transactions while preserving the standalone helper used by internal code/tests.

## Qualification

Regression coverage deliberately creates a table inside a migration callback and then returns an error. The test verifies both that the table is absent after rollback and that `PRAGMA user_version` is unchanged.
