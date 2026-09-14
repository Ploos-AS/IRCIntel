# M4.42 — DR evidence integrity chain

M4.42 turns the previously qualified DR stages into one fail-closed chain:

`scheduled acceptance -> evidence record -> off-host archive -> restore -> SHA-256 verification`

The chain is implemented by `ops/postgres/dr-integrity-chain.sh` and qualified by `.github/workflows/dr-integrity-chain.yml`.

## Contract

A successful scheduled run must:

1. complete DR acceptance;
2. create `ircintel.dr-acceptance-record.v1` evidence;
3. archive that exact evidence object to the configured off-host S3 boundary;
4. restore the object;
5. verify SHA-256 equality and JSON identity;
6. report `dr_integrity_chain_result=ok`.

Production scheduled runs now require the evidence archive to be configured. A production run without an archive bucket fails closed instead of reporting success with local-only evidence.

The archive metadata contains the source SHA-256 and byte size and is checked on upload. The restore step verifies the downloaded bytes against the source digest and compares the parsed JSON records exactly.

## Tamper model

The qualification workflow modifies a local copy of the evidence after archival and proves that its digest differs from the restored off-host object. The test therefore demonstrates that a changed local record cannot be mistaken for the archived record.

## Security

No credentials, webhook URLs, raw alert payloads, or provider secrets are written into the evidence record. Production endpoints and encryption policies remain governed by the existing M4.35–M4.41 readiness contracts.
