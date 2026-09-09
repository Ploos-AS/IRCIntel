# M3.8 — Persisted network incidents

Network-level incidents are now durable SQLite records rather than API-only reconstruction.

- `network_incident_records` stores one canonical JSON payload per `network_id + started_at`.
- Observation ingestion refreshes endpoint incident records first, then derives and upserts network incident records from the curated registry.
- Open incidents are updated in place as more curated endpoints become affected.
- Recovery updates the same record to `closed` with recovery time and duration.
- `GET /api/v1/networks/incidents` prefers persisted records when the backing store supports them; the derivation path remains as a compatibility fallback for test/in-memory readers.
- Records survive Core restart/reopen.

Discovery candidates remain excluded until explicit promotion into the curated registry.

This milestone intentionally retains the current bounded endpoint-history reconstruction during ingest. Incremental event-driven aggregation and schema versioning remain later optimization/migration work.
