# M4.32 — Backup encryption and access control

M4.32 closes the encryption/access-control policy gap for off-host WAL and physical base backups introduced in M4.29–M4.31.

## Contract

`ops/postgres/s3-backup-security.sh BUCKET [apply|check]` applies or verifies a fail-closed S3 bucket security baseline:

- S3 Block Public Access: all four controls enabled.
- Default server-side encryption: AES-256 (`AES256`) by default.
- Optional production KMS mode with `IRCINTEL_BACKUP_S3_ENCRYPTION=aws:kms` and `IRCINTEL_BACKUP_S3_KMS_KEY_ID`.
- Bucket policy denies requests over insecure transport.
- Bucket policy denies object uploads that do not request server-side encryption.
- `check` verifies the effective bucket configuration and fails if the contract is not present.

The script uses `IRCINTEL_BACKUP_S3_ENDPOINT`, with the WAL/base-backup endpoint variables as compatibility fallbacks. Standard AWS CLI credentials and region configuration remain external to the repository.

## Qualification

`.github/workflows/backup-security.yml` uses a pinned LocalStack community image and validates:

1. policy application to an S3-compatible bucket;
2. public-access blocking;
3. AES-256 default encryption;
4. secure-transport and encrypted-upload deny statements;
5. idempotent `check` after application;
6. fail-closed rejection of unsupported encryption modes and KMS mode without a key ID.

## Production boundary

For production, prefer a dedicated backup principal with only the minimum bucket/object permissions required by the archive/backup lifecycle. Do not reuse application credentials. KMS deployments should use a dedicated key and a key policy restricted to backup and recovery principals.

M4.32 establishes configuration and qualification semantics; credential issuance/rotation and provider-side IAM/KMS administration remain deployment responsibilities.

## Remaining disaster-recovery gaps

After M4.32 the main remaining operational gaps are:

- production alert routing for backup/archive/restore failures;
- scheduled recovery drills against the real remote backup destination;
- deployment-specific IAM/KMS credential rotation and audit review.
