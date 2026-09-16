# M4.50 — DR Manifest Restore Verification

M4.50 closes the off-host manifest loop introduced by M4.49: the archived chain-of-custody manifest must be independently downloadable and verifiable from object metadata plus its own contents.

`ops/postgres/dr-evidence-manifest-restore-verify.sh BUCKET MANIFEST_KEY OUTPUT_JSON` fetches the archived manifest, verifies SHA-256 and byte-size metadata, object kind, manifest schema/result, mode, acceptance result and evidence SHA binding. Missing objects exit 66, missing commands 69, and integrity/data mismatches 74.

The verifier emits a compact `ircintel.dr-evidence-manifest-restore.v1` record containing the immutable source key, manifest digest/size, mode/result and the bound evidence key/digest. Raw manifest contents are not copied into the verification record.

`dr-integrity-chain.sh` now requires this restore verification after M4.49 archive upload and emits `dr_integrity_chain_manifest_restore_verified=true` only on success.

The `DR Integrity Chain` workflow qualifies the real chain on LocalStack 4.4.0 and includes a negative metadata-tamper test that must fail with exit 74.

M4.50 is PASS only when `DR Integrity Chain` succeeds on the exact milestone HEAD.
