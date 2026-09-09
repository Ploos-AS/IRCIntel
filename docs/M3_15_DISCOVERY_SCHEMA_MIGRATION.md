# M3.15 — Discovery schema migration

M3.15 moves the discovery pipeline's SQLite schema into IRCIntel's canonical `PRAGMA user_version` migration chain.

## Schema version

`sqliteSchemaVersion` is now `2`.

Migration sequence:

- `v0 -> v1`: create the original Core schema and perform the one-time legacy timestamp normalization introduced by M3.14.
- `v1 -> v2`: create `discovery_candidates`, `discovery_reviews`, `discovery_promotions`, and their indexes.

Fresh databases run both migrations in sequence. Existing M3.14 databases start at v1 and receive only the discovery-schema migration.

## Compatibility

The migration uses `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`. Deployments where earlier IRCIntel releases lazily created discovery tables keep their existing rows when moving to schema v2.

The existing discovery storage compatibility guards remain idempotent for now, but schema ownership is centralized: a successfully opened v2 database is expected to contain the complete discovery/review/promotion schema before any discovery API call occurs.

## Qualification

Regression coverage verifies that a database marked `user_version=1` and lacking the three discovery tables is upgraded to v2 at open time and receives all required tables.

## Next debt

A later cleanup can remove the now-redundant per-feature schema guards entirely. Other database work still includes foreign-key enforcement verification, transactionality around observation/incident refresh, and incremental incident maintenance for large long-running public deployments.
