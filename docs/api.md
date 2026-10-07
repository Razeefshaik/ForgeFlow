# API contract

All JSON endpoints live under /api. Errors return {"error":"message"}. Mutations require application/json, one JSON value and no unknown fields. Browser writes must use the same Origin as the request Host. Requests require a loopback host.

| Method | Path | Behavior |
|---|---|---|
| GET | /health | Health and database mode |
| GET | /overview | Counts, states, config version, execution availability |
| GET | /discovery | Availability, auth source (never token), running/automatic state, retry deadline, last run |
| GET | /auth/github | Account, public Client ID, pending user code and safe messages; never access/refresh/device tokens |
| POST | /auth/github/configure | `{client_id:"..."}`; save public OAuth app identity once |
| POST | /auth/github/login | `{}`; start GitHub device authorization; backend polls at GitHub's interval |
| POST | /auth/github/logout | `{}`; clear saved/in-memory account; does not revoke GitHub authorization |
| POST | /auth/github/cancel | `{}`; cancel pending sign-in |
| GET | /discovery/runs | Latest 50 durable scan records |
| POST | /discovery/run | {}; 202 accepted; 409 running; 429 backoff |
| POST | /discovery/pause | {}; durable preference and audit event |
| POST | /discovery/resume | {}; enable when positive interval is configured |
| POST | /discovery/cancel | {}; 202; audited cancellation; completed observations retained |
| GET | /opportunities | Canonical quality-score order (descending, ID tie-break) |
| GET | /opportunities/{id} | Explained quality ranking and separate effort |
| POST | /opportunities/{id}/preview | {}; fresh issue/repository/configuration/commit and approval token; no workspace side effects |
| POST | /opportunities/{id}/proceed | {approved:true,token,execute:true}; 202 persisted approval and asynchronous preparation; 409 changed inputs; repeated approval returns same record |
| GET | /contributions/{id} | Persisted workspace state, branch, base commit and status |
| GET | /contributions | Persisted contribution records |
| GET | /agents | Latest 200 persisted real agent sessions |
| GET | /usage | All persisted agent token observations and durations; null official allowance |
| GET | /events | Latest 100 matching events in chronological order; entity filters the page; explicit after enables forward cursor replay |
| GET | /events/stream | SSE activity events; Last-Event-ID or after replay |
| GET | /config | Current version and full config snapshot |
| GET | /config/history | All immutable config versions, newest first |
| GET | /config/proposals | Retained pending/applied/cancelled proposals |
| POST | /config/proposals | {base_version,config,reason}; 201 proposal; 409 stale base |
| POST | /config/proposals/{id}/apply | {}; atomic version + audit event |
| POST | /config/proposals/{id}/cancel | {}; mark cancelled and append event |
| POST | /config/rollback | {version,base_version}; new version restoring old snapshot |
| POST | /operator/chat | {message}; AI structured allowlisted reply/proposal/confirmation; deterministic demo fallback |

The frontend TypeScript contracts mirror internal/domain JSON tags. Browser/API integration tests exercise the serialized contract; future schema generation is a planned improvement.
Live opportunities include nullable evidence fields, source links, observation timestamp, configuration version and scan ID. Discovery writes use the same loopback/same-origin/strict-JSON checks. Demo mode cannot scan live GitHub issues.


## Execution endpoints

GET /runtime reports mode, configured contribution root, installed CLI availability and model override (availability does not prove login).
GET /contributions/{id}/execution returns current plan/review/diff/report/PR status; GET /contributions/{id}/tests returns immutable real outcomes.
Active agent sessions append AgentHeartbeat events every ten seconds with elapsed time and the last output timestamp. A heartbeat indicates that the runner is waiting for the process; it does not prove coding progress. The contribution panel polls execution, tests and recent activity every three seconds, including when SSE is disconnected.
Contributor and reviewer inputs include bounded excerpts of the latest outcome for each command (at most 64 KiB of encoded test evidence). Complete saved test records remain in the audit store and a workspace-local `.forgeflow-runtime/evidence/` snapshot. Original output truncation is disclosed. Total inline task input is bounded to 256 KiB; oversized tasks retain a complete workspace snapshot that the agent is instructed to read. `AgentPromptPrepared` events record input byte counts and any overflow snapshot path.
POST /contributions/{id}/{action} accepts a strict JSON body. Actions: start/resume `{approved:true,network:false}`; pause/stop `{}`; abandon/approve-plan `{approved:true}`; tests/review `{approved:true}`; constraints `{approved:true,constraints:"..."}`; open-workspace `{}`; prepare-pr `{}`; submit-pr `{approved:true,token:"<prepared commit/text token>"}`.

Network defaults off. Execution requires an approved isolated workspace and CLI sandbox proof. READY follows real verification and independent review. Preparing commits does not submit a PR. Same-origin and loopback checks apply to every mutation. Operator temporary proposals carry expires_at; only Apply activates them.
