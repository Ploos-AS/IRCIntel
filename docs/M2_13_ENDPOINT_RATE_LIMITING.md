# M2.13 — endpoint-aware rate limiting

M2.13 fixes scheduled probing when one IRC host exposes multiple endpoints.

The policy allow/deny decision remains hostname-based, but the rate-limit slot for `RunEndpoint` is now keyed by normalized host, effective port, and TLS mode. This lets an agent probe, for example, `irc.example:6697` with TLS and `irc.example:6667` without TLS in the same scheduled cycle without creating a false `rate_limited` result.

IPv4 and IPv6 measurements remain part of the same scheduled endpoint operation and therefore consume only the parent endpoint slot. Explicit and implicit default ports are normalized to the same endpoint identity (`6697` for TLS and `6667` for cleartext).

Qualification covers independent slots for different ports and TLS modes, rate limiting for repeated identical endpoints, case-normalized host identity, and equivalence between implicit and explicit default ports. Existing probe, agent, Core, dependency-cleanliness, vet, build, and OCI runtime gates remain mandatory.
