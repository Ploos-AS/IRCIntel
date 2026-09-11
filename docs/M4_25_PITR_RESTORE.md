# M4.25 — PostgreSQL point-in-time recovery restore qualification

M4.25 closes the gap deliberately left by M4.24: readiness is now exercised as a real recovery drill.

## Qualification contract

The dedicated `PITR Restore` workflow uses PostgreSQL 17 and:

1. starts a source server with WAL archiving enabled;
2. creates a qualification table and baseline row;
3. takes a physical `pg_basebackup` while WAL archiving is active;
4. writes a row that must survive recovery;
5. records an explicit recovery target timestamp;
6. writes a later row that must not survive recovery;
7. forces WAL rotation and verifies archived WAL exists;
8. stops the source server;
9. starts a fresh PostgreSQL 17 server from the base backup with `restore_command`, `recovery.signal`, and `recovery_target_time`;
10. verifies recovery promotes successfully;
11. verifies the baseline and pre-target rows exist while the post-target row does not.

This is an end-to-end PITR correctness qualification, not merely a configuration check.

## Production boundary

The CI archive is local and disposable. Production still needs a durable off-host/object-storage WAL archive, retention policy, monitoring, encryption/access controls, and periodic restore drills using the production backup path. M4.25 proves PostgreSQL recovery semantics and the operational sequence used by IRCIntel; it does not claim that a particular offsite provider has been deployed.

## Relationship to earlier milestones

- M4.21 qualifies logical backup/restore operations.
- M4.23 qualifies rollup and backfill-state survival through logical DR.
- M4.24 qualifies WAL/PITR configuration readiness and real WAL archival.
- M4.25 qualifies an actual base-backup + archived-WAL point-in-time restore.
