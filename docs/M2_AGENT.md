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

## M2.1 Core submission transport

When `IRCINTEL_CORE_URL` is unset, the agent emits one NDJSON observation per endpoint to stdout. When configured, each observation is POSTed as `application/json` to Core.

The HTTP transport accepts 2xx responses, supports an optional bearer token, uses a bounded timeout, retries failures with exponential backoff, remains context-cancellable, and returns the final error instead of silently dropping observations.

## M2.2 Core observation ingestion

M2.2 closes the first agent-to-Core path with `POST /api/v1/observations`.

Core now:

- accepts the M2 observation envelope
- optionally requires the same bearer token through `IRCINTEL_CORE_TOKEN`
- rejects malformed JSON and unknown fields
- rejects observations without agent ID, timestamp, or endpoint host
- limits request bodies to 1 MiB
- returns `202 Accepted` only after the observation store accepts the measurement
- uses an `ObservationStore` interface so durable persistence can replace the initial in-memory implementation without changing the HTTP contract

The initial `MemoryStore` is intentionally non-durable. It proves and qualifies the ingestion boundary; database persistence is the next storage milestone rather than being hidden inside the transport work.

## Configuration

Required agent environment variables:

- `IRCINTEL_AGENT_ID`: stable logical probe identifier, for example `oslo-1`
- `IRCINTEL_AGENT_CONTACT`: public operator/contact URL included in the IRC probe identity
- `IRCINTEL_AGENT_TARGETS`: JSON array of endpoint objects

Optional agent environment variables:

- `IRCINTEL_AGENT_INTERVAL`: measurement interval, default `5m`
- `IRCINTEL_AGENT_NICK`: IRC probe nick, default `IRCIntelProbe`
- `IRCINTEL_AGENT_USERNAME`: IRC probe username, default `ircintel`
- `IRCINTEL_CORE_URL`: Core observation ingestion URL; unset means stdout mode
- `IRCINTEL_CORE_TOKEN`: bearer token used by the agent and, when set on Core, required by ingestion
- `IRCINTEL_AGENT_RETRIES`: retry count after the initial submission attempt, default `3`

Example target configuration:

```json
[
  {"host":"irc.example.net","port":"6697","tls":true},
  {"host":"irc.example.net","port":"6667","tls":false}
]
```

The endpoint host set is also used to construct the probe allowlist. The probe engine remains default-deny and the agent cannot measure arbitrary hosts that were not explicitly configured.

## Observation envelope

Each completed endpoint measurement contains `agent_id`, UTC `observed_at`, the configured endpoint, and the full M1 `EndpointResult`. Endpoint results retain separate IPv4/IPv6 measurements, stable failure codes/stages, latency data, TLS metadata, IRCv3 capabilities, and passive server metadata.

## Runtime behavior

The agent performs one measurement cycle immediately on startup, then repeats at the configured interval. A cycle runs every configured endpoint once. The M1 runner policy uses the same interval as its minimum per-host rate limit. SIGINT or SIGTERM cancels the agent context and exits cleanly.

## Privacy and safety

The M2 agent inherits the M1 privacy model. It does not join channels, collect message content, enumerate users, or request private data. Targets must be explicitly configured and the probe identity carries a contact URL.

## Qualification

M2 qualification verifies agent serialization/configuration, HTTP submission/authentication/retry behavior, and Core ingestion authentication, schema rejection, required envelope fields, and successful storage.

CI must continue to pass `go test ./...`, `go vet ./...`, application builds, and the OCI runtime gate.
