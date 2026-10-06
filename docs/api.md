# API contract

All JSON endpoints live under /api. Errors return {"error":"message"}. Mutations require application/json, one JSON value and no unknown fields. Browser writes must use the same Origin as the request Host. Requests require a loopback host.

| Method | Path | Behavior |
|---|---|---|
| GET | /health | Health and database mode |
| GET | /overview | Counts, states, config version, execution availability |
| GET | /discovery | Availability, auth source (never token), running/automatic state, retry deadline, last run |
| GET | /discovery/runs | Latest 50 durable scan records |
| POST | /discovery/run | {}; 202 accepted; 409 running; 429 backoff |
| POST | /discovery/pause | {}; durable preference and audit event |
| POST | /discovery/resume | {}; enable when positive interval is configured |
| POST | /discovery/cancel | {}; 202; audited cancellation; completed observations retained |
| GET | /opportunities | Canonical quality-score order (descending, ID tie-break) |
| GET | /opportunities/{id} | Explained quality ranking and separate effort |
| POST | /opportunities/{id}/proceed | 501: execution unavailable; no side effects |
| GET | /contributions | Persisted contribution records |
| GET | /agents | Empty until execution adapter exists |
| GET | /usage | Observed sessions (currently zero), null official allowance |
| GET | /events | Latest 100 events; after/entity query enables cursor reads |
| GET | /events/stream | SSE activity events; Last-Event-ID or after replay |
| GET | /config | Current version and full config snapshot |
| GET | /config/history | All immutable config versions, newest first |
| GET | /config/proposals | Retained pending/applied/cancelled proposals |
| POST | /config/proposals | {base_version,config,reason}; 201 proposal; 409 stale base |
| POST | /config/proposals/{id}/apply | {}; atomic version + audit event |
| POST | /config/proposals/{id}/cancel | {}; mark cancelled and append event |
| POST | /config/rollback | {version,base_version}; new version restoring old snapshot |
| POST | /operator/chat | {message}; deterministic allowlisted reply/action/proposal |

The frontend TypeScript contracts mirror internal/domain JSON tags. Browser/API integration tests exercise the serialized contract; future schema generation is a planned improvement.
Live opportunities include nullable evidence fields, source links, observation timestamp, configuration version and scan ID. Discovery writes use the same loopback/same-origin/strict-JSON checks. Demo mode cannot scan live GitHub issues.

