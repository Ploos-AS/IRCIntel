# M4.30 — Off-host base-backup transfer

M4.30 closes the next PostgreSQL PITR durability gap after M4.29 off-host WAL archiving: physical base backups can now be transferred to S3-compatible storage with an explicit integrity and immutability contract.

## Scope

`ops/postgres/basebackup-s3.sh` transfers one already-created physical PostgreSQL base-backup bundle to object storage.

The existing PITR qualification remains responsible for proving that IRCIntel can create and restore PostgreSQL 17 physical base backups. M4.30 adds the missing off-host transfer boundary rather than replacing that recovery test.

## Configuration

Required:

- `IRCINTEL_BASEBACKUP_S3_BUCKET`

Optional:

- `IRCINTEL_BASEBACKUP_S3_PREFIX` — defaults to `postgres/basebackup`
- `IRCINTEL_BASEBACKUP_S3_ENDPOINT` — for S3-compatible endpoints; omit for AWS S3
- standard AWS CLI credentials and region variables

Usage:

```sh
ops/postgres/basebackup-s3.sh BACKUP_PATH BACKUP_NAME
```

`BACKUP_NAME` is intentionally a single object basename. Paths and `..` components are rejected so callers cannot escape the configured prefix.

## Integrity and fail-closed contract

For each object the uploader records metadata:

- `sha256`
- `size-bytes`
- `kind=postgres-base-backup`

Transfer succeeds only if the uploaded object can immediately be read back via `HeadObject` with matching SHA-256 and size metadata.

Retries are idempotent: if the key already exists with the same SHA-256 and size, the operation succeeds as `already_present` without rewriting the object.

If the key already exists with different integrity metadata, the operation fails closed with exit code 73. Existing backup objects are never overwritten by the helper.

## Qualification

`.github/workflows/basebackup-s3.yml` performs an isolated GitHub Actions qualification using PostgreSQL 17 and LocalStack S3:

1. Starts an S3-compatible endpoint.
2. Starts PostgreSQL 17 and inserts a known fixture row.
3. Creates a real `pg_basebackup` physical backup.
4. Packages the backup and verifies that `PG_VERSION` is present.
5. Uploads the bundle to the configured base-backup prefix.
6. Repeats the transfer and requires the idempotent `already_present` result.
7. Attempts to reuse the same object key for different content and requires a fail-closed collision.
8. Verifies the stored SHA-256, size and backup-kind metadata.

This CI workflow qualifies the transfer contract; it is not itself a production backup destination.

## Production expectations

Production deployments should use an off-host/object-storage destination independent of the PostgreSQL host and supply credentials outside the repository. Bucket policy, encryption, object retention/versioning and lifecycle policy are deployment concerns and must not weaken the no-overwrite integrity contract.

## Remaining durability work

After M4.30, the main remaining backup/PITR operational gaps are:

- retention and lifecycle policy for WAL and base backups,
- encryption and access-control policy,
- production alert routing for backup/archive failures,
- scheduled recovery drills against the real remote backup/archive destination.
