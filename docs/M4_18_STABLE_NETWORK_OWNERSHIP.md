# M4.18 — Stable historical network ownership foundation

M4.13 and M4.14 deliberately exclude endpoint tuples that are currently shared by multiple networks, but their attribution still depends on the **current registry**. If an endpoint or server is moved to another network later, old historical statistics can therefore be interpreted using the new ownership.

M4.18 introduces the durable ownership journal required to remove that retroactive behavior.

## Ownership intervals

PostgreSQL now maintains `endpoint_network_ownership` with immutable historical intervals for:

- endpoint ID
- server ID
- network ID
- host
- port
- TLS mode
- `valid_from`
- optional `valid_to`

Exactly one active interval is allowed for an endpoint ID.

Registry mutations are transactional with the ownership journal:

- creating an endpoint opens its first ownership interval;
- moving an endpoint to another server closes the previous interval and opens a new one;
- changing endpoint host, port or TLS closes the previous interval and opens a new one;
- moving a server to another network closes and reopens ownership for every endpoint on that server;
- repeating an identical upsert does not create another interval.

## Upgrade bootstrap

The first ownership-aware registry operation creates the ownership schema and freezes the existing registry as the baseline. Existing endpoints receive an open interval beginning at PostgreSQL `-infinity`.

This is intentionally a compatibility baseline, not a claim that IRCIntel knows registry ownership transitions that happened before M4.18 was deployed.

A singleton bootstrap marker prevents later restarts from reinterpreting that baseline.

## What M4.18 fixes

From this milestone onward, registry ownership changes are durably recorded instead of overwriting the only available ownership state. The journal survives Core restarts and provides the stable identity/time model needed for long-lived public statistics.

## What M4.18 does not yet change

The M4.13 network-history API and M4.14 ranking API still read the existing endpoint rollups using the current-registry ambiguity guard. M4.18 is the persistence foundation; a following milestone must build/use ownership-aware network rollups so those public historical APIs are driven by the journal rather than current registry state.

This staged approach avoids silently changing historical aggregation semantics before the ownership history itself has been qualified.

## Qualification

M4.18 is qualified when all ordinary CI gates remain green and the dedicated `Ownership History` runtime workflow passes against PostgreSQL 17. That workflow drives registry mutations through the real Core HTTP API and verifies in PostgreSQL that:

1. the endpoint begins in Network A;
2. moving its server to Network B closes the Network A interval and opens a Network B interval;
3. changing endpoint identity closes the second interval and opens a third;
4. exactly the final interval remains active;
5. all intervals remain present after Core restart.
