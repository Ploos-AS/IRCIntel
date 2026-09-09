# M2.10 — persisted incident lifecycle records

M2.10 makes the default incident lifecycle path durable.

Core now stores incident lifecycle snapshots in SQLite in `incident_records`. Records are keyed by endpoint identity plus incident start time, and later observations update an existing open record to its closed form when distributed recovery is confirmed.

The persisted record contains the same public lifecycle object used by `GET /api/v1/incidents/lifecycle`: endpoint identity, open/closed state, start time, recovery time, duration, down agents, and recovery agents.

## Runtime behavior

Every accepted observation is stored first. Core then refreshes lifecycle records derived from the bounded recent-observation correlation window using the canonical five-minute distributed-correlation rule. Existing older incident records are retained; refreshes use UPSERT and never delete historical records.

The default `GET /api/v1/incidents/lifecycle` request reads persisted records directly when the configured reader supports them. Supplying an explicit `window=` keeps the previous on-demand reconstruction behavior so callers can explore alternate correlation windows without rewriting the canonical persisted history.

## Qualification

M2.10 qualification verifies that a distributed outage creates an open incident record, distributed recovery updates that same record to closed with duration, and the record survives closing and reopening the SQLite database. Existing Go, dependency-cleanliness, vet, build, and OCI runtime gates remain mandatory.
