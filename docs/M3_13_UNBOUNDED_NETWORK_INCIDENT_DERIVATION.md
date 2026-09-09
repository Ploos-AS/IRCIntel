# M3.13 — Unbounded canonical network incident derivation

M3.13 removes the historical 500 endpoint-incident ceiling from the canonical persisted network-incident refresh path.

## Problem

`refreshNetworkIncidentRecords()` previously called `ListIncidentRecords(... Limit: 500)`. Once more than 500 endpoint lifecycle records existed, older records were invisible to network-level derivation. This could silently omit historical network incidents and make long-running public statistics incomplete.

## Change

The SQLite store now has an internal `listAllIncidentRecords()` derivation path that reads the complete persisted endpoint-incident history in deterministic newest-first order. `refreshNetworkIncidentRecords()` uses this internal path.

The public endpoint-incident query API remains bounded. The unbounded read is deliberately private to canonical derivation and does not expose an unbounded HTTP response.

A regression test inserts 501 independent endpoint lifecycle records and verifies that all 501 produce persisted network incident records.

## Remaining scalability work

Correctness is preferred over silent truncation, but a full history scan on each refresh is not the final long-running architecture. A later milestone should make incident maintenance incremental/event-driven and transactional with observation ingestion, avoiding O(N) refresh work while preserving complete history.
