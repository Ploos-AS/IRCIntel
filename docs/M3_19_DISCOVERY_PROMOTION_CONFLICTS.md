# M3.19 — Discovery promotion conflict classification

M3.19 makes expected registry conflicts during discovery promotion explicit instead of treating them as generic storage failures.

## Behaviour

Promotion still requires an accepted discovery candidate and an existing registry server.

Before inserting the promoted endpoint, Core now checks both registry uniqueness constraints:

- the requested endpoint ID must not already exist;
- the same `(server_id, host, port, tls)` endpoint tuple must not already exist under another ID.

Either condition returns the typed sentinel error `ErrDiscoveryPromotionEndpointConflict`.

The HTTP promotion endpoint maps this condition to `409 Conflict`. Unexpected storage failures continue to use the existing service-unavailable path.

Candidate lookup also uses `errors.Is(err, sql.ErrNoRows)` instead of matching SQLite error text.

## Qualification

Regression coverage verifies:

- endpoint-ID conflict classification;
- endpoint-tuple conflict classification;
- HTTP `409 Conflict` mapping;
- wrapped sentinel errors remain classifiable with `errors.Is`;
- successful promotion and persisted audit behaviour remain unchanged.

## Scope

This milestone does not add merge/replace semantics for existing registry endpoints. Operators must resolve endpoint conflicts explicitly; promotion never mutates an existing endpoint implicitly.
