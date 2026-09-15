# M4.44 — DR Evidence Retention

## Goal

Give archived DR acceptance evidence an explicit, independently verified retention policy so audit evidence cannot silently inherit the shorter WAL/base-backup lifecycle.

## Contract

`ops/postgres/dr-evidence-retention.sh BUCKET [apply|check]` manages exactly one lifecycle rule:

- rule ID: `ircintel-dr-evidence-retention`
- prefix: `IRCINTEL_DR_EVIDENCE_S3_PREFIX` (default `postgres/dr-evidence`)
- retention: `IRCINTEL_DR_EVIDENCE_RETENTION_DAYS` (default 365 days)
- evidence retention must be at least `IRCINTEL_BASEBACKUP_RETENTION_DAYS` (default 35 days)
- unrelated lifecycle rules are preserved when applying the evidence rule
- malformed policy exits 64
- unsafe evidence/base-backup window exits 65
- missing required command exits 69
- provider policy mismatch exits 74

This milestone intentionally separates evidence retention from M4.31 backup retention. DR evidence is an audit artifact and normally needs a substantially longer lifetime than the recoverable backup window.

## Qualification

`.github/workflows/dr-evidence-retention.yml` uses LocalStack 4.4.0 and proves:

1. the 365-day evidence rule can be applied and checked;
2. the configured evidence prefix is exact;
3. an unrelated lifecycle rule survives the update;
4. evidence retention shorter than the base-backup window fails closed with exit 65;
5. malformed retention fails closed with exit 64.

M4.44 is PASS only after `DR Evidence Retention` succeeds on GitHub Actions for the milestone HEAD.
