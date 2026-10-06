# ForgeFlow

A local-first open-source contribution control plane, built as a React/TypeScript dashboard and a Go modular monolith with SQLite.

**Current delivery: working foundation and live GitHub discovery.** Contribution execution remains pending. Original rules and specification are preserved; this isolated repository lives at `contributions/forgeflow-foundation/repo` under the supplied ForgeFlow folder.

## Run on Windows
Requirements: Go 1.26+, Node 24 LTS, npm and Git. Docker is not required.

From this repository:
```powershell
npm run setup
npm run dev
```
Open http://127.0.0.1:5173. This explicitly starts demo mode. Ctrl+C stops both processes.

For real GitHub discovery:
```powershell
npm run dev:live
```
Open Opportunities and select **Run discovery now**. Automatic scans run every 15 minutes while the server runs; Pause/Resume persists. Public discovery works without credentials, with smaller scan limits. Authenticated public scans use `GH_TOKEN`, `GITHUB_TOKEN`, or a previous `gh auth login`. Never paste tokens into configuration or Operator. The chat's GitHub plugin does not authenticate the running app.

Build and run the dashboard with a single Go process:
```powershell
npm run build
.\bin\forgeflow.exe --demo
```
Open http://127.0.0.1:8080. On Linux use `./bin/forgeflow --demo`.

Separate development terminals:
```powershell
go run ./apps/server --demo
npm --prefix apps/web run dev
```

## Implemented
- React dashboard with responsive navigation, dark/light themes, opportunities table, filters, score breakdown, contribution records, event timeline, agent/usage status and Operator panel.
- SQLite embedded migrations, mode separation, transactionally persisted events and lifecycle changes, immutable audit/config history.
- Validated contribution state machine and bounded retry configuration.
- Configuration proposals, explicit Apply/Cancel, stale-version checks and rollback as a new version.
- Reconnectable SSE replay and frontend cache invalidation.
- Explained quality ranking, independent heuristic effort estimates and the required ranking-independence regression test.
- Explicit, idempotent demo seeding; no mocked data in live mode.
- Deterministic Operator for inspection and a narrowly scoped Rust proposal.
- Read-only GitHub discovery: bounded requests, credential lookup, rate-limit backoff, memory cache and ETag revalidation.
- Repository/issue evidence, eleven explained quality factors, timestamped snapshots and durable scan history.
- Live Run/Cancel and automatic Pause/Resume controls; demo mode cannot fetch live issues.

## Honest boundaries
External workspace creation, Codex contributor/reviewer adapters, real contribution tests, final reports and PR preparation remain future phases. Proceed is disabled; the API returns 501 without starting work. No repository has been cloned by the application. Demo states, issues and scores are illustrative; they are not claims about the named repositories.
Live discovery is a bounded public-repository sample. File presence does not prove tests/CI pass. Acceptance signals are heuristics, not merge probabilities. Older observations remain visible with their timestamp/profile version. See [discovery](docs/discovery.md).
The Operator is not connected to an AI model. It cannot run shell commands. Multi-setting natural-language requests require the configuration editor.
Plugin authentication in the build chat is separate from runtime authentication.

## Persistence and environment
No credentials are required for demo mode or public discovery. Go flags:
- `--demo`: labeled seed mode; default database `data/demo.db`.
- default live database: `data/forgeflow.db`.
- `--db <path>`: explicit SQLite file. A database cannot switch modes.
- `--listen 127.0.0.1:8080`: numeric loopback bind only.
- `--web apps/web/dist`: built frontend directory.
- `--discovery-interval 15m`: automatic live scan interval; `0` disables scheduling while keeping Run available. Positive values must be at least one minute.

GitHub credentials remain in server memory, outside SQLite, events and the browser. Restart the server after changing credentials. Codex authentication is not integrated yet.
Contribution workspaces will live at `contributions/<id>/repo`; metadata will live alongside them under `.autopilot`.

## Verification
```powershell
go test ./...
go vet ./...
npm --prefix apps/web run build
# with both dev servers running in DEMO mode:
npm --prefix apps/web test
```
Browser tests use installed Edge on Windows. On Linux run `npx playwright install chromium` in apps/web first. Run browser tests against a disposable demo database if you want to keep normal demo config untouched; tests preserve history and create real config versions.
Four browser checks run by default, and two live checks are skipped. To run all six against an already-scanned disposable live server:
```powershell
$env:FORGEFLOW_LIVE_TEST_URL='http://127.0.0.1:8081'
npm --prefix apps/web test
```
The live Run check performs read-only GitHub requests and appends scan/audit history.

## Approval rules
Human approval precedes contribution execution and PR submission. No automatic PR submission/merge. Config changes wait for Apply. Codex effort never influences canonical ranking.

## Structure
```
apps/server           Go HTTP entry point
apps/web              React application and browser tests
internal/api          REST, SSE, same-origin request checks
internal/analyzer     Evidence-derived quality factors and tree inspection
internal/discovery    Bounded scanning, cancellation and scheduling
internal/github       Read-only REST, credentials, cache and rate limits
internal/config       Default profile and validation
internal/contributions Persisted lifecycle validation
internal/domain       JSON contracts
internal/estimator    Independent heuristic scope estimate
internal/operator     Allowlisted foundation dispatcher
internal/ranking      Explained quality scoring
internal/seed         Explicit demo data
internal/storage      SQLite transactions and migrations
scripts               Cross-platform startup and checks
docs                  Architecture, workflow, API, security and milestones
```

See [architecture](docs/architecture.md), [workflow](docs/workflow.md), [API](docs/api.md), [design](docs/design.md) and [milestones](docs/milestones.md).
See [verification](docs/verification.md) for actual check results and desktop/mobile screenshots.

