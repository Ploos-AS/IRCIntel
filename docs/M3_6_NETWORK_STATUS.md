# M3.6 — Network-level status aggregation

IRCIntel now aggregates fresh endpoint observations through the curated registry hierarchy into a network-level operational status.

## API

`GET /api/v1/networks/status`

Each network reports its curated endpoint count, up/degraded/down/stale endpoint counts, latest observation time, and overall status.

## Status semantics

Endpoint measurements use the same 15-minute default freshness boundary as `/api/v1/endpoints/status`.

A curated endpoint is:
- `stale` when it has no fresh agent observations;
- `down` when fresh agents exist and none report it reachable;
- `up` when every fresh agent reports it reachable;
- `degraded` otherwise.

A network is:
- `stale` when it has no curated endpoints or all curated endpoints are stale;
- `down` when every curated endpoint is down;
- `up` when every curated endpoint is up;
- `degraded` for every mixed state, including partial outages and mixtures involving stale endpoints.

Only endpoints attached to servers and networks in the curated registry participate. Discovery candidates that have not been explicitly promoted cannot affect network status.

## Scope

M3.6 deliberately does not infer network identity from IRC protocol metadata, does not weight endpoints, and does not introduce network-level incident persistence. Those can be layered on after the aggregation contract is qualified.
