#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 BACKUP_PATH BACKUP_NAME" >&2
  exit 64
fi

backup_path="$1"
backup_name="$2"

if [ ! -f "$backup_path" ]; then
  echo "basebackup_transfer_result=source_missing" >&2
  exit 66
fi

case "$backup_name" in
  ''|*/*|*..*)
    echo "basebackup_transfer_result=invalid_name" >&2
    exit 64
    ;;
esac

bucket="${IRCINTEL_BASEBACKUP_S3_BUCKET:?IRCINTEL_BASEBACKUP_S3_BUCKET is required}"
prefix="${IRCINTEL_BASEBACKUP_S3_PREFIX:-postgres/basebackup}"
endpoint="${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}"

aws_args=()
if [ -n "$endpoint" ]; then
  aws_args+=(--endpoint-url "$endpoint")
fi

sha256="$(sha256sum "$backup_path" | awk '{print $1}')"
size_bytes="$(stat -c '%s' "$backup_path")"
key="${prefix%/}/$backup_name"

head_json=""
if head_json="$(aws "${aws_args[@]}" s3api head-object \
  --bucket "$bucket" \
  --key "$key" \
  --output json 2>/dev/null)"; then
  existing_sha="$(printf '%s' "$head_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("Metadata", {}).get("sha256", ""))')"
  existing_size="$(printf '%s' "$head_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("Metadata", {}).get("size-bytes", ""))')"
  if [ "$existing_sha" = "$sha256" ] && [ "$existing_size" = "$size_bytes" ]; then
    echo "basebackup_transfer_result=already_present"
    echo "basebackup_key=$key"
    echo "basebackup_sha256=$sha256"
    echo "basebackup_size_bytes=$size_bytes"
    exit 0
  fi

  echo "basebackup_transfer_result=collision" >&2
  echo "basebackup_key=$key" >&2
  exit 73
fi

aws "${aws_args[@]}" s3api put-object \
  --bucket "$bucket" \
  --key "$key" \
  --body "$backup_path" \
  --metadata "sha256=$sha256,size-bytes=$size_bytes,kind=postgres-base-backup" \
  >/dev/null

verify_json="$(aws "${aws_args[@]}" s3api head-object \
  --bucket "$bucket" \
  --key "$key" \
  --output json)"
verify_sha="$(printf '%s' "$verify_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("Metadata", {}).get("sha256", ""))')"
verify_size="$(printf '%s' "$verify_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("Metadata", {}).get("size-bytes", ""))')"

if [ "$verify_sha" != "$sha256" ] || [ "$verify_size" != "$size_bytes" ]; then
  echo "basebackup_transfer_result=verification_failed" >&2
  exit 74
fi

echo "basebackup_transfer_result=uploaded"
echo "basebackup_key=$key"
echo "basebackup_sha256=$sha256"
echo "basebackup_size_bytes=$size_bytes"
