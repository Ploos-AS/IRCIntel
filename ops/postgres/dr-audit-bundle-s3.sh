#!/usr/bin/env bash
set -euo pipefail
usage() { echo "usage: $0 AUDIT_BUNDLE" >&2; exit 64; }
[[ $# -eq 1 ]] || usage
source=$1
[[ -f "$source" ]] || { echo "dr_audit_bundle_archive_result=source_missing" >&2; exit 66; }
for cmd in aws python3 sha256sum stat mktemp; do command -v "$cmd" >/dev/null 2>&1 || { echo "dr_audit_bundle_archive_missing_command=$cmd" >&2; exit 69; }; done
bucket=${IRCINTEL_DR_EVIDENCE_S3_BUCKET:-${IRCINTEL_BASEBACKUP_S3_BUCKET:-}}
[[ -n "$bucket" ]] || usage
prefix=${IRCINTEL_DR_AUDIT_BUNDLE_S3_PREFIX:-postgres/dr-audit-bundles}; prefix=${prefix#/}; prefix=${prefix%/}
endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
aws_args=(); [[ -n "$endpoint" ]] && aws_args+=(--endpoint-url "$endpoint")
read -r mode result evidence_sha manifest_sha < <(python3 - "$source" <<'PY'
import json,sys
try: d=json.load(open(sys.argv[1],encoding='utf-8'))
except Exception: raise SystemExit(65)
if d.get('schema')!='ircintel.dr-audit-bundle.v1' or d.get('result')!='ok' or d.get('chain_verified') is not True: raise SystemExit(65)
mode=d.get('mode'); result=d.get('acceptance_result'); es=d.get('evidence_sha256'); ms=d.get('manifest_sha256')
if mode not in ('ci','production') or result not in ('ok','failed'): raise SystemExit(65)
if not isinstance(es,str) or len(es)!=64 or not isinstance(ms,str) or len(ms)!=64: raise SystemExit(65)
print(mode,result,es,ms)
PY
) || { echo "dr_audit_bundle_archive_result=invalid_bundle" >&2; exit 65; }
sha=$(sha256sum "$source"|awk '{print $1}'); size=$(stat -c '%s' "$source"); key="$prefix/dr-audit-bundle-$sha.json"
set +e
head=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json 2>/dev/null); hrc=$?
set -e
if (( hrc == 0 )); then
  existing=$(printf '%s' "$head"|python3 -c 'import json,sys; m=json.load(sys.stdin).get("Metadata",{}); print(m.get("sha256","")+" "+m.get("size-bytes",""))')
  [[ "$existing" == "$sha $size" ]] || { echo "dr_audit_bundle_archive_result=collision" >&2; exit 73; }
  echo "dr_audit_bundle_archive_result=already_present"; echo "dr_audit_bundle_archive_key=$key"; echo "dr_audit_bundle_archive_sha256=$sha"; echo "dr_audit_bundle_archive_size_bytes=$size"; exit 0
fi
aws "${aws_args[@]}" s3api put-object --bucket "$bucket" --key "$key" --body "$source" --metadata "sha256=$sha,size-bytes=$size,kind=ircintel-dr-audit-bundle,mode=$mode,result=$result,evidence-sha256=$evidence_sha,manifest-sha256=$manifest_sha" >/dev/null
head=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json)
printf '%s' "$head"|python3 -c 'import json,sys; m=json.load(sys.stdin).get("Metadata",{}); exp=sys.argv[1:]; keys=("sha256","size-bytes","kind","mode","result","evidence-sha256","manifest-sha256"); raise SystemExit(0 if all(m.get(k)==v for k,v in zip(keys,exp)) else 1)' "$sha" "$size" ircintel-dr-audit-bundle "$mode" "$result" "$evidence_sha" "$manifest_sha" || { echo "dr_audit_bundle_archive_result=metadata_mismatch" >&2; exit 74; }
tmp=$(mktemp); trap 'rm -f "$tmp"' EXIT
aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$key" "$tmp" >/dev/null
[[ $(sha256sum "$tmp"|awk '{print $1}') == "$sha" ]] || { echo "dr_audit_bundle_archive_result=roundtrip_mismatch" >&2; exit 74; }
echo "dr_audit_bundle_archive_result=ok"; echo "dr_audit_bundle_archive_key=$key"; echo "dr_audit_bundle_archive_sha256=$sha"; echo "dr_audit_bundle_archive_size_bytes=$size"
