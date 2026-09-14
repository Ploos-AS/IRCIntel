# M4.40 — Off-host DR evidence

M4.39 keeps scheduled DR acceptance evidence on local operations storage. M4.40 adds an S3-compatible archive boundary and wires it into the scheduled acceptance path.

## Archive helper

`ops/postgres/dr-evidence-s3.sh EVIDENCE_JSON`

The helper accepts only `dr-acceptance-*.json` records with schema `ircintel.dr-acceptance-record.v1`. It verifies the record, calculates SHA-256 and byte size, uploads immutable object metadata, performs a remote metadata check, downloads the object again, and verifies the SHA-256 round trip.

Configuration uses `IRCINTEL_DR_EVIDENCE_S3_BUCKET`, optional `IRCINTEL_DR_EVIDENCE_S3_PREFIX` (default `postgres/dr-evidence`) and optional `IRCINTEL_DR_EVIDENCE_S3_ENDPOINT`. The bucket falls back to `IRCINTEL_BASEBACKUP_S3_BUCKET`, and the endpoint can fall back to the existing backup endpoint settings.

Retries with identical content are idempotent. Reusing an existing object key with different content returns exit 73. Invalid records return exit 65.

## Scheduled integration

`ops/postgres/dr-acceptance-scheduled.sh` now archives the acceptance record after every completed acceptance attempt when an evidence bucket is configured. Both successful and failed acceptance records are archived.

The original acceptance verdict is preserved: an acceptance failure remains the returned failure even after its evidence is archived. If acceptance succeeds but off-host archival fails, the scheduled operation fails with the archive error rather than reporting a false success.

The scheduler emits `dr_schedule_archive_result` and `dr_schedule_archive_configured` in addition to the M4.39 scheduling fields.

## CI qualification

`.github/workflows/dr-evidence-s3.yml` uses LocalStack 4.4.0 to prove upload plus download verification, idempotent retry, immutable-key collision rejection, and rejection of invalid evidence records.

`.github/workflows/dr-acceptance-scheduled.yml` additionally proves the integrated chain: scheduled acceptance creates evidence, archives it across the S3-compatible boundary, verifies a remote object exists, archives evidence from a deliberate acceptance failure, and preserves that failure's exit status.

The CI provider is only a deterministic S3-compatible test boundary; it is not a production off-host provider.

## Production

Production deployments should set `IRCINTEL_DR_EVIDENCE_S3_BUCKET` to protected off-host storage with appropriate encryption, access policy, retention and independent durability. The scheduled runner then treats acceptance and evidence durability as one operational chain: a successful acceptance cannot be reported as successful when its configured archive fails.
