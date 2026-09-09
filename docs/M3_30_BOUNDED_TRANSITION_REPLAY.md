# M3.30 — Bounded transition replay

M3.30 bounds the transition-event history read used by normal endpoint incident maintenance.

## Motivation

M3.29 removed raw-observation replay from normal ingest, but every real reachability transition still read the complete `endpoint_transition_events` history for the affected endpoint. That cost would grow with the lifetime of a public IRCIntel service.

## Checkpoint rule

When an endpoint has no persisted lifecycle, Core bootstraps from the complete transition history for that endpoint.

Once at least one `incident_records` lifecycle exists, Core uses the newest lifecycle `started_at` as a checkpoint and reads transition events from:

`checkpoint - defaultCorrelationWindow`

through the newest transition.

The one-window overlap preserves clusters that cross the checkpoint boundary while excluding older clusters that cannot affect the newest lifecycle.

## Properties

- no schema or HTTP/JSON changes;
- steady-state observations still perform no incident refresh;
- a real transition no longer causes lifetime-sized transition replay after the endpoint has incident history;
- the exact correlation boundary is inclusive;
- first-incident bootstrap remains canonical and unbounded for correctness;
- all writes remain inside the M3.21 atomic ingest transaction.

## Qualification

Regression coverage verifies that transition events older than the checkpoint overlap are excluded and that an event exactly on the correlation boundary remains included.

## Remaining work

M3.30 is bounded replay, not a fully incremental correlation-state machine. A future production-hardening milestone may persist open correlation/lifecycle state directly, particularly if profiling under multi-year workloads shows the bounded window to be material.
