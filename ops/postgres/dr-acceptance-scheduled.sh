#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 [ci|production] BUCKET EVIDENCE_DIR" >&2
  exit 64
}

[[ $# -eq 3 ]] || usage
mode=$1
bucket=$2
evidence_dir=$3
[[ "$mode" == "ci" || "$mode" == "production" ]] || usage

retention_days=${IRCINTEL_DR_EVIDENCE_RETENTION_DAYS:-90}
case "$retention_days" in
  ''|*[!0-9]*) echo "dr_schedule_invalid_retention=$retention_days" >&2; exit 64 ;;
esac
(( retention_days >= 1 )) || { echo "dr_schedule_invalid_retention=$retention_days" >&2; exit 64; }

command -v flock >/dev/null 2>&1 || { echo "dr_schedule_missing_command=flock" >&2; exit 69; }

mkdir -p "$evidence_dir"
lock_file="$evidence_dir/.dr-acceptance.lock"
exec 9>"$lock_file"
if ! flock -n 9; then
  echo "dr_schedule_result=already_running"
  exit 75
fi

ts=$(date -u +%Y%m%dT%H%M%SZ)
out="$evidence_dir/dr-acceptance-$ts.json"

set +e
ops/postgres/dr-acceptance-record.sh "$mode" "$bucket" "$out"
rc=$?
set -e

archive_rc=0
archive_configured=no
if [[ -n "${IRCINTEL_DR_EVIDENCE_S3_BUCKET:-${IRCINTEL_BASEBACKUP_S3_BUCKET:-}}" ]]; then
  archive_configured=yes
  set +e
  archive_out=$(ops/postgres/dr-evidence-s3.sh "$out" 2>&1)
  archive_rc=$?
  set -e
  printf '%s\n' "$archive_out"
  if (( archive_rc == 0 )); then
    echo "dr_schedule_archive_result=ok"
  else
    echo "dr_schedule_archive_result=failed" >&2
  fi
else
  echo "dr_schedule_archive_result=not_configured" >&2
  if [[ "$mode" == "production" ]]; then
    archive_rc=64
  fi
fi

find "$evidence_dir" -maxdepth 1 -type f -name 'dr-acceptance-*.json' -mtime "+$retention_days" -print -delete || {
  echo "dr_schedule_retention_result=failed" >&2
  exit 74
}

echo "dr_schedule_mode=$mode"
echo "dr_schedule_evidence=$out"
echo "dr_schedule_retention_days=$retention_days"
echo "dr_schedule_archive_configured=$archive_configured"

if (( rc != 0 )); then
  echo "dr_schedule_result=acceptance_failed"
  exit "$rc"
fi

if (( archive_rc != 0 )); then
  echo "dr_schedule_result=archive_failed"
  exit "$archive_rc"
fi

echo "dr_schedule_result=ok"
