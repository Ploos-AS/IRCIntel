# M1 — IRC probe engine

M1 introduces IRCIntel's first protocol-aware measurement component.

## Scope

A probe performs, in order:

1. DNS resolution of the configured IRC host.
2. Explicit address-family selection: automatic, IPv4-only (`tcp4`) or IPv6-only (`tcp6`).
3. TCP connection and latency measurement with deterministic fallback across all resolved addresses in the selected family.
4. Optional TLS handshake with certificate verification and TLS 1.2 minimum.
5. TLS metadata capture for verified connections: negotiated TLS version, cipher suite, leaf certificate issuer/subject, validity interval and SAN DNS/IP entries.
6. IRC registration using a clearly identified probe identity.
7. IRCv3 capability discovery with `CAP LS 302`, including multiline responses and `CAP END`.
8. Registration-time `PING` handling with corresponding `PONG`.
9. Passive capture of public registration metadata from numerics such as `004` and `005 ISUPPORT`.
10. Successful-registration detection from numeric `001`.
11. Clean `QUIT` after the measurement.

The structured result records all resolved addresses, the address that ultimately succeeded, the requested/normalized address family, stage latencies, TLS metadata when applicable, the responding server name, advertised IRCv3 capabilities, and public server metadata announced during registration.

## Address-family behavior

`Config.Family` accepts automatic/any mode plus explicit IPv4 and IPv6 aliases. Explicit IPv4 probes retain all A-derived addresses and dial them with `tcp4`; explicit IPv6 probes retain all AAAA-derived addresses and dial them with `tcp6`. Candidate addresses are tried in the deterministic order produced by the sorted DNS result until one TCP connection succeeds or all candidates fail. `selected_address` identifies the address that actually succeeded.

If the requested family has no usable address at all, the probe fails with the corresponding no-address code. If addresses exist but every TCP attempt fails, the probe returns `tcp_connect_failed`. It never silently falls back from an explicit IPv4 probe to IPv6 or vice versa. TLS verification continues to use the configured hostname/SNI rather than the selected literal IP address.

This behavior prevents a single dead A or AAAA record from producing a false family outage when another address in the same family is healthy, while still letting IRCIntel distinguish, for example, IPv4 health from IPv6 health.

## Endpoint measurement model

`Runner.RunEndpoint` represents one scheduled measurement of an IRC endpoint as two explicit family observations: IPv4 and IPv6. The parent runner performs the allowlist/denylist and rate-limit decision once for the endpoint, then the two family sub-probes run independently.

A family failure is measurement data, not an endpoint-level execution error. Each family observation records `ok`, the probe result when available, a human-readable error, plus stable `error_code` and `error_stage` values when that family fails. The aggregate endpoint result uses:

- `reachable=true` when at least one family succeeds.
- `dual_stack_ok=true` only when both IPv4 and IPv6 succeed.
- a stable measurement order: IPv4 first, IPv6 second.

This means an endpoint can correctly be represented as IPv4 healthy / IPv6 failed without losing the successful IPv4 latency, TLS, server, and IRCv3 measurements.

## Structured failure taxonomy

Probe-stage failures use stable machine-readable codes instead of requiring consumers to parse human-readable error strings. Current codes include:

- `invalid_config`
- `target_denied`
- `target_not_allowed`
- `rate_limited`
- `dns_lookup_failed`
- `no_ipv4_address`
- `no_ipv6_address`
- `no_address`
- `tcp_connect_failed`
- `tls_handshake_failed`
- `irc_registration_write_failed`
- `irc_pong_write_failed`
- `irc_cap_end_write_failed`
- `irc_read_failed`
- `irc_registration_failed`

Each classified error also carries a stable stage such as `config`, `policy`, `dns`, `address_selection`, `tcp`, `tls`, or `irc_registration`. Human-readable error text remains available for diagnostics, but aggregation and incident detection should use the stable code/stage fields.

## IRC server metadata

M1.7 records only metadata the server already announces during the normal registration exchange. No additional WHO, WHOIS, LIST, STATS, LINKS, or channel/user enumeration is performed.

`ServerMetadata` currently captures:

- `network` from the `NETWORK=` ISUPPORT token when present.
- `software` and `software_version` from numeric `004` when supplied by the server.
- the complete public `005 ISUPPORT` token map, including values such as `CHANTYPES`, `PREFIX`, `CASEMAPPING`, `CHANMODES`, `NICKLEN`, `TOPICLEN`, and similar implementation-advertised features.

ISUPPORT is stored as key/value data so later core components can compare protocol features across networks without parsing raw IRC lines. Missing metadata remains absent rather than guessed.

## Safety and privacy

The M1 probe does not join channels and does not collect `PRIVMSG`, `NOTICE`, channel messages, user histories or credentials. It performs only the minimum protocol exchange needed to measure a public IRC endpoint.

TLS metadata is limited to information already presented by the public endpoint during the verified handshake. IRCIntel does not disable certificate verification to collect metadata from an invalid endpoint.

## Qualification

M1 is qualified by deterministic local tests using synthetic IRC listeners. Protocol regression tests require correct CAP negotiation, multiline CAP handling and PING/PONG behavior. TLS metadata extraction is separately tested from a deterministic connection state/certificate fixture.

Address-family qualification covers explicit IPv4 selection/dialing and an explicit IPv6 loopback registration probe when IPv6 loopback is available on the CI runner. Helper tests also verify family normalization, address selection and no-family-available failure behavior.

Multi-address fallback qualification verifies that family filtering preserves candidate order, that a failed first address advances to the next address, that the successful address is reported, and that all-address failure returns the last connection failure.

Endpoint-model qualification covers mixed-family outcomes and verifies that one successful family keeps the endpoint reachable while `dual_stack_ok` remains false. It also verifies that the parent endpoint measurement consumes one host rate-limit slot.

Failure-taxonomy qualification locks stable code/stage mappings for address-selection, policy, and TCP failures and verifies that endpoint family observations expose those fields.

Server-metadata qualification verifies deterministic parsing of numeric `004`, `005 ISUPPORT`, `NETWORK=`, and the structured ISUPPORT map, plus end-to-end capture during a synthetic registration exchange.

CI must continue to pass `go test ./...`, `go vet ./...`, the application build, and the M0.1 OCI runtime gate.

The registration path has also been live-qualified against the approved public target `irc.libera.chat:6697` using TLS and the identified IRCIntel probe identity. Live qualification remains an explicit/manual action; it is not run on every push.
