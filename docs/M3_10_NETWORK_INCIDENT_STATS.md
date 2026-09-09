# M3.10 — Network incident statistics

IRCIntel now exposes server-side incident statistics suitable for dashboards and public status views.

`GET /api/v1/networks/incidents/stats`

Optional filters:

- `network_id=<registry network id>`
- `since=<RFC3339Nano>`
- `until=<RFC3339Nano>`

The response contains:

- total, open, and closed incident counts
- degraded and down incident counts
- total downtime seconds across closed incidents
- mean recovery time in seconds for closed incidents with a duration
- maximum closed incident duration
- latest incident start time

`since` and `until` are inclusive and apply to incident `started_at`. Reversed ranges are rejected. The endpoint uses persisted network-incident records and therefore survives Core restarts.

Statistics are intentionally based only on curated registry endpoints. Discovery candidates remain excluded until explicit promotion.

The current implementation summarizes at most the newest 500 matching persisted incidents. Removing that bound with SQL-native aggregate queries is deferred to a later scalability milestone.
