# M3.1 Registry write API

M3.1 adds controlled administrative mutation of the M3.0 network registry.

## Endpoints

All write endpoints require `IRCINTEL_CORE_TOKEN` to be configured and require the matching `Authorization: Bearer <token>` header.

- `POST /api/v1/registry/networks`
- `POST /api/v1/registry/servers`
- `POST /api/v1/registry/endpoints`

Registry writes are deliberately disabled when no Core token is configured. This differs from read-only APIs, which may be exposed without a token in development deployments.

## Semantics

Each endpoint accepts exactly one JSON object and rejects unknown fields. Request bodies are limited to 64 KiB.

Writes are idempotent UPSERT operations keyed by the stable registry object ID introduced in M3.0.

Parent relationships are validated:

- a server requires an existing network;
- an endpoint requires an existing server.

A missing parent returns HTTP 409 Conflict. Invalid payloads return HTTP 400. Authentication failures return HTTP 401. Storage failures return HTTP 503.

Endpoint hostnames are normalized to lowercase and surrounding whitespace is removed from registry identifiers and other string fields before storage.

## Scope

M3.1 is an administrative Core API. It does not yet implement public submissions, operator claims, automated discovery, moderation workflow, or provenance metadata. Those belong to later M3 milestones.
