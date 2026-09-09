# M3.2 Discovery candidate intake

M3.2 introduces a quarantine-style intake layer between discovery and the curated IRCIntel registry.

Discovery candidates are not registry records. A candidate is stored separately with endpoint identity, source provenance, first/last seen timestamps, a seen counter, and review status. Newly observed candidates always start as `pending`.

## API

- `POST /api/v1/discovery/candidates`
- `GET /api/v1/discovery/candidates`
- optional `status=pending|accepted|rejected` filter on the read endpoint

Both endpoints require `IRCINTEL_CORE_TOKEN`. Discovery intake is deliberately unavailable when no Core token is configured.

## Candidate identity and deduplication

Candidate IDs are deterministic hashes over normalized host, port, TLS mode, and source. Repeated sightings from the same source update `last_seen`, `source_ref`, and `seen_count` while preserving `first_seen` and review status.

This keeps repeated probe observations from creating unbounded duplicate rows while retaining provenance by source.

## Trust boundary

M3.2 does not automatically create networks, servers, or registry endpoints from discovery data. Discovery cannot publish or promote itself into the curated registry. Candidate review/promotion is intentionally a separate future operation.

## Privacy

The model stores infrastructure metadata only: endpoint host, port, TLS mode, source identifier/reference, timestamps, and review state. It does not store IRC conversation content, private messages, credentials, nick activity history, or private channel content.

## Qualification

Tests cover deterministic deduplication, normalization, provenance updates, timestamps, pending state, authenticated HTTP intake/listing, status filtering, and disabled-without-token behavior.
