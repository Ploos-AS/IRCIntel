# M3.3 — Discovery candidate review workflow

M3.3 adds an explicit administrative review step between discovery intake and the curated IRCIntel registry.

## State model

Discovery candidates are created as `pending` and may transition exactly once to either:

- `accepted`
- `rejected`

A reviewed candidate cannot be changed by a second review request. A new observation of the same candidate updates its discovery evidence but does not reset the review decision.

Acceptance is a review decision only. It does **not** create a network, server, or endpoint in the curated registry. Registry mutation remains an explicit administrative action through the M3.1 registry write API.

## Audit record

Each review persists:

- candidate ID
- resulting status
- reviewer identity
- optional review note
- UTC review timestamp

Review records survive Core restarts and can be queried independently of discovery candidates.

## API

All review endpoints require `IRCINTEL_CORE_TOKEN`.

### Review candidate

`POST /api/v1/discovery/candidates/review`

Example payload:

```json
{
  "candidate_id": "dc_example",
  "status": "accepted",
  "reviewer": "operator-1",
  "note": "verified against operator documentation"
}
```

Responses:

- `200` review committed
- `400` malformed or invalid review
- `401` invalid bearer token
- `404` candidate does not exist
- `409` candidate has already been reviewed
- `503` review service unavailable or token not configured

### List audit records

`GET /api/v1/discovery/reviews`

Returns review records newest first.

## Qualification

M3.3 tests cover:

- pending → accepted transition
- candidate status visibility after review
- persisted reviewer/note/timestamp audit data
- rejection of a second review decision
- authentication
- HTTP `409` for repeated review
- persistence across SQLite reopen
- normal Go test/vet/build and OCI runtime qualification
