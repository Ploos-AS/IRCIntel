#!/usr/bin/env bash
set -euo pipefail
usage() { echo "usage: $0 BUCKET [apply|check]" >&2; exit 64; }
[[ $# -ge 1 && $# -le 2 ]] || usage
bucket=$1; mode=${2:-check}
[[ "$mode" == "apply" || "$mode" == "check" ]] || usage
prefix=${IRCINTEL_DR_AUDIT_BUNDLE_S3_PREFIX:-postgres/dr-audit-bundles}
days=${IRCINTEL_DR_AUDIT_BUNDLE_RETENTION_DAYS:-365}
evidence_days=${IRCINTEL_DR_EVIDENCE_RETENTION_DAYS:-365}
base_days=${IRCINTEL_BASEBACKUP_RETENTION_DAYS:-35}
endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
for value in "$days" "$evidence_days" "$base_days"; do
  [[ "$value" =~ ^[1-9][0-9]*$ ]] || { echo "dr_audit_bundle_retention_result=invalid_days" >&2; exit 64; }
done
# Audit bundles are the terminal chain-of-custody record: never expire before
# either the evidence they bind or the base-backup window being attested to.
if (( days < evidence_days || days < base_days )); then
  echo "dr_audit_bundle_retention_result=unsafe_window" >&2
  echo "dr_audit_bundle_retention_days=$days" >&2
  echo "dr_evidence_retention_days=$evidence_days" >&2
  echo "basebackup_retention_days=$base_days" >&2
  exit 65
fi
for cmd in aws python3 mktemp; do command -v "$cmd" >/dev/null 2>&1 || { echo "dr_audit_bundle_retention_missing_command=$cmd" >&2; exit 69; }; done
aws_args=(s3api); [[ -z "$endpoint" ]] || aws_args+=(--endpoint-url "$endpoint")
policy=$(mktemp); trap 'rm -f "$policy"' EXIT
python3 - "$prefix" "$days" >"$policy" <<'PY'
import json,sys
prefix,days=sys.argv[1:]
print(json.dumps({'Rules':[{'ID':'ircintel-dr-audit-bundle-retention','Status':'Enabled','Filter':{'Prefix':prefix.rstrip('/')+'/'},'Expiration':{'Days':int(days)}}]},separators=(',',':')))
PY
if [[ "$mode" == "apply" ]]; then
  current=$(aws "${aws_args[@]}" get-bucket-lifecycle-configuration --bucket "$bucket" --output json 2>/dev/null || printf '{"Rules":[]}')
  merged=$(mktemp); trap 'rm -f "$policy" "$merged"' EXIT
  python3 - "$current" "$policy" >"$merged" <<'PY'
import json,sys
current=json.loads(sys.argv[1]); wanted=json.load(open(sys.argv[2],encoding='utf-8'))['Rules'][0]
rules=[r for r in current.get('Rules',[]) if r.get('ID')!=wanted['ID']]; rules.append(wanted)
print(json.dumps({'Rules':rules},separators=(',',':')))
PY
  aws "${aws_args[@]}" put-bucket-lifecycle-configuration --bucket "$bucket" --lifecycle-configuration "file://$merged" >/dev/null
fi
actual=$(aws "${aws_args[@]}" get-bucket-lifecycle-configuration --bucket "$bucket" --output json)
python3 - "$actual" "$prefix" "$days" <<'PY'
import json,sys
obj=json.loads(sys.argv[1]); prefix=sys.argv[2].rstrip('/')+'/'; days=int(sys.argv[3])
m=[r for r in obj.get('Rules',[]) if r.get('ID')=='ircintel-dr-audit-bundle-retention']
if len(m)!=1: print('dr_audit_bundle_retention_result=mismatch'); raise SystemExit(74)
r=m[0]
if r.get('Status')!='Enabled' or r.get('Filter',{}).get('Prefix')!=prefix or r.get('Expiration',{}).get('Days')!=days:
    print('dr_audit_bundle_retention_result=mismatch'); raise SystemExit(74)
print('dr_audit_bundle_retention_result=ok')
print(f'dr_audit_bundle_retention_days={days}')
print(f'dr_audit_bundle_retention_prefix={prefix}')
PY
