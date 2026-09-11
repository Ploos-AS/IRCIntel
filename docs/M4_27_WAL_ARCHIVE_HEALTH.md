# M4.27 — WAL archive continuity health

M4.27 turns the PITR production boundary from M4.25/M4.26 into an operationally observable contract.

A configured archive command and one successful recovery drill are not enough for a long-running public service. IRCIntel must be able to detect when PostgreSQL WAL archival stops progressing or starts failing.

## Qualification contract

`ops/postgres/wal-archive-health.sh` reads PostgreSQL 17 `pg_stat_archiver` and fails closed unless the archive is currently healthy.

The check reports machine-readable key/value fields:

- `wal_archive_healthy`
- `wal_archive_reason`
- `wal_archived_count`
- `wal_failed_count`
- `wal_last_archived_age_seconds`
- `wal_last_archived`
- `wal_last_failed`

`IRCINTEL_WAL_ARCHIVE_MAX_AGE_SECONDS` controls the maximum accepted age of the last successfully archived WAL file and defaults to 300 seconds.

The check fails when:

1. no WAL has been archived yet;
2. the latest archived WAL is older than the configured threshold; or
3. PostgreSQL reports archive failures.

## CI qualification

`.github/workflows/wal-archive-health.yml` starts PostgreSQL 17 with real WAL archiving and proves three states:

1. a forced WAL switch reaches the archive and is reported healthy;
2. the same archive is rejected when its age exceeds the configured threshold;
3. a deliberately broken `archive_command` increments `pg_stat_archiver.failed_count` and is reported unhealthy.

This is deliberately a continuity/health qualification, not an object-storage implementation. Production deployments should run the health check periodically and export/translate its values into their monitoring stack. The key/value output is intentionally suitable for a Prometheus textfile collector or another monitoring adapter without making Prometheus mandatory.

## Relationship to previous milestones

- M4.24 verifies PITR configuration readiness and real WAL archival.
- M4.25 proves a successful point-in-time recovery.
- M4.26 proves missing WAL causes an explicit, safe recovery failure.
- M4.27 makes ongoing WAL archive continuity observable and fail-closed.

Off-host/object-storage durability, retention, encryption/access control, alert routing and periodic restore drills remain deployment responsibilities.
