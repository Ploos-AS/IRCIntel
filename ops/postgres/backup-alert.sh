#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 EVENT SEVERITY MESSAGE" >&2
  exit 64
}

[[ $# -eq 3 ]] || usage
event=$1
severity=$2
message=$3

case "$severity" in
  warning|critical) ;;
  *) echo "backup_alert_result=invalid_severity" >&2; exit 64 ;;
esac

[[ "$event" =~ ^[a-z0-9][a-z0-9._-]*$ ]] || {
  echo "backup_alert_result=invalid_event" >&2
  exit 64
}

webhook=${IRCINTEL_BACKUP_ALERT_WEBHOOK_URL:-}
[[ -n "$webhook" ]] || {
  echo "backup_alert_result=not_configured" >&2
  exit 78
}

source_name=${IRCINTEL_BACKUP_ALERT_SOURCE:-ircintel-postgres}
environment=${IRCINTEL_ENVIRONMENT:-production}
timeout=${IRCINTEL_BACKUP_ALERT_TIMEOUT_SECONDS:-10}

[[ "$timeout" =~ ^[1-9][0-9]*$ ]] || {
  echo "backup_alert_result=invalid_timeout" >&2
  exit 64
}

payload=$(python3 - "$event" "$severity" "$message" "$source_name" "$environment" <<'PY'
import json, sys
from datetime import datetime, timezone
event, severity, message, source, environment = sys.argv[1:]
print(json.dumps({
    "schema":"ircintel.backup-alert.v1",
    "event":event,
    "severity":severity,
    "message":message,
    "source":source,
    "environment":environment,
    "timestamp":datetime.now(timezone.utc).isoformat().replace("+00:00","Z"),
}, separators=(",", ":")))
PY
)

response=$(mktemp)
trap 'rm -f "$response"' EXIT
http_code=$(curl --silent --show-error --fail-with-body \
  --connect-timeout "$timeout" --max-time "$timeout" \
  -o "$response" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -H 'User-Agent: IRCIntel-backup-alert/1' \
  --data-binary "$payload" \
  "$webhook") || {
    rc=$?
    echo "backup_alert_result=delivery_failed" >&2
    [[ ! -s "$response" ]] || cat "$response" >&2
    exit "$rc"
  }

case "$http_code" in
  2??)
    echo "backup_alert_result=delivered"
    echo "backup_alert_event=$event"
    echo "backup_alert_severity=$severity"
    ;;
  *)
    echo "backup_alert_result=delivery_failed" >&2
    echo "backup_alert_http_status=$http_code" >&2
    exit 75
    ;;
esac
