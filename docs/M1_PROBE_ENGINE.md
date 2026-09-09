# M1 — IRC probe engine

M1 introduces IRCIntel's first protocol-aware measurement component.

## Scope

A probe performs, in order:

1. DNS resolution of the configured IRC host.
2. TCP connection and latency measurement.
3. Optional TLS handshake with certificate verification and TLS 1.2 minimum.
4. TLS metadata capture for verified connections: negotiated TLS version, cipher suite, leaf certificate issuer/subject, validity interval and SAN DNS/IP entries.
5. IRC registration using a clearly identified probe identity.
6. IRCv3 capability discovery with `CAP LS 302`, including multiline responses and `CAP END`.
7. Registration-time `PING` handling with corresponding `PONG`.
8. Successful-registration detection from numeric `001`.
9. Clean `QUIT` after the measurement.

The structured result records resolved addresses, stage latencies, TLS metadata when applicable, the responding server name and advertised IRCv3 capabilities.

## Safety and privacy

The M1 probe does not join channels and does not collect `PRIVMSG`, `NOTICE`, channel messages, user histories or credentials. It performs only the minimum protocol exchange needed to measure a public IRC endpoint.

TLS metadata is limited to information already presented by the public endpoint during the verified handshake. IRCIntel does not disable certificate verification to collect metadata from an invalid endpoint.

## Qualification

M1 is qualified by deterministic local tests using a synthetic IRC listener. Protocol regression tests require correct CAP negotiation, multiline CAP handling and PING/PONG behavior. TLS metadata extraction is separately tested from a deterministic connection state/certificate fixture.

CI must continue to pass `go test ./...`, `go vet ./...`, the application build, and the M0.1 OCI runtime gate.

The registration path has also been live-qualified against the approved public target `irc.libera.chat:6697` using TLS and the identified IRCIntel probe identity. Live qualification remains an explicit/manual action; it is not run on every push.
