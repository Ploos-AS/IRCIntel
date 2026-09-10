# M4.7 — PostgreSQL read-model parity

M4.7 closes the remaining PostgreSQL read-model gap needed by the curated probe plan, network status, network incident statistics, and per-network statistics APIs.

## Scope

PostgreSQL now implements `NetworkIncidentStatsSnapshot(since, until)` with the same unbounded persisted-history semantics as SQLite. The method returns the curated registry snapshot together with persisted network incident records, with optional inclusive time bounds and deterministic newest-first ordering.

The existing handlers are intentionally backend-agnostic and require no PostgreSQL-specific forks:

- probe plan uses `RegistryReader`
- network status uses `RegistrySnapshot` plus `LatestEndpointObservations`
- aggregate incident statistics use `ListNetworkIncidentRecords`
- per-network statistics use `NetworkIncidentStatsSnapshot`

M4.7 adds end-to-end PostgreSQL qualification covering a curated network, live observation state, a persisted closed network incident, probe-plan generation, current network status, aggregate statistics, per-network statistics, time filtering, and invalid-range rejection.

## Production note

This milestone does not switch the Core runtime to PostgreSQL. Runtime activation remains fail-closed until the complete handler-facing store contract is audited and the remaining backend-specific gaps are closed. SQLite remains unchanged.

No schema migration is required for M4.7; it operates on the PostgreSQL schema introduced by M4.1 through M4.6.
