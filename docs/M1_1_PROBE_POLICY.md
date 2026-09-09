# M1.1 — Probe identity, target policy and rate limits

M1.1 makes the probe engine safe-by-default before any live public-network qualification.

## Identity

Every probe runner requires an explicit IRC nick, username and operator contact string. The contact is included in the IRC realname so network operators can identify and reach the probe operator.

## Target policy

Targets are deny-by-default. A runner cannot be created without an allowlist, and a host must match the allowlist before DNS or network I/O starts. Explicit deny rules override allow rules. Wildcard rules are limited to suffix-style entries such as `*.example.net`.

## Rate limiting

Each runner tracks the last attempt per host. Repeated probes inside `MinInterval` are rejected locally before network I/O. If not configured, the interval defaults to five minutes.

## Privacy boundary

These controls do not change the M1 privacy model: probes do not join channels or collect message content, user histories, private-channel data or credentials.

## Live qualification gate

Public-network probing remains disabled operationally until a reviewed deployment configuration supplies the actual IRCIntel contact identity and an explicit target allowlist. The first live qualification should use a small set of networks whose public connection policies permit automated measurement.
