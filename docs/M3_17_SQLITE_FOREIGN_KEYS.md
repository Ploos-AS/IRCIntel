# M3.17 — SQLite foreign key enforcement

M3.17 makes SQLite referential integrity an explicit runtime invariant for IRCIntel Core.

## Change

`OpenSQLiteStore` now configures the store to use a single SQLite connection and enables `PRAGMA foreign_keys = ON` before migrations or application queries run.

SQLite foreign-key enforcement is connection-scoped. Restricting the store to one open/idle connection ensures the PRAGMA applies consistently to all registry and discovery operations using the store.

Startup verifies `PRAGMA foreign_keys` and fails closed if enforcement cannot be enabled.

## Qualification

Regression coverage verifies that:

- `PRAGMA foreign_keys` reports enabled;
- a `network_servers` row referencing a missing network is rejected;
- a `network_endpoints` row referencing a missing server is rejected;
- valid parent/child registry inserts continue to work.

This removes the earlier dependency on application-level parent existence checks as the sole protection against orphaned registry rows.
