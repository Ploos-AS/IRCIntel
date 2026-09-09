# M3.25 — Network-scoped incident refresh

M3.25 reduces the cost of persisted network-incident maintenance during observation ingestion.

Previously, every accepted observation caused Core to read and decode the complete `incident_records` history before deriving network incidents. That was correct after M3.13, but its cost grew with the total history across every curated IRC network.

The refresh is now scoped to the curated network or networks containing the endpoint from the observation being ingested:

1. The newest inserted observation identifies its endpoint.
2. Core resolves the curated network IDs containing that endpoint.
3. Only endpoint incident records belonging to those networks are loaded.
4. Network incident derivation and upserts run from that scoped history.

If an observation endpoint is not present in the curated registry, network-incident refresh returns immediately without scanning endpoint-incident history.

Correctness properties retained:

- observation insertion, endpoint incident refresh, and network incident refresh remain in one SQLite transaction;
- corrupt incident history in the affected network still aborts and rolls back the ingest;
- corrupt or otherwise unrelated history in another network is no longer read by that ingest;
- persisted network-incident and HTTP API schemas are unchanged;
- no historical record cap is reintroduced for the affected networks.

This is a performance hardening step, not the final incremental incident architecture. Endpoint incident derivation still scans up to `incidentScanLimit` observations, and an affected network still scans its complete endpoint-incident history. A later milestone can maintain incident state incrementally per endpoint/network.
