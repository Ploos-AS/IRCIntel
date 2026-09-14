# M4.38 — DR acceptance evidence

M4.38 makes the M4.37 acceptance result durable and machine-readable instead of leaving the verdict only in terminal output.

## Evidence command

```sh
ops/postgres/dr-acceptance-record.sh production BUCKET OUTPUT_JSON
```

The wrapper runs the real M4.37 acceptance path in the selected mode and writes schema `ircintel.dr-acceptance-record.v1` with UTC start/completion timestamps, mode, bucket name, result, preserved exit code, and a SHA-256 digest of the complete acceptance output.

The command preserves the M4.37 exit status. Failed acceptance therefore still fails closed while leaving evidence describing the failure.

The JSON intentionally does not persist credentials, endpoints, webhook URLs, KMS identifiers, or raw acceptance output.

## CI qualification

`.github/workflows/dr-acceptance-evidence.yml` runs the wrapper in `ci` mode against LocalStack 4.4.0 and the local alert receiver. This is deliberate: production readiness must continue to reject loopback/HTTP endpoints, so CI must not weaken or bypass that safety contract.

The workflow proves both evidence paths:

- successful CI acceptance produces an `ok` evidence record with exit code 0 and `mode=ci`;
- deliberate provider encryption mismatch produces a `failed` evidence record with exit code 74 while the wrapper itself returns 74;
- both records conform to the v1 schema and contain a 64-character SHA-256 digest.

The records are uploaded as GitHub Actions artifacts so a qualification run leaves durable evidence.

CI qualifies the evidence mechanism and verdict preservation. It does not claim production readiness for LocalStack or the local HTTP alert receiver.

## Production use

Use `production` mode only with the real off-host provider and HTTPS alert route:

```sh
ops/postgres/dr-acceptance-record.sh production BUCKET OUTPUT_JSON
```

Store the generated JSON in the deployment/operations evidence system after initial provisioning and after material changes to the DR destination, IAM policy, KMS policy/key, lifecycle policy, network path, or alert routing. Production scheduling and retention of these evidence records remain deployment responsibilities.
