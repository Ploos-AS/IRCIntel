# M4.35 — DR production readiness gate

M4.35 adds a single fail-closed preflight for the PostgreSQL disaster-recovery chain.

The goal is to make the distinction explicit between:

- **CI-qualified DR building blocks** — repository contracts and qualification workflows exist and have been exercised in CI.
- **Production-ready DR configuration** — deployment-specific off-host storage, alerting and retention/encryption inputs are present and internally consistent.

## Command

```sh
bash ops/postgres/dr-readiness.sh ci
bash ops/postgres/dr-readiness.sh production
```

## CI mode

`ci` mode checks that the complete DR contract introduced in M4.29–M4.34 is present in the repository. It requires the WAL uploader, base-backup uploader, retention policy, backup security, alert routing, remote PITR fetch/drill tooling and the corresponding milestone documents.

A successful result ends with:

```text
dr_readiness_mode=ci
dr_contract_complete=true
dr_ready=true
```

This does **not** claim that a real deployment is production-ready.

## Production mode

`production` mode fails closed unless deployment inputs satisfy the minimum production contract:

- base-backup S3 bucket configured;
- WAL S3 bucket configured;
- non-local S3-compatible endpoint configured;
- alert webhook configured and using HTTPS;
- backup encryption mode is `AES256` or `aws:kms`;
- `aws:kms` mode includes a KMS key id;
- WAL and base-backup retention are numeric;
- WAL retention is at least as long as base-backup retention.

Default retention remains 42 days for WAL and 35 days for base backups.

The preflight deliberately rejects LocalStack/localhost-style endpoints in production mode. LocalStack remains valid for CI qualification only.

A successful production preflight ends with:

```text
dr_readiness_mode=production
dr_ready=true
```

## Qualification

`.github/workflows/dr-readiness.yml` qualifies both positive and negative behavior:

1. repository DR contract passes in CI mode;
2. production mode fails when deployment inputs are missing;
3. local-only object-store endpoints are rejected;
4. an invalid retention relationship is rejected;
5. KMS mode without a key is rejected;
6. a representative production configuration passes.

## Scope boundary

Passing M4.35 means the repository has a coherent, fail-closed production readiness contract. It does not prove availability of the chosen production object store, credentials, IAM policy, KMS key, webhook receiver or isolated recovery host. Those remain deployment-specific and must be validated in the real production environment.
