# M4.16 — Operational maintenance scheduling and observability

M4.16 turns the M4.15 retention primitive into an opt-in operational scheduler while preserving the safety rule that Core startup itself must never trigger destructive maintenance.

## Configuration

Maintenance is disabled by default.

- `IRCINTEL_MAINTENANCE_ENABLED=false`
- `IRCINTEL_MAINTENANCE_INTERVAL=24h`
- `IRCINTEL_RAW_OBSERVATION_RETENTION=720h` (30 days)
- `IRCINTEL_HOURLY_ROLLUP_RETENTION=9600h` (400 days)

When enabled, maintenance requires a storage backend implementing retention maintenance. PostgreSQL does; SQLite does not.

The scheduler waits one full interval before the first run. A process restart therefore does not immediately prune data.

## Status API

`GET /api/v1/maintenance/status`

The response reports:

- whether maintenance is enabled;
- configured interval;
- last start/completion/success time;
- last duration;
- last error;
- most recent prune result;
- total runs and failures.

The endpoint follows the normal Core bearer-token contract.

## Failure behavior

A failed maintenance pass is recorded in status and the scheduler continues to the next interval. The underlying M4.15 maintenance transaction remains atomic: failed passes do not partially prune history.

## Qualification

Tests verify:

- maintenance defaults to disabled;
- explicit environment configuration is accepted;
- invalid booleans and invalid retention ordering fail closed;
- scheduler does not run immediately after startup;
- successful runs expose timestamps/results/counters;
- failed runs expose failure counters and last error;
- status API responds while maintenance is disabled.

M4.16 is qualified only when all normal CI and historical-statistics gates are green on the final commit.
