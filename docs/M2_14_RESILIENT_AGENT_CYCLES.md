# M2.14 — resilient agent cycles

M2.14 makes endpoint probe failures first-class observations instead of fatal agent-cycle errors.

## Behavior

`Agent.RunOnce` now continues across configured endpoints when `RunEndpoint` returns a probe error. The failed endpoint is submitted with:

- the endpoint identity
- the partial `EndpointResult`, when available
- `error`
- structured `error_code`
- structured `error_stage`

The next endpoint is still probed and submitted in the same cycle.

Transport/submission failures remain fatal for the cycle. If Core cannot receive an observation, the agent stops instead of silently skipping delivery; durable delivery/queueing is a later concern.

Configuration errors that prevent the cycle from starting remain fatal as before.

## Qualification

Tests verify that:

- successful observations remain unchanged
- a structured DNS probe failure is emitted as data
- the following endpoint still runs and is submitted
- submission failure stops the cycle immediately

Existing Go tests, vet, dependency cleanliness, build, and OCI runtime gates remain mandatory.
