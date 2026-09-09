# M1.2 — controlled live qualification

M1.2 adds an explicit, opt-in path for qualifying the IRC probe engine against a real public IRC endpoint.

## Approved target

Initial approved target:

- `irc.libera.chat:6697` with TLS enabled.

This endpoint is documented by Libera.Chat for normal IRC client access.

## Controls

- Live probing is **not** part of normal push/PR CI.
- The GitHub Actions workflow uses `workflow_dispatch` only.
- The workflow has an explicit target allowlist and rejects other hosts.
- The probe identifies itself as `IRCIntelProbe` / `ircintel`.
- The realname contains the IRCIntel project contact URL.
- One workflow invocation performs one registration-only probe.
- The probe does not join channels, authenticate, send messages, enumerate users, or collect conversation content.
- TLS certificate verification remains enabled.

## Expected result

A successful run prints one JSON result containing DNS, TCP, TLS and registration latency, resolved addresses, the responding server name and advertised IRCv3 capabilities.

The qualification is considered PASS when the manually dispatched workflow completes successfully against the approved target.
