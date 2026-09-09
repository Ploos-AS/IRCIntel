# M3.27 — Checkpoint-bounded endpoint incident replay

M3.27 reduces persisted endpoint-incident replay after an endpoint has already produced an incident lifecycle.

## Behaviour

The first persisted incident for an endpoint still bootstraps from the endpoint's complete observation history. Once an `incident_records` lifecycle exists, later ingest uses the newest persisted lifecycle as a replay checkpoint.

The replay starts one default correlation window before that lifecycle's `started_at`. IRCIntel then includes all observations at or after the replay anchor plus exactly one earlier baseline observation for each agent that is active in the replay window. This preserves transition detection at the window boundary without carrying unrelated historical agent state forward indefinitely.

Agents with no observations in the replay window do not contribute a baseline row. Ancient malformed payloads belonging only to inactive agents therefore no longer block later ingest for the endpoint.

## Correctness

The persisted lifecycle format, distributed correlation window, transaction boundaries, and HTTP APIs are unchanged. Baseline observations are retained so an agent whose previous state predates the replay anchor can still produce a correct down or recovered transition inside the replay window.

If the newest persisted lifecycle itself is malformed, or an observation needed by the replay window/baseline is malformed, ingest still fails closed and the M3.21 atomic transaction rolls back.

## Remaining work

This is bounded replay, not fully event-driven incident state. An unusually long latest lifecycle can still require a substantial replay window. A future milestone can persist compact per-agent transition/correlation state and remove most replay work entirely.
