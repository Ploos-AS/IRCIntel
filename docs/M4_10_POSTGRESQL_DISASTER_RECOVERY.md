# M4.10 — PostgreSQL backup/restore and disaster-recovery qualification

M4.10 qualifies logical PostgreSQL disaster recovery for IRCIntel's production storage path.

## Scope

CI now runs a dedicated `postgres-disaster-recovery` job against PostgreSQL 17.

The job:

1. Builds the real `ircintel` Core binary.
2. Starts Core with `IRCINTEL_STORAGE_BACKEND=postgres` against a source database.
3. Seeds curated registry data through the public administrative HTTP API.
4. Seeds six observations through the ingest HTTP API, including a two-agent down/recovery lifecycle.
5. Confirms a persisted closed endpoint incident exists before backup.
6. Creates a custom-format logical backup with PostgreSQL 17 `pg_dump` using `--no-owner --no-acl`.
7. Creates a completely new empty PostgreSQL database.
8. Restores the backup with PostgreSQL 17 `pg_restore --exit-on-error`.
9. Stops the source Core process and starts Core against the restored database.
10. Verifies through the HTTP API that the restored database contains:
   - the curated network and endpoint registry,
   - all six observations,
   - the closed endpoint incident lifecycle,
   - network incident statistics for the restored network.

## Recovery contract

The qualified backup artifact is a PostgreSQL custom-format logical dump produced by a PostgreSQL client matching the production major version. Restores are performed into a fresh database, not over a running production database.

Application startup after restore remains migration-aware: Core opens the restored database through the normal PostgreSQL runtime path and refuses unsupported newer schema versions.

## Production guidance

This CI qualification proves logical dump/restore correctness for the current schema and application data path. A real deployment still needs an operational backup policy covering at least:

- scheduled encrypted backups,
- off-host/off-site copies,
- retention rotation,
- periodic restore drills,
- monitoring of backup age and failures,
- documented RPO/RTO targets,
- database credentials stored outside repository configuration,
- PostgreSQL version-aware upgrade procedures.

For multi-year public operation, logical backups should not be the only protection once data volume grows substantially. PostgreSQL physical/base backups and WAL/PITR should be evaluated in a later production-hardening milestone.

## Qualification gate

M4.10 is qualified when the final CI run is green for:

- `test`
- `oci-runtime`
- `postgres-runtime`
- `postgres-disaster-recovery`
