# M4.41 — DR evidence restore verification

M4.41 proves that evidence archived by M4.40 can be retrieved from the off-host S3 boundary and independently hashed after restore.

The qualification does not expose or persist the restored evidence payload. It verifies object existence, downloads to a temporary file, records size and SHA-256, and removes the temporary payload before completion.

The restore verifier accepts an optional `IRCINTEL_BACKUP_S3_ENDPOINT` for CI S3-compatible providers and otherwise uses the normal AWS endpoint configuration.

Production use should periodically select a known-good archived acceptance record and run the verifier. A restore failure is a DR evidence incident and must not be treated as a successful backup cycle.
