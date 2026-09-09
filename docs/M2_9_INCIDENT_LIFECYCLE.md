# M2.9 — distributed incident lifecycle

M2.9 turns correlated outage/recovery events into incident lifecycles.

`GET /api/v1/incidents/lifecycle` reads recent observations, derives per-agent reachability transitions, correlates transitions across multiple agents using the same distributed incident rules as M2.8, then pairs a correlated `down` with the next correlated `recovered` event for the same `(host, port, tls)` endpoint.

Each lifecycle reports:

- endpoint identity (`host`, `port`, `tls`)
- `status`: `open` or `closed`
- `started_at`
- `recovered_at` when closed
- `duration_seconds` when closed
- agents contributing to the correlated outage
- agents contributing to the correlated recovery

Open incidents intentionally omit recovery time and duration. A recovery without an earlier correlated outage is ignored rather than creating a synthetic incident.

The endpoint accepts the M2.8 `window` bounds (`30s` through `15m`, default `5m`) and the standard incident `limit` (`1..500`, default `100`). It uses the same optional Core bearer authentication as the other read APIs.

Qualification covers open and closed lifecycles, deterministic duration calculation, distributed agent membership, handler serialization, and the existing repository test/vet/build and OCI runtime gates.
