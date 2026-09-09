# M4.4 — PostgreSQL incident persistence parity

M4.4 moves the endpoint incident state machine onto the PostgreSQL production path while preserving the established SQLite semantics.

## Schema

PostgreSQL schema version advances from v1 to v2 with:

- `agent_endpoint_state`,
- `endpoint_transition_events`,
- `incident_records`.

The migration is transactional and backfills state, transition events, and reconstructable endpoint lifecycles from existing PostgreSQL observations.

## Atomic ingest

A newly accepted PostgreSQL observation is processed in one transaction:

1. insert the observation idempotently,
2. compare it with persisted agent/endpoint state,
3. create a `down` or `recovered` transition only for a newer reachability change,
4. refresh the affected endpoint lifecycle only when a transition occurred,
5. advance `agent_endpoint_state` only when the observation is newer,
6. commit all writes together.

Duplicate retries commit as strict no-ops for derived state. Out-of-order telemetry remains stored but cannot create transitions or regress persisted agent state.

## Lifecycle replay

Endpoint lifecycle maintenance uses the same bounded checkpoint rule as SQLite M3.30: once a lifecycle exists, transition replay begins one correlation window before the latest persisted lifecycle start. This retains boundary correctness without lifetime-sized transition scans.

## Read parity

`PostgresStore` now implements `IncidentRecordReader` through `ListIncidentRecords`, including status, host, time-range, and limit validation.

## Qualification

CI against PostgreSQL 17 covers:

- v2 schema creation,
- two-agent correlated down incident,
- correlated recovery and closed lifecycle fields,
- persisted agent state,
- duplicate retry idempotency,
- out-of-order state protection,
- incident query validation.

## Deliberate boundary

M4.4 covers endpoint incident persistence only. Network incident persistence, discovery state, network statistics, and the full runtime backend switch remain later milestones. PostgreSQL is therefore still not advertised as a complete production runtime backend after M4.4.
