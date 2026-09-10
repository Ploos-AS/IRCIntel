# M4.6 — PostgreSQL discovery/review/promotion parity

M4.6 ports the curated discovery workflow to PostgreSQL while keeping PostgreSQL disabled as the full runtime backend until the remaining Core contracts are qualified.

## PostgreSQL schema v4

Adds:

- `discovery_candidates`
- `discovery_reviews`
- `discovery_promotions`

All timestamps use `TIMESTAMPTZ`. Candidate status is constrained to `pending|accepted|rejected`; review status to `accepted|rejected`. Promotions reference both the candidate and curated registry endpoint/server.

## Candidate intake

`PostgresStore` now implements `UpsertDiscoveryCandidate` and `ListDiscoveryCandidates` with the same deterministic candidate ID and normalization contract as SQLite. Repeated sightings update `source_ref`, `last_seen`, and `seen_count` without resetting review state.

## Review workflow

`ReviewDiscoveryCandidate` and `ListDiscoveryReviews` are implemented. Review is transactional and locks the candidate row with `SELECT ... FOR UPDATE`, preserving a single pending-to-accepted/rejected transition under concurrent reviewers. Existing typed errors remain the public contract.

## Promotion workflow

`PromoteDiscoveryCandidate` and `ListDiscoveryPromotions` are implemented transactionally. Promotion requires an accepted candidate and existing server, rejects duplicate promotion and endpoint conflicts with the existing typed errors, inserts the curated endpoint, and records the audit row in one transaction.

## Qualification

CI uses PostgreSQL 17 and covers:

- deterministic intake/dedup and seen-count updates;
- accepted review plus duplicate-review rejection;
- promotion into the curated registry;
- persisted review/promotion listing;
- missing candidate/server typed errors;
- promotion-before-acceptance rejection;
- endpoint conflict classification.

## Remaining production work

PostgreSQL is still not selected by `cmd/ircintel` as a complete application backend. Probe-plan/network-status/statistics interfaces and the runtime storage abstraction still need parity before activation. Historical rollups, retention and production backup/restore qualification remain later M4 work.
