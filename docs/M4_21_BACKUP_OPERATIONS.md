# M4.21 — PostgreSQL backup operations

M4.21 turns the logical disaster-recovery mechanism qualified in M4.10 into a conservative operator-facing backup contract for long-running IRCIntel deployments.

## Operations

`ops/postgres/backup-logical.sh`

- requires `IRCINTEL_DATABASE_URL` and `IRCINTEL_BACKUP_DIR`;
- creates PostgreSQL custom-format logical dumps;
- uses `--no-owner --no-acl`;
- writes to a temporary file first and publishes the dump only after `pg_restore --list` validates it;
- creates a SHA-256 sidecar;
- uses `umask 077` so newly created artifacts are private by default.

`ops/postgres/restore-logical.sh`

- requires an explicit restore URL and the confirmation value `restore-into-empty-database`;
- validates the dump before restore;
- never drops, creates, cleans, or overwrites a database automatically;
- is intended only for a fresh empty target database.

`ops/postgres/prune-logical-backups.sh`

- defaults to 30-day local retention;
- accepts `IRCINTEL_BACKUP_RETENTION_DAYS`;
- only deletes IRCIntel dump/checksum names in the selected backup directory;
- never recursively deletes unrelated files.

## Scheduling contract

The repository deliberately does not embed credentials or choose an off-site provider. Production operators should schedule `backup-logical.sh` externally (systemd timer, cron, orchestration platform, or backup system), copy completed `.dump` and `.sha256` artifacts to encrypted off-host/off-site storage, and run pruning only after the remote copy has been verified.

A reasonable initial policy for a small public deployment is daily logical backups, 30 days of local copies, a longer encrypted off-site retention policy, and a periodic restore drill. Exact RPO/RTO and retention remain deployment decisions and must be documented before production launch.

Do not place `IRCINTEL_DATABASE_URL` or storage credentials in the repository. Supply them through the deployment secret mechanism.

## PITR boundary

M4.21 operationalizes logical backups; it does not claim point-in-time recovery. For a multi-year public service, physical/base backups plus WAL archiving/PITR remain the next database-protection layer once the production topology and backup destination are selected.

## Qualification

The dedicated `Backup Operations` workflow uses PostgreSQL 17 and verifies that:

1. a source database can be backed up with the repository script;
2. the checksum validates and the dump catalogue is readable;
3. the dump restores into a newly created empty database;
4. restored data matches the source fixture;
5. retention removes only matching expired backup artifacts and preserves unrelated/current files.
