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

mkdir -p "$(dirname "$output")"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

aws_args=()
if [[ -n "${IRCINTEL_BACKUP_S3_ENDPOINT:-}" ]]; then
  aws_args+=(--endpoint-url "$IRCINTEL_BACKUP_S3_ENDPOINT")
fi

aws s3api head-object --bucket "$bucket" --key "$key" "${aws_args[@]}" >/dev/null
aws s3api get-object --bucket "$bucket" --key "$key" "$tmp" "${aws_args[@]}" >/dev/null

python3 - "$tmp" "$output" <<'PY'
import hashlib
import json
import os
import sys
from pathlib import Path

source = Path(sys.argv[1])
out = Path(sys.argv[2])
data = source.read_bytes()
record = {
    "schema": "ircintel.dr-evidence-restore.v1",
    "source_key": os.environ.get("IRCINTEL_DR_EVIDENCE_KEY", ""),
    "restored_size": len(data),
    "restored_sha256": hashlib.sha256(data).hexdigest(),
    "result": "ok",
}
out.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

# Never retain restored evidence itself in the qualification output directory.
rm -f "$tmp"

echo "dr_evidence_restore_result=ok"
echo "dr_evidence_restore_output=$output"
