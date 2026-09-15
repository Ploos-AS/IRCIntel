# M4.43 — DR Evidence Archive Metadata Verification

## Goal

Make evidence restore independently fail closed against the metadata contract written by M4.40, rather than relying on a workflow fixture or a later integrity-chain comparison.

## Contract

`ops/postgres/dr-evidence-restore-verify.sh BUCKET KEY OUTPUT` now requires the archived object to carry:

- `sha256`: exactly 64 lowercase hexadecimal characters
- `size-bytes`: decimal byte count
- `kind=ircintel-dr-acceptance-evidence`
- `mode=ci|production`
- `result=ok|failed`

The downloaded object must match the archived SHA-256 and size. Its JSON must be an `ircintel.dr-acceptance-record.v1` record whose `mode` and `result` agree with the object metadata.

The restore record uses the positional `KEY` as `source_key`; it no longer depends on `IRCINTEL_DR_EVIDENCE_KEY` for provenance.

Integrity or metadata failures exit 74. A missing object exits 66.

## Qualification

`.github/workflows/dr-evidence-restore.yml` qualifies M4.43 against LocalStack 4.4.0. It proves:

1. a correctly archived acceptance record restores successfully;
2. the restore record contains the exact source key and verified archive metadata;
3. an object without metadata fails closed;
4. tampered SHA-256 metadata fails closed;
5. metadata/payload semantic disagreement fails closed;
6. a missing object fails closed.

## Status

Implementation committed. Qualification is PASS only after the `DR Evidence Restore` GitHub Actions workflow succeeds on the M4.43 HEAD.
