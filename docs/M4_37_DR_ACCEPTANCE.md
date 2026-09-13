# M4.37 — DR acceptance and alert integration

M4.37 turns the M4.35 readiness contract and M4.36 provider smoke test into one operational acceptance path, and closes an alert-routing configuration mismatch discovered between M4.35 and M4.33.

## Canonical alert setting

The canonical production setting is:

```text
IRCINTEL_BACKUP_ALERT_WEBHOOK_URL
```

This is the variable consumed by `ops/postgres/backup-alert.sh`. `ops/postgres/dr-readiness.sh production` now requires the same variable and requires an HTTPS URL. The older non-`_URL` spelling does not satisfy production readiness.

## Acceptance command

```sh
ops/postgres/dr-acceptance.sh production BUCKET
```

The wrapper executes, in order:

1. `dr-readiness.sh production`;
2. `dr-provider-smoke.sh BUCKET` against the configured remote object store;
3. a critical `dr.readiness.failed` or `dr.provider.failed` alert when the corresponding stage fails and alert routing is configured.

A successful run ends with:

```text
dr_acceptance_mode=production
dr_acceptance_bucket=BUCKET
dr_acceptance_result=ok
```

The wrapper preserves the failing readiness/provider exit code. Alert delivery is attempted without masking the original DR failure.

## CI qualification

`.github/workflows/dr-acceptance.yml` uses LocalStack 4.4.0 plus a local test alert receiver. It proves:

- the end-to-end CI acceptance path succeeds with a correctly configured provider contract;
- a deliberate provider encryption mismatch fails closed with exit status 74;
- that failure is routed through the real M4.33 `backup-alert.sh` helper;
- the emitted alert uses schema `ircintel.backup-alert.v1`, event `dr.provider.failed`, severity `critical`, and the qualification environment.

The CI workflow qualifies orchestration and alert integration. It does not claim that a real production object store, IAM identity, KMS key, network path, or external incident receiver is healthy.

## Production use

Run the acceptance command from the isolated recovery/deployment environment after credentials and endpoint configuration are installed. A production acceptance result should be recorded before relying on a new or changed DR bucket, IAM policy, KMS key, retention policy, or network path.
