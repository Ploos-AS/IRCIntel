# M4.39 — Scheduled DR acceptance

M4.39 turns the M4.38 acceptance-evidence mechanism into an unattended operational runner without weakening the production readiness contract.

## Runner

```sh
ops/postgres/dr-acceptance-scheduled.sh production BUCKET EVIDENCE_DIR
```

The runner:

- executes the real M4.38 evidence wrapper in the selected `ci` or `production` mode;
- writes timestamped acceptance records into the configured evidence directory;
- preserves the underlying acceptance exit code;
- uses a non-blocking `flock` guard so overlapping runs fail with exit 75 instead of running concurrently;
- prunes old `dr-acceptance-*.json` records according to `IRCINTEL_DR_EVIDENCE_RETENTION_DAYS`, default 90 days;
- rejects invalid retention configuration fail-closed.

Production mode remains subject to the M4.35/M4.37 production readiness checks. Local or insecure endpoints are not permitted merely because the runner is scheduled.

## CI qualification

`.github/workflows/dr-acceptance-scheduled.yml` qualifies the operational runner in `ci` mode against LocalStack 4.4.0 and a local alert receiver. It proves:

1. a successful unattended run produces timestamped evidence;
2. a provider-contract failure preserves the M4.37/M4.38 failure exit code;
3. evidence older than the configured retention window is removed;
4. an already-held lock produces `dr_schedule_result=already_running` and exit 75.

CI does not claim that LocalStack is a production DR destination.

## Production scheduling

The repository deliberately does not assume systemd, cron, Kubernetes, or a specific cloud scheduler. Deployments should call the runner from their platform scheduler in an isolated recovery/operations environment. The scheduler should treat any non-zero exit as an operational alert condition; M4.37 already routes readiness/provider failures through the configured backup alert webhook.

A weekly production cadence is a reasonable baseline, and a run should also be performed after material changes to the backup destination, IAM/KMS policy, lifecycle configuration, network path, or alert routing.

The evidence directory itself must be placed on durable operations storage appropriate to the deployment. M4.39 manages record age, not the durability of that filesystem.
