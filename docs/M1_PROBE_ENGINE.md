# M1 — IRC probe engine

M1 introduces IRCIntel's first protocol-aware measurement component.

## Scope

A probe performs, in order:

1. DNS resolution of the configured IRC host.
2. TCP connection and latency measurement.
3. Optional TLS handshake with certificate verification and TLS 1.2 minimum.
4. IRC registration using a clearly identified probe identity.
5. IRCv3 capability discovery with `CAP LS 302`.
6. Successful-registration detection from numeric `001`.
7. Clean `QUIT` after the measurement.

The structured result records resolved addresses, stage latencies, the responding server name and advertised IRCv3 capabilities.

## Safety and privacy

The M1 probe does not join channels and does not collect `PRIVMSG`, `NOTICE`, channel messages, user histories or credentials. It performs only the minimum protocol exchange needed to measure a public IRC endpoint.

## Qualification

M1 is qualified by deterministic local tests using a synthetic IRC listener. CI must continue to pass `go test ./...`, `go vet ./...`, the application build, and the M0.1 OCI runtime gate.

Live public-network qualification is deliberately deferred until probe rate limits, identity/contact metadata and target policy are explicit.
