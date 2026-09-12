# M4.28 — Scheduled PITR recovery drills

M4.28 closes the restore-drill gap left explicitly open by M4.27.

A backup and WAL archive are only useful if recovery continues to work. IRCIntel therefore treats a successful point-in-time restore as a recurring operational qualification rather than a one-time development milestone.

## Operational contract

`.github/workflows/pitr-restore.yml` now supports four trigger modes:

- normal `push` qualification;
- `pull_request` qualification;
- manual `workflow_dispatch` recovery drills; and
- a weekly scheduled recovery drill at `03:17 UTC` every Sunday.

The scheduled job uses the same PostgreSQL 17 recovery path as M4.25. It creates a real base backup and WAL archive, chooses a recovery target between two writes, restores into a fresh PostgreSQL instance, and proves that the target boundary is respected.

The job is bounded by a 15-minute timeout and does not cancel an in-progress drill when another run for the same ref is queued.

## PASS contract

A drill passes only when:

1. PostgreSQL 17 creates the source database and base backup;
2. WAL needed for the target is archived;
3. a fresh recovery instance becomes ready;
4. rows written before the target are present;
5. rows written after the target are absent; and
6. PostgreSQL has promoted out of recovery.

The workflow writes a concise result to the GitHub Actions job summary including trigger, PostgreSQL major version, recovered rows, target timestamp and `PASS` result.

Any failed assertion fails the workflow. Source and recovery container logs are emitted on failure.

## Relationship to previous milestones

- M4.24 verifies PITR configuration and WAL archive readiness.
- M4.25 proves a successful point-in-time restore.
- M4.26 proves missing WAL fails safely.
- M4.27 continuously exposes WAL archive health.
- M4.28 makes successful recovery a recurring, unattended qualification.

This milestone schedules the repository-level recovery qualification. Production deployments should additionally run restore drills against their real off-host backup/archive destination and route scheduled workflow failures into their normal alerting path.
