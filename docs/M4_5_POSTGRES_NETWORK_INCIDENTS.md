# M4.5 — PostgreSQL network incident persistence parity

M4.5 extends the PostgreSQL backend from endpoint incident persistence to persisted network incident lifecycles.

## Schema

PostgreSQL schema version 3 adds `network_incident_records` with:

- `network_id` foreign-keyed to the curated registry;
- canonical `started_at` as `TIMESTAMPTZ`;
- `open` / `closed` lifecycle state;
- `degraded` / `down` severity;
- JSONB lifecycle payload;
- uniqueness on `(network_id, started_at)`;
- indexes for status, network, severity and time-oriented queries.

The v2→v3 migration derives persisted network incidents from existing endpoint incident records and registry state in the same migration transaction.

## Atomic ingest path

On a real endpoint reachability transition the PostgreSQL ingest transaction now performs:

1. observation insert;
2. transition persistence;
3. endpoint incident lifecycle refresh;
4. network incident lifecycle refresh;
5. agent endpoint state update;
6. commit.

A duplicate observation remains a no-op and steady-state observations do not refresh incident state.

## Read parity

`PostgresStore` now implements `NetworkIncidentRecordReader` and supports the same public query dimensions as SQLite:

- status;
- severity;
- network ID;
- since / until;
- bounded limit;
- newest-first deterministic ordering.

## Qualification

The PostgreSQL CI path verifies a curated two-endpoint network across:

- healthy baselines from two agents;
- one endpoint down → network `degraded`;
- both endpoints down → network `down`;
- endpoint recoveries → one persisted `closed` network lifecycle;
- filter and time-window reads;
- persisted-row count;
- invalid query rejection.

## Remaining production work

M4.5 intentionally prioritizes semantic parity over final multi-year efficiency. On a real transition the PostgreSQL implementation currently derives network state from the registry snapshot and persisted endpoint incident history. Subsequent M4 work should scope/incrementalize that refresh before production qualification at large historical volumes.
