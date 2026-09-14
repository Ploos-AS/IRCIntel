#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 [ci|production] BUCKET OUTPUT_JSON" >&2
  exit 64
}

[[ $# -eq 3 ]] || usage
mode=$1
bucket=$2
output=$3
case "$mode" in
  ci|production) ;;
  *) usage ;;
esac

mkdir -p "$(dirname "$output")"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
set +e
acceptance_output=$(ops/postgres/dr-acceptance.sh "$mode" "$bucket" 2>&1)
rc=$?
set -e
completed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '%s\n' "$acceptance_output"

result=failed
if (( rc == 0 )); then
  result=ok
fi

ACCEPTANCE_OUTPUT="$acceptance_output" \
STARTED_AT="$started_at" \
COMPLETED_AT="$completed_at" \
MODE="$mode" \
BUCKET="$bucket" \
RESULT="$result" \
EXIT_CODE="$rc" \
python3 - "$tmp" <<'PY'
import hashlib
import json
import os
import sys

raw = os.environ["ACCEPTANCE_OUTPUT"]
record = {
    "schema": "ircintel.dr-acceptance-record.v1",
    "started_at": os.environ["STARTED_AT"],
    "completed_at": os.environ["COMPLETED_AT"],
    "mode": os.environ["MODE"],
    "bucket": os.environ["BUCKET"],
    "result": os.environ["RESULT"],
    "exit_code": int(os.environ["EXIT_CODE"]),
    "acceptance_output_sha256": hashlib.sha256((raw + "\n").encode()).hexdigest(),
}
json.dump(record, open(sys.argv[1], "w"), sort_keys=True, indent=2)
open(sys.argv[1], "a").write("\n")
PY

mv "$tmp" "$output"
trap - EXIT

echo "dr_acceptance_record=$output"
echo "dr_acceptance_record_mode=$mode"
echo "dr_acceptance_record_result=$result"
exit "$rc"
