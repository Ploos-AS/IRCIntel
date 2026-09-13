# M4.36 — DR provider smoke qualification

M4.36 validates the S3-compatible disaster-recovery destination itself, after the repository-level production-readiness gate introduced in M4.35.

The smoke test is intentionally provider-facing. It does not merely check that configuration variables are present: it performs a real object roundtrip against the configured bucket and verifies that the storage provider applies the expected production controls.

## Command

```sh
bash ops/postgres/dr-provider-smoke.sh BUCKET
```

The script uses `IRCINTEL_BACKUP_S3_ENDPOINT` when configured and otherwise falls back to the WAL/base-backup endpoint variables. The expected encryption mode comes from `IRCINTEL_BACKUP_S3_ENCRYPTION` and defaults to `AES256`.

## What is verified

A successful smoke test requires all of the following:

1. the bucket exposes an explicit default server-side encryption configuration matching `AES256` or `aws:kms`;
2. the M4.31 lifecycle/retention contract can be read back exactly;
3. a random 32 KiB object can be uploaded without an explicit SSE request header;
4. `HEAD` shows that provider-side default encryption was actually applied;
5. integrity metadata survives the provider roundtrip;
6. the downloaded object has the same SHA-256 digest as the source;
7. deletion succeeds and the object is no longer addressable.

Success ends with:

```text
dr_provider_smoke_result=ok
provider_default_encryption=AES256
provider_lifecycle_verified=yes
provider_roundtrip_verified=yes
provider_delete_verified=yes
```

Failures are fail-closed. Provider/configuration mismatches use exit status 74; invalid invocation/configuration uses exit status 64.

## CI qualification

`.github/workflows/dr-provider-smoke.yml` qualifies the contract against LocalStack 4.4.0. It configures explicit default AES256 encryption and the M4.31 lifecycle policy, performs the full roundtrip, verifies object encryption and integrity, verifies deletion, and proves that an encryption mismatch fails closed.

The LocalStack workflow qualifies the implementation of the provider contract. It does **not** prove that a production provider, IAM credentials, network path or KMS key are available.

## Production use

After `ops/postgres/dr-readiness.sh production` passes, run the provider smoke against the real off-host DR bucket from an isolated deployment/recovery environment with the real production credentials.

M4.36 should be treated as a deployment acceptance check before relying on that bucket for PostgreSQL WAL/base-backup recovery.
