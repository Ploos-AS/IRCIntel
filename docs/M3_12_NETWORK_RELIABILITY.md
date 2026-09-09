# M3.12 — Network reliability ranking

M3.12 adds a deterministic, web-ready comparison view over curated IRC networks.

## API

`GET /api/v1/networks/reliability`

Optional query parameters:

- `since=<RFC3339Nano>`
- `until=<RFC3339Nano>`

The response contains every curated network, including networks with zero recorded incidents, and assigns a deterministic rank.

## Ordering

The initial ranking deliberately uses only observed incident burden:

1. lower total closed-incident downtime
2. fewer `down` incidents
3. fewer total incidents
4. network name
5. network ID

This is an operational comparison, not yet a statistically normalized availability score. Networks can have different observation coverage, so IRCIntel must not present this initial rank as a universal uptime percentage or SLA claim.

A later milestone should add coverage-aware availability/scoring before stronger public reliability claims are made.

## Trust and privacy

Only curated registry networks participate. Unreviewed or accepted-but-unpromoted discovery candidates remain outside the public intelligence model. The endpoint uses incident metadata only and does not introduce collection of IRC conversation content, private messages, nick histories, credentials, or private-channel content.
