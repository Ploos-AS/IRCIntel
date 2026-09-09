# M2.11 — incident lifecycle query API

M2.11 makes persisted incident history directly useful to API and web consumers.

`GET /api/v1/incidents/lifecycle` now accepts filters over canonical persisted incident records:

- `status=open|closed`
- `host=<exact hostname>`
- `since=<RFC3339/RFC3339Nano timestamp>`
- `until=<RFC3339/RFC3339Nano timestamp>`
- `limit=<1..500>`, default `100`

`since` and `until` are inclusive and apply to incident `started_at`. All filters may be combined. Invalid status values, timestamps, limits, or reversed time ranges return `400 Bad Request`.

Without an explicit `window=`, filters are executed in SQLite against `incident_records`, preserving newest-first ordering and avoiding reconstruction from raw observations. When callers provide `window=`, Core retains the M2.9 analytical reconstruction path and applies the same lifecycle filters to the reconstructed result.

Exact-host matching is intentional at this stage; network identities and higher-level network/server relationships belong to the later registry model rather than being inferred from hostnames.

## Qualification

M2.11 qualification covers SQLite filtering by status, host and inclusive time range, invalid query validation, HTTP lifecycle filtering, persistence compatibility, dependency cleanliness, Go tests/vet/build, and the OCI runtime gate.
