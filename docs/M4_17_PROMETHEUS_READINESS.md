# M4.17 — Prometheus metrics and readiness

M4.17 adds production-oriented readiness and Prometheus-compatible metrics without changing the existing liveness contract.

## Endpoints

- `GET /healthz` remains a simple process liveness check and returns `ok` while Core is serving HTTP.
- `GET /readyz` checks the selected storage backend with a bounded ping. It returns HTTP 200 with `ready` when storage is reachable, otherwise HTTP 503.
- `GET /metrics` exposes Prometheus text format.

Both SQLite and PostgreSQL implement the readiness checker.

## Metrics

Initial metrics are deliberately small and stable:

- `ircintel_ready`
- `ircintel_process_uptime_seconds`
- `ircintel_http_requests_total`
- `ircintel_maintenance_enabled`
- `ircintel_maintenance_runs_total`
- `ircintel_maintenance_failures_total`
- `ircintel_maintenance_last_success_timestamp_seconds`
- `ircintel_maintenance_last_duration_seconds`
- `ircintel_maintenance_last_raw_observations_pruned`
- `ircintel_maintenance_last_hourly_rollups_pruned`

Maintenance metrics reuse the M4.16 scheduler status model. When maintenance is disabled, enabled/runs/failures remain zero and no destructive action occurs.

The initial HTTP counter is intentionally global rather than route-labelled, avoiding unbounded label cardinality. Route/status-class metrics can be added later with a fixed route vocabulary.

## Qualification

Unit tests verify ready/unready behavior, Prometheus output and maintenance values. A dedicated PostgreSQL runtime workflow starts the real Core binary, checks `/readyz`, and validates the key metric series on `/metrics`.

M4.17 is qualified only when the normal CI, Historical Stats and Observability workflows are green on the final commit.
