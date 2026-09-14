#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 EVIDENCE_JSON" >&2
  exit 64
}

[[ $# -eq 1 ]] || usage
evidence=$1
[[ -f "$evidence" ]] || { echo "dr_evidence_archive_result=source_missing" >&2; exit 66; }

for cmd in aws python3 sha256sum stat mktemp; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "dr_evidence_archive_missing_command=$cmd" >&2; exit 69; }
done

bucket=${IRCINTEL_DR_EVIDENCE_S3_BUCKET:-${IRCINTEL_BASEBACKUP_S3_BUCKET:-}}
[[ -n "$bucket" ]] || { echo "dr_evidence_archive_result=bucket_missing" >&2; exit 64; }
prefix=${IRCINTEL_DR_EVIDENCE_S3_PREFIX:-postgres/dr-evidence}
endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}

aws_args=()
[[ -n "$endpoint" ]] && aws_args+=(--endpoint-url "$endpoint")

name=$(basename "$evidence")
case "$name" in
  dr-acceptance-*.json) ;;
  *) echo "dr_evidence_archive_result=invalid_name" >&2; exit 64 ;;
esac

json_meta=$(python3 - "$evidence" <<'PY'
import json,sys
p=sys.argv[1]
try:
    d=json.load(open(p, encoding='utf-8'))
except Exception:
    print('invalid_json')
    raise SystemExit(65)
if d.get('schema') != 'ircintel.dr-acceptance-record.v1':
    print('invalid_schema')
    raise SystemExit(65)
mode=d.get('mode')
result=d.get('result')
if mode not in {'ci','production'} or result not in {'ok','failed'}:
    print('invalid_fields')
    raise SystemExit(65)
print(f"{mode}\t{result}")
PY
) || { echo "dr_evidence_archive_result=invalid_record" >&2; exit 65; }
IFS=$'\t' read -r mode result <<<"$json_meta"

sha256=$(sha256sum "$evidence" | awk '{print $1}')
size_bytes=$(stat -c '%s' "$evidence")
key="${prefix%/}/$name"

if head_json=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json 2>/dev/null); then
  existing=$(printf '%s' "$head_json" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=d.get("Metadata",{}); print(m.get("sha256","")+"\t"+m.get("size-bytes",""))')
  IFS=$'\t' read -r existing_sha existing_size <<<"$existing"
  if [[ "$existing_sha" == "$sha256" && "$existing_size" == "$size_bytes" ]]; then
    echo "dr_evidence_archive_result=already_present"
    echo "dr_evidence_archive_key=$key"
    echo "dr_evidence_archive_sha256=$sha256"
    exit 0
  fi
  echo "dr_evidence_archive_result=collision" >&2
  echo "dr_evidence_archive_key=$key" >&2
  exit 73
fi

aws "${aws_args[@]}" s3api put-object \
  --bucket "$bucket" \
  --key "$key" \
  --body "$evidence" \
  --metadata "sha256=$sha256,size-bytes=$size_bytes,kind=ircintel-dr-acceptance-evidence,mode=$mode,result=$result" \
  >/dev/null

verify_json=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json)
verify=$(printf '%s' "$verify_json" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=d.get("Metadata",{}); print(m.get("sha256","")+"\t"+m.get("size-bytes","")+"\t"+m.get("kind","")+"\t"+m.get("mode","")+"\t"+m.get("result",""))')
IFS=$'\t' read -r verify_sha verify_size verify_kind verify_mode verify_result <<<"$verify"
if [[ "$verify_sha" != "$sha256" || "$verify_size" != "$size_bytes" || "$verify_kind" != "ircintel-dr-acceptance-evidence" || "$verify_mode" != "$mode" || "$verify_result" != "$result" ]]; then
  echo "dr_evidence_archive_result=metadata_verification_failed" >&2
  exit 74
fi

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$key" "$tmp" >/dev/null
download_sha=$(sha256sum "$tmp" | awk '{print $1}')
[[ "$download_sha" == "$sha256" ]] || { echo "dr_evidence_archive_result=roundtrip_verification_failed" >&2; exit 74; }

echo "dr_evidence_archive_result=uploaded"
echo "dr_evidence_archive_key=$key"
echo "dr_evidence_archive_sha256=$sha256"
echo "dr_evidence_archive_size_bytes=$size_bytes"
echo "dr_evidence_archive_mode=$mode"
echo "dr_evidence_archive_acceptance_result=$result"
