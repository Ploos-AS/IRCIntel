# M4.47 — DR Evidence Production Readiness Gate

M4.47 integrates the M4.44–M4.46 evidence controls into the production DR readiness contract.

Production readiness now requires the evidence-retention, manifest, and manifest-verification implementation files and documentation. It resolves the evidence bucket from `IRCINTEL_DR_EVIDENCE_S3_BUCKET`, falling back to the base-backup bucket, and validates `IRCINTEL_DR_EVIDENCE_RETENTION_DAYS` (default 365).

The gate fails closed when evidence retention is malformed or shorter than base-backup retention. A successful production verdict emits `dr_evidence_bucket`, `dr_evidence_retention_days`, and `dr_evidence_retention_configured=true` in addition to the existing DR readiness fields.

`.github/workflows/dr-readiness.yml` qualifies CI contract completeness, missing production configuration, unsafe endpoints, WAL/base retention, evidence/base retention, malformed evidence retention, and a representative production configuration.

M4.47 is PASS only after `DR Readiness` succeeds on GitHub Actions for the exact milestone HEAD.
