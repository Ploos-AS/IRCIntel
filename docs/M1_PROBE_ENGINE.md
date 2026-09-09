# M1 — IRC probe engine

M1 introduces IRCIntel's first protocol-aware measurement component.

## Scope

A probe performs, in order:

1. DNS resolution of the configured IRC host.
2. Explicit address-family selection: automatic, IPv4-only (`tcp4`) or IPv6-only (`tcp6`).
3. TCP connection and latency measurement to the selected address.
4. Optional TLS handshake with certificate verification and TLS 1.2 minimum.
5. TLS metadata capture for verified connections: negotiated TLS version, cipher suite, leaf certificate issuer/subject, validity interval and SAN DNS/IP entries.
6. IRC registration using a clearly identified probe identity.
7. IRCv3 capability discovery with `CAP LS 302`, including multiline responses and `CAP END`.
8. Registration-time `PING` handling with corresponding `PONG`.
9. Successful-registration detection from numeric `001`.
10. Clean `QUIT` after the measurement.

The structured result records all resolved addresses, the selected address, the requested/normalized address family, stage latencies, TLS metadata when applicable, the responding server name and advertised IRCv3 capabilities.

## Address-family behavior

`Config.Family` accepts automatic/any mode plus explicit IPv4 and IPv6 aliases. Explicit IPv4 probes select an A-derived address and dial with `tcp4`; explicit IPv6 probes select an AAAA-derived address and dial with `tcp6`. If the requested family is unavailable, the probe fails instead of silently falling back to the other family. TLS verification continues to use the configured hostname/SNI rather than the selected literal IP address.

This behavior lets IRCIntel distinguish, for example, a network whose IPv4 endpoint is healthy while IPv6 is unreachable or materially slower.

## Safety and privacy

The M1 probe does not join channels and does not collect `PRIVMSG`, `NOTICE`, channel messages, user histories or credentials. It performs only the minimum protocol exchange needed to measure a public IRC endpoint.

TLS metadata is limited to information already presented by the public endpoint during the verified handshake. IRCIntel does not disable certificate verification to collect metadata from an invalid endpoint.

## Qualification

M1 is qualified by deterministic local tests using synthetic IRC listeners. Protocol regression tests require correct CAP negotiation, multiline CAP handling and PING/PONG behavior. TLS metadata extraction is separately tested from a deterministic connection state/certificate fixture.

Address-family qualification covers explicit IPv4 selection/dialing and an explicit IPv6 loopback registration probe when IPv6 loopback is available on the CI runner. Helper tests also verify family normalization, address selection and no-family-available failure behavior.

CI must continue to pass `go test ./...`, `go vet ./...`, the application build, and the M0.1 OCI runtime gate.

The registration path has also been live-qualified against the approved public target `irc.libera.chat:6697` using TLS and the identified IRCIntel probe identity. Live qualification remains an explicit/manual action; it is not run on every push.
