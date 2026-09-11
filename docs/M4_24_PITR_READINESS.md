# M4.24 — PostgreSQL PITR readiness

M4.24 establishes the production contract for PostgreSQL point-in-time recovery (PITR) readiness.

This milestone does **not** yet claim a complete end-to-end PITR restore drill. It qualifies the prerequisites that must exist before such a drill is meaningful: PostgreSQL 17, WAL settings compatible with recovery, enabled archiving, a non-noop archive command, and proof that a switched WAL segment reaches the configured archive destination.

## Operator preflight

Run:

```sh
IRCINTEL_DATABASE_URL='postgres://...' ops/postgres/pitr-preflight.sh
```

A successful result includes:

```text
pitr_ready=true
postgres_major=17
wal_level=replica
archive_mode=on
```

The script fails closed when:

- PostgreSQL is not major version 17.
- `wal_level` is not `replica` or `logical`.
- `archive_mode` is not `on` or `always`.
- `archive_command` is empty, disabled, `true`, or `/bin/true`.

The script cannot prove that the configured archive destination is durable or off-site merely by inspecting server settings. Production operators must use an archive command/tool that places WAL outside the primary PostgreSQL host and must monitor archival failures and lag.

## Qualification

`.github/workflows/pitr-readiness.yml` starts a real PostgreSQL 17 server configured with WAL archiving, runs the preflight, writes qualification data, forces a WAL switch, and verifies that at least one WAL file reaches the archive directory. It also starts a default PostgreSQL 17 instance and verifies that the preflight rejects the unsafe/default non-archiving configuration.

## Production policy

For a long-running public IRCIntel deployment:

1. Keep the M4.21 logical backup path as an independent recovery mechanism.
2. Archive WAL continuously to storage outside the database host.
3. Protect the archive with retention/versioning appropriate to the required recovery window.
4. Alert on PostgreSQL archiver failures and on unexpectedly old last-successful archive time.
5. Regularly perform an actual restore drill from a base backup plus archived WAL.

The actual base-backup + WAL replay + recovery-target qualification is intentionally left for the next PITR milestone rather than being implied by this readiness check.
