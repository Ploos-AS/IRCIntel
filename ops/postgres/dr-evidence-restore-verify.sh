#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 BUCKET KEY OUTPUT" >&2
  exit 64
}

[[ $# -eq 3 ]] || usage
bucket=$1
key=$2
output=$3

for cmd in aws python3 sha256sum stat mktemp; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "dr_evidence_restore_missing_command=$cmd" >&2; exit 69; }
done

mkdir -p "$(dirname "$output")"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

endpoint=${IRCINTEL_DR_EVIDENCE_S3_ENDPOINT:-${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
aws_args=()
[[ -n "$endpoint" ]] && aws_args+=(--endpoint-url "$endpoint")

head_json=$(aws "${aws_args[@]}" s3api head-object --bucket "$bucket" --key "$key" --output json) || {
  echo "dr_evidence_restore_result=object_missing" >&2
  exit 66
}

meta=$(printf '%s' "$head_json" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=d.get("Metadata",{}); print("\t".join(m.get(k,"") for k in ("sha256","size-bytes","kind","mode","result")))')
IFS=$'\t' read -r expected_sha expected_size expected_kind expected_mode expected_result <<<"$meta"

[[ "$expected_sha" =~ ^[0-9a-f]{64}$ ]] || { echo "dr_evidence_restore_result=metadata_sha256_invalid" >&2; exit 74; }
[[ "$expected_size" =~ ^[0-9]+$ ]] || { echo "dr_evidence_restore_result=metadata_size_invalid" >&2; exit 74; }
[[ "$expected_kind" == "ircintel-dr-acceptance-evidence" ]] || { echo "dr_evidence_restore_result=metadata_kind_invalid" >&2; exit 74; }
[[ "$expected_mode" == "ci" || "$expected_mode" == "production" ]] || { echo "dr_evidence_restore_result=metadata_mode_invalid" >&2; exit 74; }
[[ "$expected_result" == "ok" || "$expected_result" == "failed" ]] || { echo "dr_evidence_restore_result=metadata_result_invalid" >&2; exit 74; }

aws "${aws_args[@]}" s3api get-object --bucket "$bucket" --key "$key" "$tmp" >/dev/null
actual_sha=$(sha256sum "$tmp" | awk '{print $1}')
actual_size=$(stat -c '%s' "$tmp")
[[ "$actual_sha" == "$expected_sha" ]] || { echo "dr_evidence_restore_result=sha256_mismatch" >&2; exit 74; }
[[ "$actual_size" == "$expected_size" ]] || { echo "dr_evidence_restore_result=size_mismatch" >&2; exit 74; }

python3 - "$tmp" "$output" "$key" "$expected_sha" "$expected_size" "$expected_kind" "$expected_mode" "$expected_result" <<'PY'
import json
import sys
from pathlib import Path

source = Path(sys.argv[1])
out = Path(sys.argv[2])
key, sha256, size_bytes, kind, mode, archive_result = sys.argv[3:]
try:
    evidence = json.loads(source.read_text(encoding="utf-8"))
except Exception:
    raise SystemExit(74)
if evidence.get("schema") != "ircintel.dr-acceptance-record.v1":
    raise SystemExit(74)
if evidence.get("mode") != mode or evidence.get("result") != archive_result:
    raise SystemExit(74)
record = {
    "schema": "ircintel.dr-evidence-restore.v1",
    "source_key": key,
    "restored_size": int(size_bytes),
    "restored_sha256": sha256,
    "archive_kind": kind,
    "archive_mode": mode,
    "archive_result": archive_result,
    "metadata_verified": True,
    "result": "ok",
}
out.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
rc=$?
[[ $rc -eq 0 ]] || { echo "dr_evidence_restore_result=evidence_metadata_mismatch" >&2; exit 74; }

rm -f "$tmp"

echo "dr_evidence_restore_result=ok"
echo "dr_evidence_restore_output=$output"
echo "dr_evidence_restore_sha256=$actual_sha"
echo "dr_evidence_restore_size_bytes=$actual_size"
