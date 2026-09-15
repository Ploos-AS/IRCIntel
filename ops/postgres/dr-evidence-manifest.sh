#!/usr/bin/env bash
set -euo pipefail

usage() { echo "usage: $0 EVIDENCE_JSON ARCHIVE_KEY RESTORE_JSON OUTPUT_JSON" >&2; exit 64; }
[[ $# -eq 4 ]] || usage

evidence=$1
archive_key=$2
restore=$3
output=$4

for cmd in python3 sha256sum stat date; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "dr_evidence_manifest_missing_command=$cmd" >&2; exit 69; }
done
[[ -f "$evidence" && -f "$restore" ]] || { echo "dr_evidence_manifest_result=source_missing" >&2; exit 66; }
[[ -n "$archive_key" ]] || usage

sha=$(sha256sum "$evidence" | awk '{print $1}')
size=$(stat -c %s "$evidence")
created=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$(dirname "$output")"

python3 - "$evidence" "$restore" "$archive_key" "$sha" "$size" "$created" "$output" <<'PY'
import json,sys
src_path,restore_path,key,sha,size,created,out=sys.argv[1:]
try:
    src=json.load(open(src_path,encoding='utf-8'))
    restore=json.load(open(restore_path,encoding='utf-8'))
except Exception as exc:
    print(f'dr_evidence_manifest_result=invalid_json:{exc}', file=sys.stderr)
    raise SystemExit(74)
if src.get('schema')!='ircintel.dr-acceptance-record.v1': raise SystemExit(74)
if restore.get('schema')!='ircintel.dr-evidence-restore.v1': raise SystemExit(74)
if restore.get('result')!='ok' or restore.get('metadata_verified') is not True: raise SystemExit(74)
if restore.get('source_key')!=key: raise SystemExit(74)
if restore.get('restored_sha256')!=sha or int(restore.get('restored_size_bytes',-1))!=int(size): raise SystemExit(74)
if restore.get('archive_mode')!=src.get('mode') or restore.get('archive_result')!=src.get('result'): raise SystemExit(74)
record={
 'schema':'ircintel.dr-evidence-manifest.v1','created_at':created,
 'evidence_key':key,'evidence_sha256':sha,'evidence_size_bytes':int(size),
 'mode':src.get('mode'),'acceptance_result':src.get('result'),
 'metadata_verified':True,'restore_verified':True,'result':'ok'
}
with open(out,'w',encoding='utf-8') as f:
    json.dump(record,f,separators=(',',':'),sort_keys=True); f.write('\n')
print('dr_evidence_manifest_result=ok')
print(f'dr_evidence_manifest_key={key}')
print(f'dr_evidence_manifest_sha256={sha}')
PY
