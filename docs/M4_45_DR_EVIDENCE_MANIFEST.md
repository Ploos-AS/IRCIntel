# M4.45 — DR Evidence Audit Manifest

## Goal

Create a compact, machine-readable chain-of-custody record that binds one DR acceptance evidence object to its off-host archive key and a successful M4.43 restore/integrity verification.

## Contract

`ops/postgres/dr-evidence-manifest.sh EVIDENCE_JSON ARCHIVE_KEY RESTORE_JSON OUTPUT_JSON`

The generated `ircintel.dr-evidence-manifest.v1` record contains:

- creation timestamp;
- immutable evidence object key;
- SHA-256 and byte size of the evidence;
- acceptance mode and result;
- explicit metadata-verification verdict;
- explicit restore-verification verdict;
- final result.

The command fails closed when the source or restore record is missing, malformed, refers to another object key, has a hash/size mismatch, disagrees on mode/result, or has not passed archive metadata verification.

Exit conventions follow the DR tooling: usage 64, missing source 66, missing command 69, integrity/semantic mismatch 74.

## Qualification

`.github/workflows/dr-evidence-manifest.yml` creates a known-good M4.38/M4.43-style evidence + restore pair and verifies a valid manifest. Negative qualification covers hash mismatch, archive-key mismatch, and an unverified restore. The resulting manifest is retained as a GitHub Actions artifact.

M4.45 is PASS only after `DR Evidence Manifest` succeeds on GitHub Actions for the milestone HEAD.
