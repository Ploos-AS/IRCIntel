# M4.15 — PostgreSQL retention and rollup maintenance

IRCIntel is intended to run as a public service for years. Raw probe observations are useful for recent diagnostics, but retaining every raw row forever is not required for long-term public statistics.

## Policy foundation

M4.15 introduces an explicit PostgreSQL maintenance pass with conservative defaults:

- raw observations: 30 days
- hourly rollups: 400 days
- daily rollups: retained indefinitely

The daily rollup is therefore the durable substrate for multi-year and `all` historical statistics.

Retention is **not run automatically at Core startup**. Operators must invoke/schedule maintenance deliberately in a later operational integration. This avoids an unexpected destructive action merely because a process restarted.

## Safety contract

`RunRetentionMaintenance` executes one transaction:

1. rebuild hourly and daily rollups for the raw-data window about to be removed;
2. delete raw observations older than the raw cutoff;
3. delete hourly rollups older than the hourly cutoff;
4. commit all changes together.

If any step fails, the transaction is rolled back. Daily rollups are never pruned by this policy.

The policy rejects zero/negative retention and rejects hourly retention shorter than raw retention.

## Qualification

The PostgreSQL test seeds recent, 40-day-old and 500-day-old observations and verifies that maintenance:

- preserves the recent raw row;
- removes the two expired raw rows;
- preserves hourly history inside the 400-day window;
- removes hourly history outside that window;
- preserves all three daily historical buckets;
- is idempotent on a second pass.

M4.15 is qualified only when the normal CI gates are green on the final commit.

## Follow-up

A later milestone should provide operational scheduling, metrics and observability around maintenance (last successful run, rows pruned, duration, failures and backup age) before enabling unattended production retention.
