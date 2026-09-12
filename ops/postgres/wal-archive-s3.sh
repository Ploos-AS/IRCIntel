#!/usr/bin/env bash
set -euo pipefail

wal_path="${1:-}"
wal_name="${2:-}"

if [[ -z "$wal_path" || -z "$wal_name" ]]; then
  echo "usage: $0 WAL_PATH WAL_NAME" >&2
  exit 64
fi

if [[ ! -f "$wal_path" ]]; then
  echo "wal_archive_result=source_missing" >&2
  exit 66
fi

if [[ "$wal_name" == */* || "$wal_name" == "." || "$wal_name" == ".." ]]; then
  echo "wal_archive_result=invalid_name" >&2
  exit 64
fi

: "${IRCINTEL_WAL_S3_BUCKET:?IRCINTEL_WAL_S3_BUCKET is required}"

prefix="${IRCINTEL_WAL_S3_PREFIX:-wal}"
region="${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"
endpoint="${IRCINTEL_WAL_S3_ENDPOINT:-}"

if [[ -n "$prefix" ]]; then
  key="${prefix%/}/${wal_name}"
else
  key="$wal_name"
fi

aws_args=(--region "$region")
if [[ -n "$endpoint" ]]; then
  aws_args+=(--endpoint-url "$endpoint")
fi

checksum="$(sha256sum "$wal_path" | awk '{print $1}')"

existing_checksum=""
if existing_checksum="$(aws "${aws_args[@]}" s3api head-object \
    --bucket "$IRCINTEL_WAL_S3_BUCKET" \
    --key "$key" \
    --query 'Metadata.sha256' \
    --output text 2>/dev/null)"; then
  if [[ "$existing_checksum" == "$checksum" ]]; then
    echo "wal_archive_result=already_present"
    echo "wal_archive_key=$key"
    echo "wal_archive_sha256=$checksum"
    exit 0
  fi

  echo "wal_archive_result=collision" >&2
  echo "wal_archive_key=$key" >&2
  exit 73
fi

aws "${aws_args[@]}" s3api put-object \
  --bucket "$IRCINTEL_WAL_S3_BUCKET" \
  --key "$key" \
  --body "$wal_path" \
  --metadata "sha256=$checksum" \
  >/dev/null

stored_checksum="$(aws "${aws_args[@]}" s3api head-object \
  --bucket "$IRCINTEL_WAL_S3_BUCKET" \
  --key "$key" \
  --query 'Metadata.sha256' \
  --output text)"

if [[ "$stored_checksum" != "$checksum" ]]; then
  echo "wal_archive_result=verification_failed" >&2
  echo "wal_archive_key=$key" >&2
  exit 74
fi

echo "wal_archive_result=uploaded"
echo "wal_archive_key=$key"
echo "wal_archive_sha256=$checksum"
