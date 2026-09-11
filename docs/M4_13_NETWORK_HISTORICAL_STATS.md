# M4.13 — Network historical statistics

M4.13 extends the M4.12 rollup-backed history API with network-scoped time series derived from the curated registry.

## API

`GET /api/v1/history/networks?network_id=<id>&period=<24h|7d|30d|1y|all>`

Resolution follows M4.12:

- `24h`, `7d`: hourly rollups
- `30d`, `1y`, `all`: daily rollups

The response includes network ID/name and observation, reachable and dual-stack counts/percentages for each bucket.

## Registry attribution

Observations do not carry a network ID. Network attribution therefore joins rollup endpoint identity `(host, port, tls)` through `network_endpoints -> network_servers -> networks`.

An endpoint tuple curated under more than one network is ambiguous. M4.13 deliberately excludes such tuples from all network history rather than double-counting or misattributing them. This preserves correctness until IRCIntel has a stronger persistent endpoint identity model.

## Storage

M4.13 adds no schema migration. It reads the M4.11 hourly/daily rollups and the existing curated registry.

SQLite remains supported for Core runtime, but historical APIs remain unavailable there; PostgreSQL is the production history backend.

## Qualification

Unit/integration coverage verifies:

- two independent networks do not contaminate each other's series;
- percentages are derived from the selected network only;
- ambiguous endpoint tuples are excluded;
- missing networks return the typed registry not-found error / HTTP 404;
- period validation is shared with M4.12.

The `Historical Stats` GitHub Actions workflow starts real Core against PostgreSQL 17, seeds two networks and their observations over HTTP, and verifies both host-scoped and network-scoped history APIs.
