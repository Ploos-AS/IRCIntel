# M3.24 — Configurable status freshness

IRCIntel now exposes the freshness window used by endpoint and network status aggregation as runtime configuration.

## Configuration

`IRCINTEL_STATUS_FRESHNESS` accepts a positive Go duration such as `5m`, `15m`, or `1h`.

Default: `15m`.

Invalid, zero, or negative values make Core fail closed during startup instead of silently changing status semantics.

The same configured freshness value is supplied to both:

- `GET /api/v1/endpoints/status`
- `GET /api/v1/networks/status`

This keeps endpoint and network aggregation consistent.

The Compose and Podman Quadlet examples explicitly set the default `15m` value so deployments can discover and override it without changing the image.

## Compatibility

There is no HTTP or JSON schema change. Deployments that do not set `IRCINTEL_STATUS_FRESHNESS` retain the existing 15-minute behavior.
