# M3.29 — Event-driven endpoint transitions

M3.29 moves normal incident maintenance from raw observation replay to persisted reachability transition events.

## Storage

SQLite schema version advances from v5 to v6 with `endpoint_transition_events`.

Each row identifies one reachability transition for an `(agent_id, host, port, TLS)` tuple and records:

- transition type (`down` or `recovered`),
- transition observation time,
- previous observation time.

The v5→v6 migration backfills the event stream once from historical observations so existing incident history remains reconstructable.

## Ingest behavior

For a newly accepted observation, Core now:

1. reads the previous `agent_endpoint_state`,
2. persists a transition only when the observation is newer and reachability changed,
3. refreshes endpoint and network incidents only when a transition was produced,
4. advances `agent_endpoint_state`,
5. commits all writes atomically.

A steady-state observation therefore stores telemetry and updates state but performs no incident refresh.

Out-of-order observations cannot create transitions or move persisted state backwards.

## Incident derivation

Persisted endpoint incidents now derive from `endpoint_transition_events` rather than replaying raw observation payloads. Correlation and lifecycle semantics remain unchanged.

The M3.27 observation-replay helpers remain temporarily available for regression/reconstruction purposes but are no longer used by the normal ingest path.

## Remaining work

The transition stream is still replayed per affected endpoint when a real transition occurs. A later milestone can incrementally maintain correlation/lifecycle state so even transition-history replay is bounded or eliminated.
