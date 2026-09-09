# M3.4 — Explicit discovery promotion

M3.4 adds the explicit administrative step that turns an accepted discovery candidate into a curated registry endpoint.

## Rules

- Discovery intake never publishes to the registry automatically.
- Only candidates already reviewed as `accepted` may be promoted.
- Promotion requires an existing registry server.
- Promotion creates exactly one registry endpoint from the candidate's normalized host, port, and TLS values.
- A candidate may be promoted only once.
- Promotion writes an audit record containing candidate ID, endpoint ID, server ID, promoter, optional note, and timestamp.
- Rejected or pending candidates cannot be promoted.

## API

- `POST /api/v1/discovery/candidates/promote`
- `GET /api/v1/discovery/promotions`

Both endpoints require `IRCINTEL_CORE_TOKEN`.

Example promotion payload:

```json
{
  "candidate_id": "dc_...",
  "endpoint_id": "libera-main-6697",
  "server_id": "libera-main",
  "promoter": "operator",
  "note": "verified against official network information"
}
```

## Qualification

Tests cover:

- pending candidates cannot be promoted;
- accepted candidates create the expected registry endpoint;
- missing registry parents are rejected;
- double promotion is rejected;
- audit records survive SQLite reopen.

This preserves the core trust boundary: discovery proposes, review accepts or rejects, and promotion is a separate explicit publication action.
