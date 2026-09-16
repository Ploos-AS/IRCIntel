#!/usr/bin/env bash
set -euo pipefail

usage() { echo "usage: $0 BUCKET MANIFEST_KEY OUTPUT_JSON" >&2; exit 64; }
[[ $# -eq 3 ]] || usage
bucket=$1; key=$2; output=$3
[[ -n "$bucket" && -n "$key" && -n "$output" ]] || usage
for cmd in aws python3 sha256sum stat mktemp; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "dr_manifest_restore_missing_command=$cmd" >&2; exit 69; }
done
endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
aws_args=(); [[ -n "$endpoint" ]] && aws_args+=(--endpoint-url "$endpoint")
head=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json 2>/dev/null) || { echo "dr_manifest_restore_result=object_missing" >&2; exit 66; }
tmp=$(mktemp); trap 'rm -f "$tmp"' EXIT
aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$key" "$tmp" >/dev/null 2>&1 || { echo "dr_manifest_restore_result=fetch_failed" >&2; exit 74; }
sha=$(sha256sum "$tmp"|awk '{print $1}'); size=$(stat -c '%s' "$tmp")
python3 - "$head" "$tmp" "$key" "$sha" "$size" "$output" <<'PY'
import json,sys,os
try:
    head=json.loads(sys.argv[1]); manifest=json.load(open(sys.argv[2],encoding='utf-8'))
    key,sha,size,out=sys.argv[3],sys.argv[4],int(sys.argv[5]),sys.argv[6]
    meta=head.get('Metadata',{})
    expected_size=int(meta.get('size-bytes','-1'))
except Exception:
    print('dr_manifest_restore_result=invalid_data',file=sys.stderr); raise SystemExit(74)
checks=(
    meta.get('sha256')==sha,
    expected_size==size,
    meta.get('kind')=='ircintel-dr-evidence-manifest',
    manifest.get('schema')=='ircintel.dr-evidence-manifest.v1',
    manifest.get('result')=='ok',
    manifest.get('metadata_verified') is True,
    manifest.get('restore_verified') is True,
    meta.get('mode')==manifest.get('mode'),
    meta.get('result')==manifest.get('acceptance_result'),
    meta.get('evidence-sha256')==manifest.get('evidence_sha256'),
)
if not all(checks):
    print('dr_manifest_restore_result=integrity_failed',file=sys.stderr); raise SystemExit(74)
record={'schema':'ircintel.dr-evidence-manifest-restore.v1','result':'ok','source_key':key,'manifest_sha256':sha,'manifest_size_bytes':size,'manifest_schema':manifest['schema'],'mode':manifest['mode'],'acceptance_result':manifest['acceptance_result'],'evidence_key':manifest['evidence_key'],'evidence_sha256':manifest['evidence_sha256'],'metadata_verified':True}
os.makedirs(os.path.dirname(out) or '.',exist_ok=True)
with open(out,'w',encoding='utf-8') as f: json.dump(record,f,sort_keys=True,separators=(',',':')); f.write('\n')
PY
echo "dr_manifest_restore_result=ok"
echo "dr_manifest_restore_key=$key"
echo "dr_manifest_restore_sha256=$sha"
echo "dr_manifest_restore_size_bytes=$size"
echo "dr_manifest_restore_record=$output"
