# M4.34 — Remote PITR recovery drill

M4.34 closes the disaster-recovery gap left after M4.29–M4.33: a recovery drill must prove that PostgreSQL can be restored from objects fetched back from the configured off-host/S3-compatible backup boundary, rather than from the original local backup and WAL directories.

## Contract

The scheduled `Remote PITR Drill` workflow creates a PostgreSQL 17 fixture, takes a physical base backup, creates WAL before and after a recovery target, and transfers the base-backup bundle and archived WAL through the production transfer helpers.

The workflow then deletes the local base backup, local WAL archive, and local transfer bundle. `ops/postgres/pitr-fetch-s3.sh` fetches the base backup and WAL objects from S3-compatible storage and verifies their recorded SHA-256/size metadata before they may be used for recovery.

The recovered cluster must contain the base row and the row committed before the recovery target, must exclude the row committed after the target, and must finish promoted rather than remaining in recovery.

A PASS therefore proves the complete qualification path:

1. create a real PostgreSQL 17 base backup;
2. produce archived WAL around a known recovery target;
3. transfer both object classes across the S3-compatible boundary;
4. destroy the original local recovery inputs;
5. fetch and integrity-check remote objects;
6. perform point-in-time recovery using only the fetched objects;
7. verify target semantics and promotion.

## Scheduling

The workflow runs on push and pull requests for regression coverage, supports manual dispatch, and runs unattended every Sunday at 03:47 UTC. The existing local PITR drill remains useful as a narrower PostgreSQL recovery regression test.

## Production deployment

CI uses LocalStack only as a deterministic S3-compatible remote boundary. Production deployments must point the same transfer/fetch contract at the real independent off-host destination and supply credentials outside the repository. A production scheduler should run the drill against an isolated recovery environment and route failures through the M4.33 alerting path.

The fetcher is fail-closed: missing backup integrity metadata, checksum/size mismatch, missing WAL, or WAL checksum mismatch aborts recovery qualification.

## Remaining deployment work

After M4.34, the repository has executable contracts for WAL archival, base-backup transfer, retention, backup security, alert routing, local PITR qualification, and remote-boundary PITR qualification. Remaining work is primarily deployment-specific: real provider credentials/IAM/KMS configuration, choosing an independent production object store, alert destination configuration, and operating the scheduled drill in the production recovery environment.
