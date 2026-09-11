# M4.26 — PostgreSQL PITR missing-WAL failure qualification

M4.26 complements the successful point-in-time restore proven by M4.25 with a deliberate negative recovery drill.

## Qualification contract

The dedicated `PITR Failure Path` workflow uses PostgreSQL 17 and:

1. starts a source server with WAL archiving enabled;
2. creates a qualification table and baseline row;
3. takes a physical `pg_basebackup` while WAL archiving is active;
4. writes post-backup data and records a recovery target;
5. forces WAL archival and verifies archived WAL exists;
6. stops the source server;
7. deliberately removes the archived WAL;
8. starts a recovery server from the base backup with the original recovery target;
9. verifies the recovery server never becomes a healthy promoted database;
10. verifies PostgreSQL terminates recovery and emits a missing-WAL/recovery-target failure diagnostic.

The workflow itself passes only when the deliberately damaged recovery fails safely and observably.

## Why this matters

A successful restore drill proves that a complete backup chain can recover. It does not prove that an incomplete chain is detected. M4.26 ensures IRCIntel's qualification suite also catches the dangerous case where a base backup exists but the WAL archive required to reach the requested recovery point is missing.

This protects against false confidence in backup health: the presence of a base backup alone is not treated as recoverability.

## Production boundary

CI deliberately destroys its disposable local archive. Production must instead monitor archive continuity, retain WAL according to the recovery policy, alert on archive failures, and keep durable off-host copies. M4.26 qualifies PostgreSQL failure semantics and detection; it does not replace production archive monitoring.

## Relationship to earlier milestones

- M4.21 qualifies logical backup/restore operations.
- M4.24 qualifies WAL/PITR readiness and real WAL archival.
- M4.25 qualifies a successful base-backup + archived-WAL point-in-time restore.
- M4.26 qualifies safe, observable failure when the required archived WAL is unavailable.
