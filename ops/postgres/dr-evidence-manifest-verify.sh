#!/usr/bin/env bash
set -euo pipefail

usage() { echo "usage: $0 MANIFEST_JSON EVIDENCE_JSON RESTORE_JSON" >&2; exit 64; }
[[ $# -eq 3 ]] || usage
manifest=$1
evidence=$2
restore=$3

for cmd in python3 sha256sum stat; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "dr_evidence_manifest_verify_missing_command=$cmd" >&2; exit 69; }
done
[[ -f "$manifest" && -f "$evidence" && -f "$restore" ]] || { echo "dr_evidence_manifest_verify_result=source_missing" >&2; exit 66; }

sha=$(sha256sum "$evidence" | awk '{print $1}')
size=$(stat -c %s "$evidence")

python3 - "$manifest" "$evidence" "$restore" "$sha" "$size" <<'PY'
import json,sys
manifest_path,evidence_path,restore_path,sha,size=sys.argv[1:]
try:
    manifest=json.load(open(manifest_path,encoding='utf-8'))
    evidence=json.load(open(evidence_path,encoding='utf-8'))
    restore=json.load(open(restore_path,encoding='utf-8'))
except Exception as exc:
    print(f'dr_evidence_manifest_verify_result=invalid_json:{exc}',file=sys.stderr)
    raise SystemExit(74)

def require(ok):
    if not ok:
        print('dr_evidence_manifest_verify_result=integrity_failed',file=sys.stderr)
        raise SystemExit(74)

require(manifest.get('schema')=='ircintel.dr-evidence-manifest.v1')
require(manifest.get('result')=='ok')
require(manifest.get('metadata_verified') is True and manifest.get('restore_verified') is True)
require(evidence.get('schema')=='ircintel.dr-acceptance-record.v1')
require(restore.get('schema')=='ircintel.dr-evidence-restore.v1')
require(restore.get('result')=='ok' and restore.get('metadata_verified') is True)
require(manifest.get('evidence_key')==restore.get('source_key'))
require(manifest.get('evidence_sha256')==sha==restore.get('restored_sha256'))
require(int(manifest.get('evidence_size_bytes',-1))==int(size)==int(restore.get('restored_size_bytes',-2)))
require(manifest.get('mode')==evidence.get('mode')==restore.get('archive_mode'))
require(manifest.get('acceptance_result')==evidence.get('result')==restore.get('archive_result'))
print('dr_evidence_manifest_verify_result=ok')
print(f"dr_evidence_manifest_verify_key={manifest['evidence_key']}")
print(f"dr_evidence_manifest_verify_sha256={sha}")
PY
