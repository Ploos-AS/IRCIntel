# M4.49 — Off-host DR Manifest Archive

M4.49 extends the qualified M4.48 integrity chain by archiving the verified chain-of-custody manifest to the configured S3-compatible off-host evidence store.

## Contract

`ops/postgres/dr-evidence-manifest-s3.sh` accepts only a valid `ircintel.dr-evidence-manifest.v1` with successful metadata and restore verification. It stores the manifest below `postgres/dr-manifests/` by default using a content-addressed SHA-256 filename.

The object carries SHA-256, byte size, kind, mode, acceptance result and source evidence SHA-256 metadata. Existing identical objects are idempotent; a conflicting object fails with exit 73. Metadata or round-trip integrity failures exit 74.

`ops/postgres/dr-integrity-chain.sh` now archives the manifest only after M4.45 generation and M4.46 independent verification have passed. A successful chain emits `dr_integrity_chain_manifest_archived=true` and the immutable manifest object key.

## Qualification

The `DR Integrity Chain` workflow uses LocalStack 4.4.0 and proves the real M4.48 chain, off-host manifest presence, archive metadata, downloaded SHA-256 identity and idempotent re-archive behavior.

M4.49 is PASS only when this workflow completes successfully on the exact milestone HEAD.
