# M3.11 — Per-network incident statistics

IRCIntel now exposes a single API view for comparing incident statistics across every curated IRC network.

`GET /api/v1/networks/incidents/stats/by-network`

Optional filters:

- `since=<RFC3339Nano>`
- `until=<RFC3339Nano>`

The response contains one entry for every network in the curated registry, including networks with zero incidents in the selected period. Each entry includes the M3.10 statistics payload: total/open/closed incidents, degraded/down counts, total downtime, mean recovery time, longest incident, and latest incident start.

Results are deterministically ordered by network name and then network ID.

Unlike the single-network/history endpoints, the SQLite implementation reads the complete persisted network-incident set for the selected period without the 500-record history cap. This makes the endpoint suitable for a public IRC ecosystem overview, comparison table, and future reliability ranking.

Discovery candidates remain excluded until explicitly promoted into the curated registry.
