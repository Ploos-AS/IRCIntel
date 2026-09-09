# M3.26 — Endpoint-scoped incident refresh

M3.26 removes the global observation-history scan from persisted endpoint incident maintenance.

## Previous behavior

Every newly accepted observation caused Core to read the newest 5000 observations across all endpoints, derive all reachability transitions again, correlate them, and upsert all derived endpoint incident lifecycles.

That was correct for the bounded history but made ingest cost grow with unrelated endpoint traffic and meant a malformed historical payload on any endpoint could block otherwise unrelated ingestion.

## New behavior

After the new observation is inserted inside the ingestion transaction, Core identifies that observation's endpoint tuple `(host, port, TLS)` and reads observation history only for that endpoint. Existing transition detection, five-minute distributed correlation, lifecycle pairing, and persisted `incident_records` semantics are unchanged.

Network-incident refresh remains network-scoped as introduced in M3.25. The observation insert, endpoint incident refresh, and network incident refresh remain atomic in one SQLite transaction.

## Properties

- no global 5000-observation scan during persisted endpoint incident maintenance
- unrelated endpoint history is not decoded during ingest
- malformed history on an unrelated endpoint cannot block the current endpoint
- malformed history on the affected endpoint still fails closed and rolls back the ingest transaction
- no public HTTP or JSON contract changes
- no schema migration is required

## Remaining scalability work

M3.26 is endpoint-scoped rather than fully event-driven. A very long-lived, high-frequency single endpoint can still accumulate a large endpoint-local observation history. A later milestone can persist transition/correlation state so each ingest needs only the previous agent state plus the active correlation window instead of replaying the complete endpoint history.
