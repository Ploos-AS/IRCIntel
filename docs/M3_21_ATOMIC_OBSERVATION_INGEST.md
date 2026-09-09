# M3.21 — Atomic observation ingest

M3.21 makes SQLite observation ingestion atomic with the incident state derived from that observation.

## Behaviour

`SQLiteStore.Store` now starts one transaction that covers:

1. idempotent observation insert,
2. endpoint incident derivation and persistence,
3. network incident derivation and persistence,
4. commit.

If either derived refresh fails, the observation insert and any incident writes performed by that ingest are rolled back together.

Duplicate observations remain idempotent: when the M3.20 identity constraint ignores a retry, the no-op transaction commits without running either incident refresh.

## Implementation

The transaction-aware internal helpers are in `internal/core/transactional_ingest.go`:

- `refreshIncidentRecordsTx`
- `recentObservationsTx`
- `refreshNetworkIncidentRecordsTx`
- `allIncidentRecordsTx`
- `registrySnapshotTx`

The existing public/read-oriented store methods are unchanged.

## Qualification

Regression coverage verifies that a failure during downstream network-incident derivation rolls back the newly inserted observation. Successful ingestion and duplicate retries are also covered.

## Remaining scalability work

Atomicity does not change the cost of canonical incident maintenance. Endpoint incident refresh still scans a bounded recent observation window, while network incident refresh still reads the complete persisted endpoint-incident history on every new observation. Incremental/event-driven maintenance remains a later milestone.
