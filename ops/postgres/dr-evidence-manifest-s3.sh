#!/usr/bin/env bash
set -euo pipefail

usage() { echo "usage: $0 MANIFEST_JSON" >&2; exit 64; }
[[ $# -eq 1 ]] || usage
manifest=$1
[[ -f "$manifest" ]] || { echo "dr_manifest_archive_result=source_missing" >&2; exit 66; }

for cmd in aws python3 sha256sum stat mktemp; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "dr_manifest_archive_missing_command=$cmd" >&2; exit 69; }
done

bucket=${IRCINTEL_DR_EVIDENCE_S3_BUCKET:-${IRCINTEL_BASEBACKUP_S3_BUCKET:-}}
[[ -n "$bucket" ]] || { echo "dr_manifest_archive_result=bucket_missing" >&2; exit 64; }
prefix=${IRCINTEL_DR_MANIFEST_S3_PREFIX:-postgres/dr-manifests}
endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
aws_args=(); [[ -n "$endpoint" ]] && aws_args+=(--endpoint-url "$endpoint")

meta=$(python3 - "$manifest" <<'PY'
import json,sys
try: d=json.load(open(sys.argv[1],encoding='utf-8'))
except Exception: raise SystemExit(65)
if d.get('schema')!='ircintel.dr-evidence-manifest.v1' or d.get('result')!='ok': raise SystemExit(65)
if d.get('metadata_verified') is not True or d.get('restore_verified') is not True: raise SystemExit(65)
key=d.get('evidence_key'); mode=d.get('mode'); result=d.get('acceptance_result'); esha=d.get('evidence_sha256')
if not isinstance(key,str) or not key or mode not in {'ci','production'} or result not in {'ok','failed'}: raise SystemExit(65)
if not isinstance(esha,str) or len(esha)!=64 or any(c not in '0123456789abcdef' for c in esha): raise SystemExit(65)
print('\t'.join((key,mode,result,esha)))
PY
) || { echo "dr_manifest_archive_result=invalid_manifest" >&2; exit 65; }
IFS=$'\t' read -r evidence_key mode acceptance_result evidence_sha <<<"$meta"
sha=$(sha256sum "$manifest"|awk '{print $1}'); size=$(stat -c '%s' "$manifest")
name="dr-evidence-manifest-${sha}.json"; key="${prefix%/}/$name"

if head=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json 2>/dev/null); then
  existing=$(printf '%s' "$head"|python3 -c 'import json,sys;m=json.load(sys.stdin).get("Metadata",{});print(m.get("sha256","")+"\t"+m.get("size-bytes",""))')
  IFS=$'\t' read -r es ez <<<"$existing"
  if [[ "$es" == "$sha" && "$ez" == "$size" ]]; then echo "dr_manifest_archive_result=already_present"; echo "dr_manifest_archive_key=$key"; echo "dr_manifest_archive_sha256=$sha"; exit 0; fi
  echo "dr_manifest_archive_result=collision" >&2; exit 73
fi

aws "${aws_args[@]}" s3api put-object --bucket "$bucket" --key "$key" --body "$manifest" \
 --metadata "sha256=$sha,size-bytes=$size,kind=ircintel-dr-evidence-manifest,mode=$mode,result=$acceptance_result,evidence-sha256=$evidence_sha" >/dev/null
head=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json)
verify=$(printf '%s' "$head"|python3 -c 'import json,sys;m=json.load(sys.stdin).get("Metadata",{});print("\t".join(m.get(k,"") for k in ("sha256","size-bytes","kind","mode","result","evidence-sha256")))')
IFS=$'\t' read -r vs vz vk vm vr ve <<<"$verify"
[[ "$vs" == "$sha" && "$vz" == "$size" && "$vk" == "ircintel-dr-evidence-manifest" && "$vm" == "$mode" && "$vr" == "$acceptance_result" && "$ve" == "$evidence_sha" ]] || { echo "dr_manifest_archive_result=metadata_verification_failed" >&2; exit 74; }
tmp=$(mktemp); trap 'rm -f "$tmp"' EXIT
aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$key" "$tmp" >/dev/null
[[ "$(sha256sum "$tmp"|awk '{print $1}')" == "$sha" ]] || { echo "dr_manifest_archive_result=roundtrip_verification_failed" >&2; exit 74; }
echo "dr_manifest_archive_result=uploaded"
echo "dr_manifest_archive_key=$key"
echo "dr_manifest_archive_sha256=$sha"
echo "dr_manifest_archive_size_bytes=$size"
echo "dr_manifest_archive_evidence_key=$evidence_key"
