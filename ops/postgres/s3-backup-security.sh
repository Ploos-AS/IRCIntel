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

endpoint=${IRCINTEL_BACKUP_S3_ENDPOINT:-${IRCINTEL_WAL_S3_ENDPOINT:-${IRCINTEL_BASEBACKUP_S3_ENDPOINT:-}}}
encryption=${IRCINTEL_BACKUP_S3_ENCRYPTION:-AES256}
kms_key=${IRCINTEL_BACKUP_S3_KMS_KEY_ID:-}

if [[ "$encryption" != "AES256" && "$encryption" != "aws:kms" ]]; then
  echo "backup_security_result=invalid_encryption" >&2
  exit 64
fi
if [[ "$encryption" == "aws:kms" && -z "$kms_key" ]]; then
  echo "backup_security_result=kms_key_required" >&2
  exit 64
fi

aws_args=(s3api)
[[ -z "$endpoint" ]] || aws_args+=(--endpoint-url "$endpoint")

enc=$(mktemp)
policy=$(mktemp)
trap 'rm -f "$enc" "$policy"' EXIT

python3 - "$encryption" "$kms_key" >"$enc" <<'PY'
import json, sys
algo, key = sys.argv[1:]
rule={"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":algo},"BucketKeyEnabled":True}
if algo == "aws:kms":
    rule["ApplyServerSideEncryptionByDefault"]["KMSMasterKeyID"]=key
print(json.dumps({"Rules":[rule]}, separators=(",", ":")))
PY

python3 - "$bucket" >"$policy" <<'PY'
import json, sys
bucket=sys.argv[1]
arn=f"arn:aws:s3:::{bucket}"
print(json.dumps({
 "Version":"2012-10-17",
 "Statement":[
  {"Sid":"DenyInsecureTransport","Effect":"Deny","Principal":"*","Action":"s3:*","Resource":[arn,arn+"/*"],"Condition":{"Bool":{"aws:SecureTransport":"false"}}},
  {"Sid":"DenyUnencryptedObjectUploads","Effect":"Deny","Principal":"*","Action":"s3:PutObject","Resource":arn+"/*","Condition":{"Null":{"s3:x-amz-server-side-encryption":"true"}}}
 ]
}, separators=(",", ":")))
PY

if [[ "$mode" == "apply" ]]; then
  aws "${aws_args[@]}" put-public-access-block --bucket "$bucket" --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
  aws "${aws_args[@]}" put-bucket-encryption --bucket "$bucket" --server-side-encryption-configuration "file://$enc"
  aws "${aws_args[@]}" put-bucket-policy --bucket "$bucket" --policy "file://$policy"
fi

pab=$(aws "${aws_args[@]}" get-public-access-block --bucket "$bucket" --output json)
actual_enc=$(aws "${aws_args[@]}" get-bucket-encryption --bucket "$bucket" --output json)
actual_policy=$(aws "${aws_args[@]}" get-bucket-policy --bucket "$bucket" --query Policy --output text)

python3 - "$pab" "$actual_enc" "$actual_policy" "$encryption" "$kms_key" <<'PY'
import json, sys
pab=json.loads(sys.argv[1]).get("PublicAccessBlockConfiguration",{})
enc=json.loads(sys.argv[2]).get("ServerSideEncryptionConfiguration",{}).get("Rules",[])
policy=json.loads(sys.argv[3])
algo=sys.argv[4]
key=sys.argv[5]
if not all(pab.get(k) is True for k in ("BlockPublicAcls","IgnorePublicAcls","BlockPublicPolicy","RestrictPublicBuckets")):
    print("backup_security_result=public_access_mismatch"); raise SystemExit(74)
if not enc:
    print("backup_security_result=encryption_missing"); raise SystemExit(74)
default=enc[0].get("ApplyServerSideEncryptionByDefault",{})
if default.get("SSEAlgorithm") != algo:
    print("backup_security_result=encryption_mismatch"); raise SystemExit(74)
if algo == "aws:kms" and default.get("KMSMasterKeyID") != key:
    print("backup_security_result=kms_key_mismatch"); raise SystemExit(74)
sids={s.get("Sid") for s in policy.get("Statement",[]) if s.get("Effect")=="Deny"}
if not {"DenyInsecureTransport","DenyUnencryptedObjectUploads"}.issubset(sids):
    print("backup_security_result=policy_mismatch"); raise SystemExit(74)
print("backup_security_result=ok")
print(f"backup_encryption={algo}")
print("public_access_block=enabled")
print("secure_transport_required=yes")
print("encrypted_upload_required=yes")
PY
