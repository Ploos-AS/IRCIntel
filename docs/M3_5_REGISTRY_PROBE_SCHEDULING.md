# M3.5 — Registry-driven probe scheduling

M3.5 connects the curated registry to distributed agent startup without weakening the discovery trust boundary.

## Core probe plan

Core exposes:

- `GET /api/v1/probe-plan?agent_id=<id>`

The plan is derived only from `network_endpoints` in the curated registry. Discovery candidates, reviews, and accepted-but-not-promoted candidates are never included.

The plan is deterministic and contains endpoint ID, host, port, and TLS state. The current M3.5 scheduler assigns the full curated endpoint set to each requesting agent; location-aware sharding is intentionally deferred.

## Agent behavior

`IRCINTEL_AGENT_TARGETS` remains supported for static deployments.

When `IRCINTEL_AGENT_TARGETS` is absent and `IRCINTEL_CORE_URL` is configured, `ircintel-agent` fetches its probe plan from Core at startup using `IRCINTEL_AGENT_ID` and the Core bearer token. The fetched endpoints are then used to construct the probe allowlist and scheduler.

The startup plan is frozen for the lifetime of the process in M3.5. Dynamic refresh is a later milestone so plan changes cannot surprise a running probe fleet.

## Qualification

Tests cover:

- probe plans contain only curated registry endpoints;
- deterministic endpoint ordering;
- Core auth and required agent ID;
- agent plan retrieval with bearer auth;
- empty plans fail closed;
- static `IRCINTEL_AGENT_TARGETS` remains compatible;
- Core-backed configuration is accepted when static targets are absent.
