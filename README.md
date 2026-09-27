PigFlow is a local HTTP control service for long-distance pipeline pigging operations.

## Station telemetry health

Station devices report their device id, sample time and connection heartbeat; the
service keeps a health profile per `(station_id, device_id)` and classifies it as
`fault` (self-reported `fault_code`), `offline` (heartbeat older than 5m),
`late` (heartbeat fresh but sample older than 90s), or `healthy` — fault has the
highest priority.

- `POST /api/stations/telemetry` — data report or heartbeat ping (omit `sample_at` for a heartbeat;
  send `"fault_code":""` to clear a fault). Body:
  `{"station_id":"station-a","device_id":"station-a/main","sample_at":"2026-09-26T08:00:00Z","fault_code":"E1"}`
- `GET /api/health` — all profiles with the freshly evaluated state/reason, sorted by station and device.
- `GET /api/health/messages?station_id=&device_id=&limit=` — the verbatim raw reports (newest first,
  up to 100 per device). `station_id` is required; `device_id` and `limit` are optional.

Profiles and raw messages are kept in memory only; devices re-report after a restart.
New fault codes are additionally recorded as `device_fault` journal events.
