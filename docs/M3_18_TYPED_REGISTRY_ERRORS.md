# M3.18 — Typed registry parent errors

M3.18 removes string matching from registry parent-not-found handling.

## Changes

- Added `ErrRegistryNetworkNotFound` and `ErrRegistryServerNotFound` sentinel errors.
- `SQLiteStore.UpsertNetworkServer` returns `ErrRegistryNetworkNotFound` when the referenced network is absent.
- `SQLiteStore.UpsertNetworkEndpoint` returns `ErrRegistryServerNotFound` when the referenced server is absent.
- `RegistryWriteHandler` now uses `errors.Is` instead of matching error-message text.
- Wrapped sentinel errors still map to HTTP 409 Conflict.
- Unrelated errors that merely contain the old text are no longer misclassified as parent-not-found conflicts.

## Rationale

Public/admin API behavior must not depend on internal error wording. Typed errors provide a stable contract between storage and HTTP layers and make future storage changes safer.

## Qualification

CI must pass Go tests, vet, build, and OCI runtime smoke tests before M3.18 is considered qualified.
