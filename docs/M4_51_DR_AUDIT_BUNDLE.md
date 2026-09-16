# M4.51 — DR Audit Bundle

M4.51 turns the independently verified DR evidence chain into one compact audit record. The bundle cryptographically binds the acceptance record, restored off-host evidence, chain-of-custody manifest and independently restored off-host manifest.

`ops/postgres/dr-audit-bundle.sh ACCEPTANCE_RECORD EVIDENCE_RESTORE MANIFEST MANIFEST_RESTORE OUTPUT_JSON` validates all four records and their cross-bindings before emitting `ircintel.dr-audit-bundle.v1`.

The bundle records immutable evidence and manifest object keys, SHA-256 digests for every input record, mode/result, and `chain_verified=true`. It does not duplicate raw evidence or manifest contents.

Missing inputs exit 66, missing commands 69, and malformed data or any integrity/binding mismatch exits 74.

`dr-integrity-chain.sh` now creates the audit bundle only after evidence restore verification, manifest verification, off-host manifest archive and independent manifest restore verification all succeed.

The `DR Integrity Chain` workflow qualifies the complete chain on LocalStack 4.4.0 and includes a negative test that tampers the manifest-restore evidence digest; bundle creation must reject it with exit 74.

M4.51 is PASS only when `DR Integrity Chain` succeeds on the exact milestone HEAD.
