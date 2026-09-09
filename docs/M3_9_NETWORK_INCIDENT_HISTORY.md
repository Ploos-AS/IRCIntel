# M3.9 — Network incident history queries

The persisted network-incident API now supports indexed historical queries.

`GET /api/v1/networks/incidents` accepts:

- `status=open|closed`
- `severity=degraded|down`
- `network_id=<registry network id>`
- `since=<RFC3339Nano>`
- `until=<RFC3339Nano>`
- `limit=1..500` (default 50)

`since` and `until` are inclusive and filter on incident `started_at`. Reversed ranges are rejected. Filters are applied in SQLite when the backing store supports persisted network incidents; the in-memory/compatibility derivation path implements the same contract.

Results remain newest-first and contain only incidents derived from endpoints in the curated registry. Discovery candidates do not enter incident history until explicitly promoted.

This milestone establishes the query contract needed by a future IRCIntel web timeline/dashboard without requiring it to download and filter the complete incident history client-side.
