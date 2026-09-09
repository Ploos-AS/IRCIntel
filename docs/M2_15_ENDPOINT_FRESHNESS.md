# M2.15 — endpoint freshness semantics

M2.15 prevents obsolete agent observations from affecting current endpoint health indefinitely.

`GET /api/v1/endpoints/status` now evaluates only fresh latest-per-agent observations when calculating `up`, `degraded`, and `down`. The default freshness window is 15 minutes. Tests may inject a deterministic clock and an alternate freshness duration through `StatusHandler`.

The response now includes `stale_agents`. Stale observations remain visible as historical evidence but do not increment `agents`, `reachable_agents`, or `dual_stack_agents`. If an endpoint has no fresh agents at all, its status is `stale` rather than `up` or `down`.

This keeps retired, disconnected, or failed probe agents from permanently skewing the current health state while preserving visibility that stale measurements exist.

## Qualification

M2.15 qualification covers mixed fresh/stale agents, fully stale endpoints, existing up/down/degraded behavior, authentication, Go tests/vet/build, dependency cleanliness, and OCI runtime smoke qualification.
