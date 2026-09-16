# M4.52 — Off-host DR Audit Bundle Archive

M4.52 extends the M4.51 audit bundle with immutable off-host archival.

## Contract

`ops/postgres/dr-audit-bundle-s3.sh` accepts only a verified `ircintel.dr-audit-bundle.v1` document with `result=ok` and `chain_verified=true`.

The object key is content-addressed by SHA-256 under `postgres/dr-audit-bundles/` by default. The archive stores and verifies metadata for bundle SHA-256, size, kind, mode, acceptance result, evidence SHA-256 and manifest SHA-256.

An identical existing object is idempotent and returns `already_present`. A conflicting object at the same content-addressed key returns 73. Invalid bundles return 65; missing sources return 66; missing commands return 69; metadata or round-trip integrity failures return 74.

## Integrity-chain integration

`dr-integrity-chain.sh` archives the M4.51 audit bundle only after acceptance evidence restore, manifest verification, manifest archive/restore verification and audit-bundle verification have succeeded. Successful output includes `dr_integrity_chain_audit_bundle_archived=true` and the immutable archive key.

## Qualification

The `DR Integrity Chain` workflow uses LocalStack 4.4.0 and verifies:

- the full M4.52 chain;
- the archived object's metadata and downloaded SHA-256;
- archive idempotency;
- rejection of a bundle whose `chain_verified` flag was tampered;
- qualification evidence upload.

M4.52 is PASS only when `DR Integrity Chain` succeeds on the exact milestone HEAD.
