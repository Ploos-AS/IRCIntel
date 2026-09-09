# M3.0 — network registry foundation

M3.0 introduces the persistent identity layer that later discovery, search, topology, and netsplit analysis will build on.

## Model

IRCIntel now distinguishes three registry entities:

- `Network`: the logical IRC network, with stable ID, display name, optional website, and description.
- `NetworkServer`: a named server or server-group identity belonging to one network.
- `NetworkEndpoint`: a concrete host/port/TLS target belonging to one server identity.

The SQLite schema persists these as `networks`, `network_servers`, and `network_endpoints`. IDs are explicit strings rather than database-generated integers so discovery and operator-supplied metadata can converge on stable identities later.

Endpoint hostnames are normalized to lowercase. Parent existence is validated before inserting servers or endpoints. Upserts update metadata while retaining stable IDs.

## API

`GET /api/v1/registry` returns a deterministic snapshot containing `networks`, `servers`, and `endpoints`. It uses the same optional `IRCINTEL_CORE_TOKEN` bearer authentication as the other internal Core APIs.

M3.0 intentionally does not expose registry mutation over HTTP. Initial population, discovery ingestion, operator claims, and provenance are separate milestones so the identity model can be qualified first.

## Qualification

Tests verify:

- network → server → endpoint persistence;
- hostname normalization;
- persistence across SQLite reopen;
- rejection of missing parent identities;
- authenticated registry read API behavior.

Existing Go tests, vet, build, dependency cleanliness, and OCI runtime gates remain mandatory.
