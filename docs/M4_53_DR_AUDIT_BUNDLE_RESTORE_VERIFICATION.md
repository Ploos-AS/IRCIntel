# M4.53 — DR Audit Bundle Restore Verification

M4.53 independently restores and verifies the immutable off-host audit bundle introduced in M4.52.

## Contract

`ops/postgres/dr-audit-bundle-restore-verify.sh BUCKET AUDIT_BUNDLE_KEY OUTPUT_JSON` downloads the archived bundle and validates both the object and the JSON payload.

It verifies object metadata for SHA-256, byte size, object kind, mode, acceptance result, evidence SHA-256 and manifest SHA-256. It also requires `ircintel.dr-audit-bundle.v1`, `result=ok`, `chain_verified=true`, valid mode/result values and valid SHA-256 bindings.

Successful verification emits `ircintel.dr-audit-bundle-restore.v1` with the source key, bundle SHA-256/size, mode/result, evidence and manifest hashes, plus `metadata_verified=true` and `bundle_verified=true`.

Missing objects return 66, missing commands return 69, and download, metadata, payload or binding integrity failures return 74.

## Integrity-chain integration

`dr-integrity-chain.sh` now restores the just-archived audit bundle and requires independent verification before the chain can report success.

## Qualification

The `DR Integrity Chain` workflow uses LocalStack 4.4.0 and verifies the full M4.53 chain, validates the restore record, and proves that tampering with archived manifest-SHA metadata is rejected with exit 74.

M4.53 is PASS only when `DR Integrity Chain` succeeds on the exact milestone HEAD.
