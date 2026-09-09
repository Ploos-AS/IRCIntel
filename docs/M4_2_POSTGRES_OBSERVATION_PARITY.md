# M4.2 — PostgreSQL observation-store parity

M4.2 gives the PostgreSQL backend parity with SQLite for the observation read/write surface used by ingest, read APIs, and endpoint status.

## Implemented methods

`PostgresStore` now implements:

- `Store(agent.Observation)`
- `List(ObservationQuery)`
- `RecentObservations(limit)`
- `LatestEndpointObservations()`
- `Count()`

Observation retries remain idempotent through the PostgreSQL unique identity constraint and `ON CONFLICT DO NOTHING`.

Queries preserve SQLite semantics:

- newest-first ordering;
- `agent_id` and host filtering;
- inclusive `since` / `until` filtering;
- validated limits;
- latest observation per `(agent, host, port, TLS)` tuple.

All runtime operations use bounded contexts rather than unbounded database calls.

## Qualification

CI runs the Postgres tests against a real PostgreSQL 17 service. Regression coverage verifies:

- write/read round trip;
- duplicate suppression;
- count semantics;
- agent/host filters;
- time-window filters;
- recent ordering;
- latest-endpoint selection;
- invalid query rejection.

## Runtime boundary

M4.2 does **not** switch `ircintel` main to the PostgreSQL backend yet. Core still depends on registry, discovery, incident, and network-statistics interfaces that are SQLite-only today. PostgreSQL becomes selectable by the production runtime only after those required capabilities reach parity.

This avoids exposing a partially functional backend as production-ready.
