# M3.16 — Centralized discovery schema ownership

M3.16 completes the cleanup started by M3.15.

## Change

The discovery candidate, review, and promotion storage paths no longer create their own SQLite tables lazily. Schema creation and upgrades are now owned exclusively by the central `SQLiteStore.migrate()` chain.

Removed lazy schema guards:

- `ensureDiscoverySchema()`
- `ensureDiscoveryReviewSchema()`
- `ensureDiscoveryPromotionSchema()`

All discovery operations now assume `OpenSQLiteStore()` has successfully migrated the database to the supported schema version before the store is exposed to callers.

## Why

Having runtime CRUD methods also mutate schema created multiple sources of truth and made versioned migrations harder to reason about. Centralizing schema ownership gives IRCIntel one deterministic startup path for database compatibility and future upgrades.

## Compatibility

M3.15 already introduced the v1 → v2 migration with `CREATE TABLE IF NOT EXISTS`, so databases from deployments that previously created discovery tables lazily remain compatible and retain their data.

No API behavior or privacy policy changes are introduced by M3.16.
