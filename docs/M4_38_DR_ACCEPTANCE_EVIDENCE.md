# M4.38 — DR acceptance evidence

M4.38 makes the M4.37 production acceptance result durable and machine-readable instead of leaving the verdict only in terminal output.

## Evidence command

```sh
ops/postgres/dr-acceptance-record.sh BUCKET OUTPUT_JSON
```

The wrapper runs the real M4.37 production acceptance path and writes schema `ircintel.dr-acceptance-record.v1` with UTC start/completion timestamps, bucket name, result, preserved exit code, and a SHA-256 digest of the complete acceptance output.

The command preserves the M4.37 exit status. Failed acceptance therefore still fails closed while leaving evidence describing the failure.

The JSON intentionally does not persist credentials, endpoints, webhook URLs, KMS identifiers, or raw acceptance output.

## CI qualification

`.github/workflows/dr-acceptance-evidence.yml` uses the same LocalStack and alert-receiver contract as M4.37 and proves both paths:

- successful production acceptance produces an `ok` evidence record with exit code 0;
- deliberate provider encryption mismatch produces a `failed` evidence record with exit code 74 while the command itself returns 74;
- both records conform to the v1 schema and contain a 64-character SHA-256 digest.

The records are uploaded as GitHub Actions artifacts so a qualification run leaves durable evidence.

## Production use

Store the generated JSON in the deployment/operations evidence system after initial provisioning and after material changes to the DR destination, IAM policy, KMS policy/key, lifecycle policy, network path, or alert routing. Production scheduling and retention of these evidence records remain deployment responsibilities.
