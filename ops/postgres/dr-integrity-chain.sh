#!/usr/bin/env bash
set -euo pipefail
usage() { echo "usage: $0 [ci|production] BUCKET EVIDENCE_DIR" >&2; exit 64; }
[[ $# -eq 3 ]] || usage
mode=$1; bucket=$2; evidence_dir=$3
[[ "$mode" == "ci" || "$mode" == "production" ]] || usage
mkdir -p "$evidence_dir"; log="$evidence_dir/integrity-chain.log"; : > "$log"
set +e
ops/postgres/dr-acceptance-scheduled.sh "$mode" "$bucket" "$evidence_dir" >"$log" 2>&1; rc=$?
set -e
cat "$log"; (( rc == 0 )) || { echo "dr_integrity_chain_result=scheduled_failed"; exit "$rc"; }
record=$(grep '^dr_schedule_evidence=' "$log"|tail -1|cut -d= -f2-); key=$(grep '^dr_evidence_archive_key=' "$log"|tail -1|cut -d= -f2-)
[[ -n "$record" && -f "$record" ]] || { echo "dr_integrity_chain_result=record_missing" >&2; exit 66; }; [[ -n "$key" ]] || { echo "dr_integrity_chain_result=archive_key_missing" >&2; exit 66; }
restore="$evidence_dir/dr-evidence-restore.json"; manifest="$evidence_dir/dr-evidence-manifest.json"
ops/postgres/dr-evidence-restore-verify.sh "$bucket" "$key" "$restore"
ops/postgres/dr-evidence-manifest.sh "$record" "$key" "$restore" "$manifest"
ops/postgres/dr-evidence-manifest-verify.sh "$manifest" "$record" "$restore"
manifest_archive=$(ops/postgres/dr-evidence-manifest-s3.sh "$manifest"); printf '%s\n' "$manifest_archive"
manifest_key=$(printf '%s\n' "$manifest_archive"|grep '^dr_manifest_archive_key='|tail -1|cut -d= -f2-); [[ -n "$manifest_key" ]] || { echo "dr_integrity_chain_result=manifest_archive_key_missing" >&2; exit 66; }
manifest_restore="$evidence_dir/dr-evidence-manifest-restore.json"
ops/postgres/dr-evidence-manifest-restore-verify.sh "$bucket" "$manifest_key" "$manifest_restore"
audit_bundle="$evidence_dir/dr-audit-bundle.json"
ops/postgres/dr-audit-bundle.sh "$record" "$restore" "$manifest" "$manifest_restore" "$audit_bundle"
audit_archive=$(ops/postgres/dr-audit-bundle-s3.sh "$audit_bundle"); printf '%s\n' "$audit_archive"
audit_key=$(printf '%s\n' "$audit_archive"|grep '^dr_audit_bundle_archive_key='|tail -1|cut -d= -f2-); [[ -n "$audit_key" ]] || { echo "dr_integrity_chain_result=audit_bundle_archive_key_missing" >&2; exit 66; }
sha=$(sha256sum "$record"|awk '{print $1}'); manifest_sha=$(sha256sum "$manifest"|awk '{print $1}'); audit_sha=$(sha256sum "$audit_bundle"|awk '{print $1}')
echo "dr_integrity_chain_result=ok"; echo "dr_integrity_chain_record=$record"; echo "dr_integrity_chain_key=$key"; echo "dr_integrity_chain_sha256=$sha"; echo "dr_integrity_chain_restore=$restore"; echo "dr_integrity_chain_manifest=$manifest"; echo "dr_integrity_chain_manifest_sha256=$manifest_sha"; echo "dr_integrity_chain_manifest_verified=true"; echo "dr_integrity_chain_manifest_archive_key=$manifest_key"; echo "dr_integrity_chain_manifest_archived=true"; echo "dr_integrity_chain_manifest_restore=$manifest_restore"; echo "dr_integrity_chain_manifest_restore_verified=true"; echo "dr_integrity_chain_audit_bundle=$audit_bundle"; echo "dr_integrity_chain_audit_bundle_sha256=$audit_sha"; echo "dr_integrity_chain_audit_bundle_verified=true"; echo "dr_integrity_chain_audit_bundle_archive_key=$audit_key"; echo "dr_integrity_chain_audit_bundle_archived=true"
