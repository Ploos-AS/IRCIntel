# M4.12 — Historical Statistics API

M4.12 exposes long-range observation statistics from PostgreSQL rollups introduced in M4.11.

## Endpoint

`GET /api/v1/history/observations`

Query parameters:

- `period`: `24h`, `7d`, `30d`, `1y`, or `all`; defaults to `24h`.
- `host`: optional endpoint-host filter.

Resolution is selected automatically:

| Period | Rollup resolution |
| --- | --- |
| 24h | hourly |
| 7d | hourly |
| 30d | daily |
| 1y | daily |
| all | daily |

The endpoint aggregates endpoint-level rollup rows into one time series per bucket. It returns observation, reachable, and dual-stack counts plus reachable and dual-stack percentages.

Long-range queries do not scan raw observations. `30d`, `1y`, and `all` are served from daily rollups; `24h` and `7d` are served from hourly rollups.

## Backend contract

PostgreSQL implements `HistoricalStatsReader`. SQLite remains suitable for development and small deployments but does not implement the historical-rollup API; the route returns HTTP 503 when the selected backend lacks the capability.

## Qualification

Automated qualification covers:

- period-to-resolution selection;
- unsupported-period rejection;
- aggregation of multiple endpoint rows into a single bucket;
- reachable and dual-stack percentage calculation;
- case-normalized host filtering;
- `all` using daily rollups without a lower time bound;
- unavailable-backend behavior;
- a real Core process against PostgreSQL 17 serving the HTTP endpoint for `24h` and `all`.

A dedicated `Historical Stats` GitHub Actions workflow seeds observations through the public ingest API and verifies the historical API over HTTP.
