# M3.23 — Network incident timestamp migration

M3.23 completes IRCIntel's fixed-width UTC timestamp migration for persisted incident data.

## Change

SQLite schema version advances from 3 to 4. The v3 -> v4 migration reads each persisted `network_incident_records.payload_json`, decodes the canonical `NetworkIncident.StartedAt`, and rewrites `network_incident_records.started_at` using IRCIntel's fixed-width UTC layout:

`2006-01-02T15:04:05.000000000Z`

The migration runs through the transactional migration helper introduced in M3.22, so the timestamp rewrites and `PRAGMA user_version = 4` commit atomically.

## Why

Older network incident rows could contain variable-width RFC3339Nano strings. SQLite compares these TEXT timestamps lexically, so mixed-width fractional seconds can produce incorrect ordering or range-filter behavior. Observations and endpoint incident records were already normalized; M3.23 closes the remaining network-incident gap.

## Compatibility

- No HTTP API changes.
- No JSON payload changes.
- Fresh databases traverse the existing migration chain through v4 automatically.
- Existing v3 databases are upgraded once on open.
- Future opens do not rescan network incident history after the database reaches v4.

## Qualification

Regression coverage creates a v3 database containing a legacy variable-width network incident timestamp, reopens it through `OpenSQLiteStore`, and verifies both the fixed-width stored value and schema version 4.
