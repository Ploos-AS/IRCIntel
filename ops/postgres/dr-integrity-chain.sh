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

mkdir -p "$evidence_dir"
log="$evidence_dir/integrity-chain.log"
: > "$log"

set +e
ops/postgres/dr-acceptance-scheduled.sh "$mode" "$bucket" "$evidence_dir" >"$log" 2>&1
rc=$?
set -e
cat "$log"

if (( rc != 0 )); then
  echo "dr_integrity_chain_result=scheduled_failed"
  exit "$rc"
fi

record=$(grep '^dr_schedule_evidence=' "$log" | tail -1 | cut -d= -f2-)
key=$(grep '^dr_evidence_archive_key=' "$log" | tail -1 | cut -d= -f2-)
[[ -n "$record" && -f "$record" ]] || { echo "dr_integrity_chain_result=record_missing" >&2; exit 66; }
[[ -n "$key" ]] || { echo "dr_integrity_chain_result=archive_key_missing" >&2; exit 66; }

endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
aws_args=()
[[ -n "$endpoint" ]] && aws_args+=(--endpoint-url "$endpoint")

sha=$(sha256sum "$record" | awk '{print $1}')
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$key" "$tmp" >/dev/null
download_sha=$(sha256sum "$tmp" | awk '{print $1}')
[[ "$download_sha" == "$sha" ]] || { echo "dr_integrity_chain_result=sha256_mismatch" >&2; exit 74; }

python3 - "$record" "$tmp" <<'PY'
import json,sys
local=json.load(open(sys.argv[1],encoding='utf-8'))
remote=json.load(open(sys.argv[2],encoding='utf-8'))
assert local == remote, 'restored evidence differs from source'
assert local['schema'] == 'ircintel.dr-acceptance-record.v1'
assert local['result'] == 'ok'
PY

echo "dr_integrity_chain_result=ok"
echo "dr_integrity_chain_record=$record"
echo "dr_integrity_chain_key=$key"
echo "dr_integrity_chain_sha256=$sha"
