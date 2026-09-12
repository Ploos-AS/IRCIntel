#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 BUCKET [apply|check]" >&2
  exit 64
}

[[ $# -ge 1 && $# -le 2 ]] || usage
bucket=$1
mode=${2:-check}
[[ "$mode" == "apply" || "$mode" == "check" ]] || usage

wal_prefix=${IRCINTEL_WAL_S3_PREFIX:-postgres/wal}
base_prefix=${IRCINTEL_BASEBACKUP_S3_PREFIX:-postgres/basebackup}
wal_days=${IRCINTEL_WAL_RETENTION_DAYS:-42}
base_days=${IRCINTEL_BASEBACKUP_RETENTION_DAYS:-35}
endpoint=${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_WAL_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}

for value in "$wal_days" "$base_days"; do
  [[ "$value" =~ ^[1-9][0-9]*$ ]] || {
    echo "retention_policy_result=invalid_days" >&2
    exit 64
  }
done

# Conservative PITR invariant: WAL must survive at least as long as base backups.
if (( wal_days < base_days )); then
  echo "retention_policy_result=unsafe_overlap" >&2
  echo "wal_retention_days=$wal_days" >&2
  echo "basebackup_retention_days=$base_days" >&2
  exit 65
fi

aws_args=(s3api)
[[ -z "$endpoint" ]] || aws_args+=(--endpoint-url "$endpoint")

policy=$(mktemp)
trap 'rm -f "$policy"' EXIT
python3 - "$wal_prefix" "$base_prefix" "$wal_days" "$base_days" >"$policy" <<'PY'
import json, sys
wal_prefix, base_prefix, wal_days, base_days = sys.argv[1:]
print(json.dumps({"Rules": [
    {"ID": "ircintel-wal-retention", "Status": "Enabled", "Filter": {"Prefix": wal_prefix.rstrip("/") + "/"}, "Expiration": {"Days": int(wal_days)}},
    {"ID": "ircintel-basebackup-retention", "Status": "Enabled", "Filter": {"Prefix": base_prefix.rstrip("/") + "/"}, "Expiration": {"Days": int(base_days)}}
]}, separators=(",", ":")))
PY

if [[ "$mode" == "apply" ]]; then
  aws "${aws_args[@]}" put-bucket-lifecycle-configuration \
    --bucket "$bucket" \
    --lifecycle-configuration "file://$policy"
fi

actual=$(aws "${aws_args[@]}" get-bucket-lifecycle-configuration --bucket "$bucket" --output json)
python3 - "$actual" "$wal_prefix" "$base_prefix" "$wal_days" "$base_days" <<'PY'
import json, sys
obj=json.loads(sys.argv[1])
expected={
 "ircintel-wal-retention": (sys.argv[2].rstrip("/")+"/", int(sys.argv[4])),
 "ircintel-basebackup-retention": (sys.argv[3].rstrip("/")+"/", int(sys.argv[5])),
}
seen={}
for r in obj.get("Rules", []):
    if r.get("ID") in expected:
        seen[r["ID"]]=(r.get("Filter",{}).get("Prefix"), r.get("Expiration",{}).get("Days"), r.get("Status"))
for rid,(prefix,days) in expected.items():
    if seen.get(rid)!=(prefix,days,"Enabled"):
        print("retention_policy_result=mismatch")
        raise SystemExit(74)
print("retention_policy_result=ok")
print(f"wal_retention_days={expected['ircintel-wal-retention'][1]}")
print(f"basebackup_retention_days={expected['ircintel-basebackup-retention'][1]}")
PY
