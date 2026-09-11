# M4.14 — Network history ranking and availability trends

M4.14 turns the M4.13 network-scoped historical series into a comparison surface suitable for a public network ranking page.

## API

`GET /api/v1/history/networks/ranking?period=<24h|7d|30d|1y|all>&limit=<1..500>`

The response is sorted by current reachable percentage, then observation count, then network name.

For bounded periods (`24h`, `7d`, `30d`, `1y`) each item also compares the selected window with the immediately preceding window of the same duration:

- `previous_observation_count`
- `previous_reachable_percent`
- `reachable_trend_points`

`reachable_trend_points` is the percentage-point change, not a relative percent change.

For `all`, no preceding window exists, so previous/trend percentage fields are omitted.

## Attribution and correctness

M4.14 uses the same conservative endpoint attribution rule as M4.13. Rollup endpoint tuples `(host, port, tls)` are attributed to a network only when the curated registry maps that tuple to exactly one network. Ambiguous tuples are excluded from ranking calculations.

Networks without observations in the selected current window are omitted rather than ranked as zero-availability networks. This avoids treating missing coverage as downtime.

## Storage

No schema migration is required. M4.14 reads the M4.11 hourly/daily rollups and existing registry tables.

- `24h`, `7d`: hourly rollups
- `30d`, `1y`, `all`: daily rollups

SQLite remains a valid Core runtime backend, but the historical ranking endpoint is unavailable there. PostgreSQL remains the production history backend.

## Qualification

Tests cover:

- deterministic ranking between two networks;
- current availability percentages;
- positive and negative percentage-point trends against the previous equal-length window;
- exclusion of ambiguous endpoint tuples;
- request period and limit validation.

The `Historical Stats` workflow starts real Core against PostgreSQL 17, seeds two curated networks plus current and previous-window observations over HTTP, then verifies the ranking and trend response over the public API.
