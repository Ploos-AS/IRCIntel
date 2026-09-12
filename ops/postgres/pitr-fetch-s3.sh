#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: $0 BASEBACKUP_NAME WAL_PREFIX DEST_DIR" >&2
  exit 64
fi

backup_name="$1"
wal_prefix="$2"
dest_dir="$3"
case "$backup_name" in ''|*/*|*..*) echo "pitr_fetch_result=invalid_backup_name" >&2; exit 64;; esac

bucket="${IRCINTEL_BASEBACKUP_S3_BUCKET:?IRCINTEL_BASEBACKUP_S3_BUCKET is required}"
wal_bucket="${IRCINTEL_WAL_S3_BUCKET:-$bucket}"
backup_prefix="${IRCINTEL_BASEBACKUP_S3_PREFIX:-postgres/basebackup}"
endpoint="${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-${IRCINTEL_WAL_S3_ENDPOINT:-}}}"

aws_args=()
[ -n "$endpoint" ] && aws_args+=(--endpoint-url "$endpoint")
backup_key="${backup_prefix%/}/$backup_name"
mkdir -p "$dest_dir/wal"

head_json="$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$backup_key" --output json)"
expected_sha="$(printf '%s' "$head_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("Metadata",{}).get("sha256",""))')"
expected_size="$(printf '%s' "$head_json" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("Metadata",{}).get("size-bytes",""))')"
[ -n "$expected_sha" ] && [ -n "$expected_size" ] || { echo "pitr_fetch_result=missing_backup_integrity" >&2; exit 74; }

aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$backup_key" "$dest_dir/basebackup.tar.gz" >/dev/null
actual_sha="$(sha256sum "$dest_dir/basebackup.tar.gz" | awk '{print $1}')"
actual_size="$(stat -c '%s' "$dest_dir/basebackup.tar.gz")"
[ "$actual_sha" = "$expected_sha" ] && [ "$actual_size" = "$expected_size" ] || { echo "pitr_fetch_result=backup_integrity_failed" >&2; exit 74; }

keys="$(aws "${aws_args[@]}" s3api list-objects-v2 --bucket "$wal_bucket" --prefix "${wal_prefix%/}/" --query 'Contents[].Key' --output text)"
[ -n "$keys" ] && [ "$keys" != "None" ] || { echo "pitr_fetch_result=wal_missing" >&2; exit 66; }
wal_count=0
for key in $keys; do
  name="${key##*/}"
  [ -n "$name" ] || continue
  expected="$(aws "${aws_args[@]}" s3api head-object --bucket "$wal_bucket" --key "$key" --query 'Metadata.sha256' --output text)"
  [ -n "$expected" ] && [ "$expected" != "None" ] || { echo "pitr_fetch_result=missing_wal_integrity" >&2; exit 74; }
  aws "${aws_args[@]}" s3api get-object --bucket "$wal_bucket" --key "$key" "$dest_dir/wal/$name" >/dev/null
  actual="$(sha256sum "$dest_dir/wal/$name" | awk '{print $1}')"
  [ "$actual" = "$expected" ] || { echo "pitr_fetch_result=wal_integrity_failed" >&2; exit 74; }
  wal_count=$((wal_count + 1))
done

echo "pitr_fetch_result=ok"
echo "pitr_fetch_backup_key=$backup_key"
echo "pitr_fetch_wal_count=$wal_count"
