#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 [ci|production] BUCKET" >&2
  exit 64
}

[[ $# -eq 2 ]] || usage
mode=$1
bucket=$2
case "$mode" in
  ci|production) ;;
  *) usage ;;
esac

alert_failure() {
  local event=$1
  local message=$2
  if [[ -n "${IRCINTEL_BACKUP_ALERT_WEBHOOK_URL:-}" ]]; then
    ops/postgres/backup-alert.sh "$event" critical "$message" || {
      echo "dr_acceptance_alert_delivery=failed" >&2
      return 1
    }
    echo "dr_acceptance_alert_delivery=delivered"
  else
    echo "dr_acceptance_alert_delivery=not_configured"
  fi
}

set +e
readiness_output=$(ops/postgres/dr-readiness.sh "$mode" 2>&1)
readiness_rc=$?
set -e
printf '%s\n' "$readiness_output"
if (( readiness_rc != 0 )); then
  alert_failure "dr.readiness.failed" "IRCIntel DR readiness failed in ${mode} mode (exit ${readiness_rc})" || true
  echo "dr_acceptance_result=readiness_failed"
  exit "$readiness_rc"
fi

set +e
provider_output=$(ops/postgres/dr-provider-smoke.sh "$bucket" 2>&1)
provider_rc=$?
set -e
printf '%s\n' "$provider_output"
if (( provider_rc != 0 )); then
  alert_failure "dr.provider.failed" "IRCIntel DR provider smoke failed for configured backup destination (exit ${provider_rc})" || true
  echo "dr_acceptance_result=provider_failed"
  exit "$provider_rc"
fi

echo "dr_acceptance_mode=$mode"
echo "dr_acceptance_bucket=$bucket"
echo "dr_acceptance_result=ok"
