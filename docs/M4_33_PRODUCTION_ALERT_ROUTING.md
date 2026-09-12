# M4.33 — Production alert routing

M4.33 adds an explicit, testable routing boundary for disaster-recovery failures. The goal is that WAL archiving, off-host backup transfer, retention/security checks, and restore drills have a common path to an external incident/alert receiver.

## Routing helper

`ops/postgres/backup-alert.sh EVENT SEVERITY MESSAGE`

Required configuration:

- `IRCINTEL_BACKUP_ALERT_WEBHOOK_URL`

Optional configuration:

- `IRCINTEL_BACKUP_ALERT_SOURCE` (default `ircintel-postgres`)
- `IRCINTEL_ENVIRONMENT` (default `production`)
- `IRCINTEL_BACKUP_ALERT_TIMEOUT_SECONDS` (default `10`)

Accepted severities are `warning` and `critical`.

The helper emits a stable JSON schema:

- `schema=ircintel.backup-alert.v1`
- event
- severity
- message
- source
- environment
- UTC timestamp

Delivery is fail-closed. Missing routing configuration, invalid event/severity data, connection failures, timeouts, and non-successful delivery all return non-zero status.

## Event naming

Use stable machine-readable event names. Recommended production events include:

- `wal.archive.failed`
- `wal.archive.stale`
- `backup.upload.failed`
- `backup.retention.failed`
- `backup.security.failed`
- `restore.drill.failed`
- `restore.remote.failed`

The alert receiver is responsible for escalation policy, deduplication, paging, chat/email fan-out, and on-call schedules.

## Qualification

`.github/workflows/backup-alert-routing.yml` starts a local HTTP receiver and proves:

1. a critical WAL failure event is delivered;
2. the JSON contract contains the expected source, environment, severity, message, event, schema, and timestamp;
3. invalid severities fail closed;
4. missing webhook configuration fails closed;
5. an unreachable receiver fails closed rather than reporting success.

No production webhook or credentials are stored in the repository.

## Production integration

Call `backup-alert.sh` from the operational wrapper or scheduler whenever a backup/archive/restore health command fails. A production deployment may point the webhook at an Alertmanager-compatible bridge, incident-management gateway, or another authenticated internal webhook service.

Authentication should normally be implemented at the internal gateway/reverse-proxy boundary so provider-specific secrets do not leak into individual backup scripts. If direct provider authentication is required later, add it as a separately qualified extension rather than hard-coding a vendor-specific token format here.

## Remaining disaster-recovery gap

After M4.33 the principal remaining M4 disaster-recovery gap is qualification of scheduled recovery drills against the actual remote backup destination, rather than only local CI storage/emulation.
