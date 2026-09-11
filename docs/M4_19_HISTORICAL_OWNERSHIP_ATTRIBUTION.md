# M4.19 — Historical ownership attribution

M4.19 removes the remaining current-registry dependency from PostgreSQL-backed network history.

## Behavior

`GET /api/v1/history/networks` and `GET /api/v1/history/networks/ranking` now attribute rollup buckets through the durable `endpoint_network_ownership` journal introduced in M4.18.

A registry move therefore does not rewrite older network history. A bucket is attributed only when one network owns the endpoint tuple for the complete bucket interval. If ownership changes inside an hourly or daily bucket, that boundary bucket is excluded rather than guessed or double-counted.

The first known ownership state for an endpoint is treated as a baseline beginning at PostgreSQL `-infinity`. Later ownership changes retain their exact transition timestamp. This preserves imported/pre-existing observation history while keeping subsequent moves time-bounded.

## Ambiguity policy

If the same `(host, port, tls)` tuple has overlapping ownership intervals for multiple networks during the same complete bucket, that bucket is excluded from network-level history. This preserves the conservative no-misattribution policy from M4.13/M4.14 while making ownership time-aware.

## Qualification

Regression coverage verifies:

- observations before a network move remain attributed to the old network;
- observations after a move belong to the new network;
- ranking uses the same historical ownership semantics;
- a bucket containing an ownership transition is excluded from both networks;
- legacy/current historical-statistics behavior remains compatible with the baseline interval.

SQLite behavior is unchanged; historical rollup APIs remain PostgreSQL-only.
