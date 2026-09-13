#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 BUCKET" >&2
  exit 64
}

[[ $# -eq 1 ]] || usage
bucket=$1
endpoint=${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_WAL_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
expected_encryption=${IRCINTEL_BACKUP_S3_ENCRYPTION:-AES256}

if [[ "$expected_encryption" != "AES256" && "$expected_encryption" != "aws:kms" ]]; then
  echo "dr_provider_smoke_result=invalid_encryption" >&2
  exit 64
fi

for cmd in aws python3 sha256sum; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "dr_provider_smoke_result=missing_dependency" >&2
    echo "missing_dependency=$cmd" >&2
    exit 69
  }
done

aws_args=(s3api)
[[ -z "$endpoint" ]] || aws_args+=(--endpoint-url "$endpoint")

# Provider must expose an explicit bucket-encryption configuration matching the
# production contract. Object uploads intentionally do not send an SSE header:
# this proves that provider-side default encryption is actually applied.
enc=$(aws "${aws_args[@]}" get-bucket-encryption --bucket "$bucket" --output json) || {
  echo "dr_provider_smoke_result=encryption_unavailable" >&2
  exit 74
}
python3 - "$enc" "$expected_encryption" <<'PY'
import json, sys
obj=json.loads(sys.argv[1])
expected=sys.argv[2]
rules=obj.get("ServerSideEncryptionConfiguration",{}).get("Rules",[])
if not rules:
    print("dr_provider_smoke_result=encryption_missing")
    raise SystemExit(74)
default=rules[0].get("ApplyServerSideEncryptionByDefault",{})
if default.get("SSEAlgorithm") != expected:
    print("dr_provider_smoke_result=encryption_mismatch")
    raise SystemExit(74)
PY

# Reuse the qualified retention checker. This verifies that the remote provider
# can persist and return the exact lifecycle contract used by IRCIntel.
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
"$script_dir/s3-retention-policy.sh" "$bucket" check >/dev/null

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
src="$work/source.bin"
dst="$work/download.bin"
key="ircintel/provider-smoke/$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM.bin"

python3 - "$src" <<'PY'
import os, sys
with open(sys.argv[1], "wb") as f:
    f.write(os.urandom(32768))
PY
sha=$(sha256sum "$src" | awk '{print $1}')
size=$(wc -c <"$src" | tr -d ' ')

aws "${aws_args[@]}" put-object \
  --bucket "$bucket" \
  --key "$key" \
  --body "$src" \
  --metadata "sha256=$sha,size-bytes=$size" >/dev/null

head=$(aws "${aws_args[@]}" head-object --bucket "$bucket" --key "$key" --output json)
python3 - "$head" "$expected_encryption" "$sha" "$size" <<'PY'
import json, sys
obj=json.loads(sys.argv[1])
expected, sha, size=sys.argv[2:]
if obj.get("ServerSideEncryption") != expected:
    print("dr_provider_smoke_result=object_encryption_mismatch")
    raise SystemExit(74)
meta=obj.get("Metadata",{})
if meta.get("sha256") != sha or meta.get("size-bytes") != size:
    print("dr_provider_smoke_result=metadata_mismatch")
    raise SystemExit(74)
PY

aws "${aws_args[@]}" get-object --bucket "$bucket" --key "$key" "$dst" >/dev/null
download_sha=$(sha256sum "$dst" | awk '{print $1}')
[[ "$download_sha" == "$sha" ]] || {
  echo "dr_provider_smoke_result=roundtrip_integrity_failed" >&2
  exit 74
}

aws "${aws_args[@]}" delete-object --bucket "$bucket" --key "$key" >/dev/null
if aws "${aws_args[@]}" head-object --bucket "$bucket" --key "$key" >/dev/null 2>&1; then
  echo "dr_provider_smoke_result=delete_verification_failed" >&2
  exit 74
fi

printf '%s\n' \
  "dr_provider_smoke_result=ok" \
  "provider_bucket=$bucket" \
  "provider_default_encryption=$expected_encryption" \
  "provider_lifecycle_verified=yes" \
  "provider_roundtrip_verified=yes" \
  "provider_delete_verified=yes"
