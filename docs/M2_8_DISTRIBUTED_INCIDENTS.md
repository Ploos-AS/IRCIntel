# M2.8 — distributed incident correlation

M2.8 correlates the per-agent transition events introduced in M2.7 into distributed endpoint incidents.

## API

`GET /api/v1/incidents/distributed`

The handler derives `down` and `recovered` transition events from recent observations, groups events by `(type, host, port, tls)`, and correlates transitions reported by different agents inside a bounded time window.

A distributed incident requires at least two distinct agents. A single regional failure therefore remains a probe event and is not promoted to a distributed incident.

Each incident exposes:

- event type (`down` or `recovered`)
- endpoint host, port, and TLS mode
- first and last correlated event timestamps
- sorted contributing agent IDs
- distinct agent count

## Correlation window

The default correlation window is `5m`.

The API accepts `window=<duration>` with bounds from `30s` through `15m`. It also accepts `limit=<1..500>`, default `100`.

Events separated by more than the configured window start a new cluster. Down and recovery events are correlated independently so that recovery cannot accidentally merge into an outage cluster.

## Semantics

This milestone deliberately performs endpoint-level distributed correlation only. It does not yet claim that an endpoint outage is a network-wide IRC outage or a netsplit. Those classifications require topology and multi-endpoint/network evidence in later milestones.

The endpoint uses the same optional `IRCINTEL_CORE_TOKEN` bearer authentication as other Core read APIs.

## Qualification

Tests cover:

- at least two distinct agents are required
- single-agent events are not promoted
- endpoint identity is respected
- down/recovered events remain separate
- correlation windows split distant events
- deterministic sorted agent IDs
- bearer authentication
- invalid correlation-window rejection

CI must continue to pass dependency cleanliness, `go test ./...`, `go vet ./...`, Core build, and OCI runtime smoke qualification.
