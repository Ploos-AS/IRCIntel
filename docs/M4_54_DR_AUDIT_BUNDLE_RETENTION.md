# M4.54 — DR Audit Bundle Retention

M4.54 adds an explicit lifecycle policy for the immutable off-host audit bundles introduced in M4.52 and independently restored in M4.53.

## Contract

`ops/postgres/dr-audit-bundle-retention.sh BUCKET [apply|check]` manages the lifecycle rule `ircintel-dr-audit-bundle-retention` for `IRCINTEL_DR_AUDIT_BUNDLE_S3_PREFIX` (default `postgres/dr-audit-bundles`).

The default retention is 365 days through `IRCINTEL_DR_AUDIT_BUNDLE_RETENTION_DAYS`.

Because the audit bundle is the terminal chain-of-custody record, its retention window must be at least as long as both `IRCINTEL_DR_EVIDENCE_RETENTION_DAYS` and `IRCINTEL_BASEBACKUP_RETENTION_DAYS`. An unsafe shorter window fails closed with exit 65. Invalid retention values return 64, missing commands return 69, and lifecycle-policy mismatches return 74.

`apply` preserves unrelated S3 lifecycle rules and only replaces the IRCIntel audit-bundle rule. `check` independently verifies the installed rule without changing it.

## Qualification

`DR Audit Bundle Retention` uses LocalStack 4.4.0 to verify apply/check behavior, preservation of unrelated lifecycle rules, the expected prefix and 365-day window, rejection of an audit window shorter than the evidence window, and rejection of malformed retention values.

M4.54 is PASS only when `DR Audit Bundle Retention` succeeds on the exact milestone HEAD.
