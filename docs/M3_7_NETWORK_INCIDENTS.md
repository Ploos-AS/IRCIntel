# M3.7 — Network-level incident detection

M3.7 derives network incidents from the persisted endpoint incident lifecycle and the curated registry hierarchy.

## API

`GET /api/v1/networks/incidents`

Optional filters:
- `status=open|closed`
- `network_id=<registry network id>`
- `limit=1..500`

The endpoint follows the same optional bearer-token policy as the rest of Core.

## Detection model

Only curated registry endpoints participate. Endpoint incidents are mapped through endpoint → server → network.

For each network:
- the first concurrently affected endpoint opens a network incident;
- an incident starts as `degraded`;
- if all curated endpoints are concurrently affected, severity is promoted to `down`;
- `affected_endpoints` records the peak concurrent number of affected endpoints;
- the incident closes when the final affected endpoint recovers;
- duration is measured from first endpoint-down to final endpoint recovery.

This means overlapping endpoint incidents are represented as one network-level operational incident rather than unrelated endpoint failures.

## Trust boundary

Discovery candidates cannot create network incidents unless they have been reviewed, explicitly promoted, and therefore exist in the curated registry.

## Scope

M3.7 derives network incidents from persisted endpoint incident records at query time. Dedicated persistence, acknowledgement, operator annotations, and public incident presentation are deferred to later milestones.
