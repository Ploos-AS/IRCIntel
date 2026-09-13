#!/usr/bin/env bash
set -euo pipefail

mode="${1:-ci}"
case "$mode" in
  ci|production) ;;
  *)
    echo "usage: $0 [ci|production]" >&2
    exit 64
    ;;
esac

required_files=(
  ops/postgres/pitr-preflight.sh
  ops/postgres/wal-archive-health.sh
  ops/postgres/wal-archive-s3.sh
  ops/postgres/basebackup-s3.sh
  ops/postgres/s3-retention-policy.sh
  ops/postgres/s3-backup-security.sh
  ops/postgres/backup-alert.sh
  ops/postgres/pitr-fetch-s3.sh
  ops/postgres/remote-pitr-drill.sh
  docs/M4_29_OFFHOST_WAL_ARCHIVE.md
  docs/M4_30_OFFHOST_BASEBACKUP.md
  docs/M4_31_BACKUP_RETENTION.md
  docs/M4_32_BACKUP_SECURITY.md
  docs/M4_33_PRODUCTION_ALERT_ROUTING.md
  docs/M4_34_REMOTE_PITR_DRILL.md
)

missing=0
for path in "${required_files[@]}"; do
  if [[ ! -f "$path" ]]; then
    echo "dr_readiness_missing=$path" >&2
    missing=1
  fi
done
if (( missing )); then
  echo "dr_ready=false"
  exit 66
fi

for path in ops/postgres/*.sh; do
  if [[ ! -x "$path" ]]; then
    echo "dr_readiness_not_executable=$path" >&2
    echo "dr_ready=false"
    exit 67
  fi
done

if [[ "$mode" == "ci" ]]; then
  echo "dr_readiness_mode=ci"
  echo "dr_contract_complete=true"
  echo "dr_ready=true"
  exit 0
fi

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "dr_readiness_missing_env=$name" >&2
    return 1
  fi
}

failed=0
for name in \
  IRCINTEL_BASEBACKUP_S3_BUCKET \
  IRCINTEL_WAL_S3_BUCKET \
  IRCINTEL_BACKUP_ALERT_WEBHOOK; do
  require_env "$name" || failed=1
done

endpoint="${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-${IRCINTEL_WAL_S3_ENDPOINT:-}}}"
if [[ -z "$endpoint" ]]; then
  echo "dr_readiness_missing_env=IRCINTEL_BACKUP_S3_ENDPOINT" >&2
  failed=1
else
  case "$endpoint" in
    http://127.0.0.1*|http://localhost*|http://localstack*|http://minio*)
      echo "dr_readiness_unsafe_endpoint=$endpoint" >&2
      failed=1
      ;;
  esac
fi

webhook="${IRCINTEL_BACKUP_ALERT_WEBHOOK:-}"
if [[ -n "$webhook" && "$webhook" != https://* ]]; then
  echo "dr_readiness_webhook_requires_https=true" >&2
  failed=1
fi

encryption="${IRCINTEL_BACKUP_S3_ENCRYPTION:-AES256}"
case "$encryption" in
  AES256) ;;
  aws:kms)
    if [[ -z "${IRCINTEL_BACKUP_S3_KMS_KEY_ID:-}" ]]; then
      echo "dr_readiness_missing_env=IRCINTEL_BACKUP_S3_KMS_KEY_ID" >&2
      failed=1
    fi
    ;;
  *)
    echo "dr_readiness_invalid_encryption=$encryption" >&2
    failed=1
    ;;
esac

wal_days="${IRCINTEL_WAL_RETENTION_DAYS:-42}"
base_days="${IRCINTEL_BASEBACKUP_RETENTION_DAYS:-35}"
if ! [[ "$wal_days" =~ ^[0-9]+$ && "$base_days" =~ ^[0-9]+$ ]]; then
  echo "dr_readiness_invalid_retention=true" >&2
  failed=1
elif (( wal_days < base_days )); then
  echo "dr_readiness_retention_invariant=false" >&2
  failed=1
fi

if (( failed )); then
  echo "dr_readiness_mode=production"
  echo "dr_ready=false"
  exit 68
fi

echo "dr_readiness_mode=production"
echo "dr_endpoint=$endpoint"
echo "dr_encryption=$encryption"
echo "dr_wal_retention_days=$wal_days"
echo "dr_basebackup_retention_days=$base_days"
echo "dr_ready=true"
