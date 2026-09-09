# M2 — distributed probe agent

M2 introduces the lightweight agent layer that runs IRCIntel measurements on remote probe nodes.

## M2.0 scope

The initial agent foundation provides:

- a dedicated `ircintel-agent` command
- environment-based configuration
- explicit agent identity
- one or more configured IRC endpoints
- a periodic scheduler
- reuse of the M1 endpoint probe engine
- normalized newline-delimited JSON observations on stdout
- graceful SIGINT/SIGTERM shutdown

M2.0 intentionally does not define the Core submission protocol yet. Keeping the first output boundary as structured JSON makes the measurement envelope testable without prematurely freezing authentication, queueing, retry, or transport semantics.

## Configuration

Required environment variables:

- `IRCINTEL_AGENT_ID`: stable logical probe identifier, for example `oslo-1`
- `IRCINTEL_AGENT_CONTACT`: public operator/contact URL included in the IRC probe identity
- `IRCINTEL_AGENT_TARGETS`: JSON array of endpoint objects

Optional environment variables:

- `IRCINTEL_AGENT_INTERVAL`: measurement interval, default `5m`
- `IRCINTEL_AGENT_NICK`: IRC probe nick, default `IRCIntelProbe`
- `IRCINTEL_AGENT_USERNAME`: IRC probe username, default `ircintel`

Example target configuration:

```json
[
  {"host":"irc.example.net","port":"6697","tls":true},
  {"host":"irc.example.net","port":"6667","tls":false}
]
```

The endpoint host set is also used to construct the probe allowlist. The probe engine therefore remains default-deny and the agent cannot measure arbitrary hosts that were not explicitly configured.

## Observation envelope

Each completed endpoint measurement emits one JSON object containing:

- `agent_id`
- UTC `observed_at`
- the configured endpoint
- the full M1 `EndpointResult`

Endpoint results retain separate IPv4/IPv6 measurements, stable failure codes/stages, latency data, TLS metadata, IRCv3 capabilities, and passive server metadata.

## Runtime behavior

The agent performs one measurement cycle immediately on startup, then repeats at the configured interval. A cycle runs every configured endpoint once. The M1 runner policy uses the same interval as its minimum per-host rate limit.

SIGINT or SIGTERM cancels the agent context and exits cleanly.

## Privacy and safety

The M2 agent inherits the M1 privacy model. It does not join channels, collect message content, enumerate users, or request private data. Targets must be explicitly configured and the probe identity carries a contact URL.

## Qualification

M2.0 tests verify:

- deterministic observation serialization
- propagation of endpoint configuration to the probe runner
- agent identity and timestamp fields
- required configuration validation
- default nick/username behavior
- target-derived allowlist construction and host deduplication

CI must continue to pass `go test ./...`, `go vet ./...`, application builds, and the OCI runtime gate.
