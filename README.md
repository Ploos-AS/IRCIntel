# IRCIntel

Open intelligence for the IRC ecosystem.

IRCIntel is a privacy-conscious platform for discovering, measuring, and presenting technical intelligence about public IRC networks.

## M0 scope

M0 establishes the project foundation:

- Go service skeleton
- health and version endpoints
- configuration via environment variables
- minimal OCI image baseline
- Docker Compose and Podman Quadlet examples
- CI for build and tests
- architecture and privacy principles

IRCIntel measures infrastructure and public network metadata. It is **not** intended to collect private messages or archive conversation content.

## Planned capabilities

- network and server directory
- distributed IRC probes
- TCP/TLS/IRC registration latency
- IPv4/IPv6 and TLS intelligence
- IRCv3 capability discovery
- netsplit and incident detection
- channel directory using public metadata
- historical statistics
- public API and open datasets

## Development

```sh
go test ./...
go run ./cmd/ircintel
```

The service listens on `IRCINTEL_LISTEN` (default `:8080`).

Endpoints:

- `GET /healthz`
- `GET /api/v1/version`

## License

MIT
