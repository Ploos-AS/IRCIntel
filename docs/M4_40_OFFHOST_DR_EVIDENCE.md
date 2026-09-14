# M4.40 — Off-host DR evidence

M4.39 keeps scheduled DR acceptance evidence on local operations storage. M4.40 adds an S3-compatible archive helper so those records can be copied to a separate durable boundary.

## Helper

`ops/postgres/dr-evidence-s3.sh EVIDENCE_JSON`

The helper accepts only `dr-acceptance-*.json` records with schema `ircintel.dr-acceptance-record.v1`. It verifies the record, calculates SHA-256 and byte size, uploads immutable object metadata, performs a remote metadata check, downloads the object again, and verifies the SHA-256 round trip.

Configuration uses `IRCINTEL_DR_EVIDENCE_S3_BUCKET`, optional `IRCINTEL_DR_EVIDENCE_S3_PREFIX` (default `postgres/dr-evidence`) and optional `IRCINTEL_DR_EVIDENCE_S3_ENDPOINT`. The bucket falls back to `IRCINTEL_BASEBACKUP_S3_BUCKET`, and the endpoint can fall back to the existing backup endpoint settings.

Retries with identical content are idempotent. Reusing an existing object key with different content returns exit 73. Invalid records return exit 65.

## CI qualification

`.github/workflows/dr-evidence-s3.yml` uses LocalStack 4.4.0 to prove upload plus download verification, idempotent retry, immutable-key collision rejection, and rejection of invalid evidence records.

The CI provider is only a deterministic S3-compatible test boundary; it is not a production off-host provider.

## Production

Production deployments should archive each M4.39 evidence record to protected off-host storage with appropriate encryption, access policy, retention and independent durability. An archive failure is an operational DR failure even when the acceptance result itself is successful.
