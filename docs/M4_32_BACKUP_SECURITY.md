# M4.32 — Backup encryption and access control

M4.32 closes the encryption/access-control policy gap for off-host WAL and physical base backups introduced in M4.29–M4.31.

## Contract

`ops/postgres/s3-backup-security.sh BUCKET [apply|check]` applies or verifies a fail-closed S3 bucket security baseline:

- S3 Block Public Access: all four controls enabled.
- Default server-side encryption: AES-256 (`AES256`) by default.
- Optional production KMS mode with `IRCINTEL_BACKUP_S3_ENCRYPTION=aws:kms` and `IRCINTEL_BACKUP_S3_KMS_KEY_ID`.
- S3 Bucket Keys are enabled only for KMS mode.
- Bucket policy denies requests over insecure transport.
- Uploaders do not need to send an explicit SSE request header: bucket default encryption is the at-rest encryption guarantee.
- `check` verifies the effective bucket configuration and fails if the contract is not present.

The script uses `IRCINTEL_BACKUP_S3_ENDPOINT`, with the WAL/base-backup endpoint variables as compatibility fallbacks. Standard AWS CLI credentials and region configuration remain external to the repository.

This contract is intentionally compatible with `wal-archive-s3.sh` and `basebackup-s3.sh`. Requiring an explicit `x-amz-server-side-encryption` header at the bucket-policy layer would reject those uploaders even though S3 default encryption encrypts their objects at rest.

## Qualification

`.github/workflows/backup-security.yml` uses pinned LocalStack 4.4.0 and validates:

1. policy application to an S3-compatible bucket;
2. public-access blocking;
3. AES-256 default encryption;
4. secure-transport deny policy;
5. idempotent `check` after application;
6. fail-closed rejection of unsupported encryption modes and KMS mode without a key ID;
7. real uploads through both `wal-archive-s3.sh` and `basebackup-s3.sh` without explicit SSE flags;
8. `head-object` confirms both uploaded objects report `ServerSideEncryption=AES256`.

The uploader-compatibility qualification is the regression test for the earlier M4.32 integration gap.

## Production boundary

For production, prefer a dedicated backup principal with only the minimum bucket/object permissions required by the archive/backup lifecycle. Do not reuse application credentials. KMS deployments should use a dedicated key and a key policy restricted to backup and recovery principals.

Transport security remains mandatory at the real provider boundary. The CI test uses a local S3-compatible endpoint for deterministic qualification; production endpoints must use TLS and provider IAM/KMS controls.

M4.32 establishes configuration and qualification semantics; credential issuance/rotation and provider-side IAM/KMS administration remain deployment responsibilities.

## Remaining disaster-recovery gaps

M4.33 and M4.34 add production alert-routing semantics and scheduled remote PITR drill qualification. After the M4.32 hardening, the remaining DR work is primarily deployment-specific:

- real off-host provider credentials and least-privilege IAM;
- KMS key administration if KMS mode is selected;
- production alert destination configuration;
- scheduled recovery drills against the independent production backup destination.
