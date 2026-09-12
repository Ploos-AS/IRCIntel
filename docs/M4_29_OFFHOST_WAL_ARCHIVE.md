# M4.29 — S3-compatible off-host WAL archive

M4.29 closes the first off-host durability gap left open by M4.27 and M4.28.

IRCIntel now provides a repository-owned WAL archive command for S3-compatible object storage. The command is designed to be used directly from PostgreSQL `archive_command` and to fail closed when the destination cannot prove that the correct WAL object is present.

## Archive command

`ops/postgres/wal-archive-s3.sh WAL_PATH WAL_NAME`

Required environment:

- `IRCINTEL_WAL_S3_BUCKET`

Optional environment:

- `IRCINTEL_WAL_S3_PREFIX` — defaults to `wal`;
- `IRCINTEL_WAL_S3_ENDPOINT` — enables S3-compatible endpoints such as MinIO;
- `AWS_REGION` / `AWS_DEFAULT_REGION` — defaults to `us-east-1`;
- normal AWS credential mechanisms supported by the AWS CLI.

A PostgreSQL deployment can use the script as its archive command, for example:

```text
archive_command='/path/to/wal-archive-s3.sh %p %f'
```

Secrets are deliberately not stored in the repository.

## Integrity and fail-closed contract

For every WAL segment the command:

1. validates the source path and WAL object name;
2. computes a SHA-256 digest of the local WAL segment;
3. checks whether the destination object already exists;
4. accepts an existing object only when its stored `sha256` metadata exactly matches the local segment;
5. refuses to overwrite an object whose checksum differs;
6. uploads a missing object with the SHA-256 digest in object metadata; and
7. performs a `head-object` verification after upload before returning success.

This makes retries idempotent without allowing a different WAL payload to silently replace an existing archive object.

## Qualification

`.github/workflows/wal-archive-s3.yml` runs a real S3-compatible qualification using MinIO on the GitHub runner.

The workflow proves:

- a new WAL object can be uploaded;
- the expected bucket/key is used;
- a retry with identical bytes succeeds as `already_present`;
- a retry with different bytes fails closed instead of overwriting the object; and
- the stored object retains the expected SHA-256 metadata.

No cloud credentials are required for repository CI. Production can point the same script at AWS S3 or another S3-compatible object store through normal AWS CLI configuration.

## Relationship to previous milestones

- M4.24 establishes PITR readiness and local WAL archival.
- M4.25 proves successful point-in-time recovery.
- M4.26 proves missing WAL fails safely.
- M4.27 makes WAL archive continuity observable.
- M4.28 schedules recurring restore drills.
- M4.29 adds a qualified off-host/object-storage WAL archive path.

M4.29 covers WAL durability, not yet the complete off-host backup lifecycle. Base-backup transfer, retention/lifecycle policy, encryption/access-control policy, production alert routing and recovery drills against the real remote destination remain separate operational concerns.
