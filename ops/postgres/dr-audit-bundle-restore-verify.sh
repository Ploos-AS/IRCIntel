#!/usr/bin/env bash
set -euo pipefail
usage() { echo "usage: $0 BUCKET AUDIT_BUNDLE_KEY OUTPUT_JSON" >&2; exit 64; }
[[ $# -eq 3 ]] || usage
bucket=$1; key=$2; output=$3
for cmd in aws python3 sha256sum stat mktemp; do command -v "$cmd" >/dev/null 2>&1 || { echo "dr_audit_bundle_restore_missing_command=$cmd" >&2; exit 69; }; done
endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
aws_args=(); [[ -n "$endpoint" ]] && aws_args+=(--endpoint-url "$endpoint")
set +e
head=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json 2>/dev/null); rc=$?
set -e
(( rc == 0 )) || { echo "dr_audit_bundle_restore_result=object_missing" >&2; exit 66; }
tmp=$(mktemp); trap 'rm -f "$tmp"' EXIT
aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$key" "$tmp" >/dev/null || { echo "dr_audit_bundle_restore_result=download_failed" >&2; exit 74; }
sha=$(sha256sum "$tmp"|awk '{print $1}'); size=$(stat -c '%s' "$tmp")
mkdir -p "$(dirname "$output")"
set +e
python3 - "$tmp" "$head" "$key" "$sha" "$size" "$output" <<'PY'
import json,sys
src,head_raw,key,sha,size,out=sys.argv[1:]
try:
    d=json.load(open(src,encoding='utf-8')); h=json.loads(head_raw); m=h.get('Metadata',{})
except Exception: raise SystemExit(74)
def h64(v): return isinstance(v,str) and len(v)==64 and all(c in '0123456789abcdef' for c in v)
if d.get('schema')!='ircintel.dr-audit-bundle.v1' or d.get('result')!='ok' or d.get('chain_verified') is not True: raise SystemExit(74)
mode=d.get('mode'); result=d.get('acceptance_result'); evidence=d.get('evidence_sha256'); manifest=d.get('manifest_sha256')
if mode not in ('ci','production') or result not in ('ok','failed') or not h64(evidence) or not h64(manifest): raise SystemExit(74)
expected={'sha256':sha,'size-bytes':size,'kind':'ircintel-dr-audit-bundle','mode':mode,'result':result,'evidence-sha256':evidence,'manifest-sha256':manifest}
if any(m.get(k)!=v for k,v in expected.items()): raise SystemExit(74)
record={'schema':'ircintel.dr-audit-bundle-restore.v1','result':'ok','source_key':key,'audit_bundle_sha256':sha,'audit_bundle_size_bytes':int(size),'mode':mode,'acceptance_result':result,'evidence_sha256':evidence,'manifest_sha256':manifest,'metadata_verified':True,'bundle_verified':True}
with open(out,'w',encoding='utf-8') as f: json.dump(record,f,sort_keys=True,separators=(',',':')); f.write('\n')
PY
rc=$?
set -e
(( rc == 0 )) || { rm -f "$output"; echo "dr_audit_bundle_restore_result=integrity_mismatch" >&2; exit 74; }
echo "dr_audit_bundle_restore_result=ok"
echo "dr_audit_bundle_restore=$output"
echo "dr_audit_bundle_restore_sha256=$(sha256sum "$output"|awk '{print $1}')"
echo "dr_audit_bundle_restore_verified=true"
