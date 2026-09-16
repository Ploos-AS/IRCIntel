#!/usr/bin/env bash
set -euo pipefail
usage() { echo "usage: $0 ACCEPTANCE_RECORD EVIDENCE_RESTORE MANIFEST MANIFEST_RESTORE OUTPUT_JSON" >&2; exit 64; }
[[ $# -eq 5 ]] || usage
record=$1; evidence_restore=$2; manifest=$3; manifest_restore=$4; output=$5
for f in "$record" "$evidence_restore" "$manifest" "$manifest_restore"; do [[ -f "$f" ]] || { echo "dr_audit_bundle_result=source_missing:$f" >&2; exit 66; }; done
for cmd in python3 sha256sum; do command -v "$cmd" >/dev/null 2>&1 || { echo "dr_audit_bundle_missing_command=$cmd" >&2; exit 69; }; done
python3 - "$record" "$evidence_restore" "$manifest" "$manifest_restore" "$output" <<'PY'
import hashlib,json,os,sys
paths=sys.argv[1:5]; out=sys.argv[5]
def load(p):
    try:
        with open(p,encoding='utf-8') as f: return json.load(f)
    except Exception:
        print('dr_audit_bundle_result=invalid_data:'+p,file=sys.stderr); raise SystemExit(74)
def sha(p):
    h=hashlib.sha256()
    with open(p,'rb') as f:
        for b in iter(lambda:f.read(131072),b''): h.update(b)
    return h.hexdigest()
r,er,m,mr=map(load,paths)
try:
    checks=[
      r.get('schema')=='ircintel.dr-acceptance-record.v1',
      m.get('schema')=='ircintel.dr-evidence-manifest.v1', m.get('result')=='ok',
      mr.get('schema')=='ircintel.dr-evidence-manifest-restore.v1', mr.get('result')=='ok', mr.get('metadata_verified') is True,
      er.get('schema')=='ircintel.dr-evidence-restore.v1', er.get('result')=='ok', er.get('metadata_verified') is True,
      m.get('evidence_sha256')==sha(paths[0]),
      m.get('evidence_key')==er.get('source_key')==mr.get('evidence_key'),
      m.get('evidence_sha256')==er.get('evidence_sha256')==mr.get('evidence_sha256'),
      mr.get('manifest_sha256')==sha(paths[2]),
      mr.get('mode')==m.get('mode'), mr.get('acceptance_result')==m.get('acceptance_result'),
    ]
except Exception:
    print('dr_audit_bundle_result=invalid_data',file=sys.stderr); raise SystemExit(74)
if not all(checks):
    print('dr_audit_bundle_result=integrity_failed',file=sys.stderr); raise SystemExit(74)
b={'schema':'ircintel.dr-audit-bundle.v1','result':'ok','mode':m['mode'],'acceptance_result':m['acceptance_result'],'evidence_key':m['evidence_key'],'evidence_sha256':m['evidence_sha256'],'manifest_key':mr['source_key'],'manifest_sha256':mr['manifest_sha256'],'acceptance_record_sha256':sha(paths[0]),'evidence_restore_sha256':sha(paths[1]),'manifest_restore_sha256':sha(paths[3]),'chain_verified':True}
os.makedirs(os.path.dirname(out) or '.',exist_ok=True)
with open(out,'w',encoding='utf-8') as f: json.dump(b,f,sort_keys=True,separators=(',',':')); f.write('\n')
PY
bundle_sha=$(sha256sum "$output"|awk '{print $1}')
echo "dr_audit_bundle_result=ok"
echo "dr_audit_bundle=$output"
echo "dr_audit_bundle_sha256=$bundle_sha"
echo "dr_audit_bundle_verified=true"
