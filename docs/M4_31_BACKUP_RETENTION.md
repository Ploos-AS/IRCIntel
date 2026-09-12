# M4.31 — Backup retention and lifecycle policy

M4.31 adds an explicit lifecycle boundary for the off-host PostgreSQL recovery material introduced by M4.29 and M4.30.

## Contract

`ops/postgres/s3-retention-policy.sh BUCKET [apply|check]` manages two S3 lifecycle rules:

- WAL objects under `postgres/wal/` expire after 42 days by default.
- Base-backup objects under `postgres/basebackup/` expire after 35 days by default.

The values and prefixes are configurable with:

- `IRCINTEL_WAL_RETENTION_DAYS`
- `IRCINTEL_BASEBACKUP_RETENTION_DAYS`
- `IRCINTEL_WAL_S3_PREFIX`
- `IRCINTEL_BASEBACKUP_S3_PREFIX`
- `IRCINTEL_BACKUP_S3_ENDPOINT`

The helper uses the normal AWS CLI credential and region environment variables.

## PITR safety invariant

The helper fails closed when WAL retention is shorter than base-backup retention. This is deliberately conservative: a retained base backup must not outlive the WAL retention horizon required to advance recovery from it.

This invariant does not replace operational restore testing. Production retention must be chosen from the required recovery-point objective, backup cadence, legal/operational retention needs, and actual remote restore drills.

## Qualification

`.github/workflows/backup-retention.yml` uses an S3-compatible qualification service and proves that:

1. the lifecycle configuration can be applied;
2. both expected rules can be read back and verified;
3. a subsequent check is idempotent;
4. an unsafe WAL/base-backup retention relationship is rejected.

## Production boundary

M4.31 establishes lifecycle semantics, but the object-storage provider remains responsible for actually enforcing expiration. Production deployments should additionally use provider-side versioning/object-lock policy where appropriate and monitor lifecycle behavior.

Remaining disaster-recovery hardening after M4.31 includes:

- encryption and access-control policy;
- production alert routing;
- scheduled recovery drills against the real remote backup destination.
