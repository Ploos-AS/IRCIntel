# M0.1 OCI runtime qualification

M0.1 qualifies the IRCIntel foundation as an actual runnable OCI workload.

## Qualification gates

The GitHub Actions workflow must prove all of the following on every push and pull request:

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. `go build ./cmd/ircintel` passes.
4. The OCI image builds from `Dockerfile`.
5. The container starts successfully.
6. The runtime user is non-root as defined by the image (`65532:65532`).
7. `GET /healthz` returns `ok`.
8. `GET /api/v1/version` returns JSON identifying the service as `IRCIntel`.

## Micro-VPS baseline

IRCIntel is intentionally designed for low-resource VPS deployments. M0.1 does not claim a measured minimum RAM footprint yet; that will be established with runtime measurements in a later qualification milestone.

Initial deployment target:

- 1 vCPU
- 768 MB to 1 GB RAM
- 10 GB or more storage
- amd64 or arm64
- rootless/non-root friendly OCI runtime

## Out of scope for M0.1

- publishing to GHCR
- multi-architecture manifest publication
- distributed probe traffic
- persistent storage
- production TLS termination

Those are separate milestones so that the foundation remains independently testable.
