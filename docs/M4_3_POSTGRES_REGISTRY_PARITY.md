# M4.3 — PostgreSQL registry parity

M4.3 adds PostgreSQL implementations of the curated IRC network registry contract.

## Supported operations

`PostgresStore` now implements:

- `UpsertNetwork`
- `UpsertNetworkServer`
- `UpsertNetworkEndpoint`
- `RegistrySnapshot`

Validation and normalization match the SQLite backend:

- IDs and names are trimmed;
- endpoint hosts are lower-cased;
- empty required fields fail closed;
- missing network/server parents return the existing typed sentinel errors;
- upserts update existing records by ID;
- registry snapshots use deterministic ordering.

## Qualification

CI exercises the implementation against a real PostgreSQL 17 service. Regression coverage verifies create/update behavior, normalization, deterministic snapshot ordering, required-field validation, and typed parent errors.

## Runtime status

PostgreSQL is still not selected as the full IRCIntel runtime backend. Observation and registry parity are now present, but incident, discovery, promotion, network-status/statistics, and related persistence contracts must reach parity before `main` may safely expose PostgreSQL as a production backend.
