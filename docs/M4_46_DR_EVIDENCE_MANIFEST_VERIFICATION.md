# M4.46 — DR Evidence Manifest Verification

M4.46 makes the M4.45 audit manifest independently verifiable against the evidence payload and the M4.43 restore-verification record.

## Contract

`ops/postgres/dr-evidence-manifest-verify.sh MANIFEST_JSON EVIDENCE_JSON RESTORE_JSON`

The verifier fails closed unless all three records form one consistent chain:

- manifest schema is `ircintel.dr-evidence-manifest.v1` and result is `ok`;
- evidence schema is `ircintel.dr-acceptance-record.v1`;
- restore schema is `ircintel.dr-evidence-restore.v1`, result is `ok`, and archive metadata was verified;
- archive key is identical in manifest and restore record;
- SHA-256 is identical across manifest, live evidence bytes, and restore record;
- byte size is identical across all three sources;
- mode and acceptance result agree across evidence, restore metadata, and manifest;
- manifest records both metadata and restore verification as true.

Missing source files exit 66, missing required commands exit 69, and any integrity/schema mismatch exits 74.

## Qualification

`.github/workflows/dr-evidence-manifest-verify.yml` creates a valid M4.45 chain and verifies it, then proves fail-closed behavior for a tampered manifest hash, changed evidence bytes, and a mismatched restore key.

M4.46 is PASS only after the GitHub Actions workflow succeeds on the exact implementation HEAD.
