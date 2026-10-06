# ForgeFlow

A local-first open-source contribution control plane, built as a React/TypeScript dashboard and a Go modular monolith with SQLite.

**Current delivery: live discovery, approved Git workspaces, Codex planning/coding, real verification with bounded fixes, independent review, reports and human-approved PR submission.** ForgeFlow lives in this repository root. External clones use the configured workspace root; this installation uses `S:/StudyResource/TechBoooo/Backend/Contriss/<contribution-id>/repo`.

## Run on Windows
Requirements: Go 1.26+, Node 24 LTS, npm, Git and an authenticated Codex CLI (`codex login`) for execution/AI Operator. Docker is not required. Codex runs finite CLI sessions with a workspace write sandbox; a real positive/negative write probe must pass before coding.

From this repository:
```powershell
npm run setup
npm run dev
```
Open http://127.0.0.1:5173. This starts live mode with real GitHub fetching. Ctrl+C stops both processes.

For illustrative demo data:
```powershell
npm run dev:demo
```
Open Opportunities and select **Run discovery now**. In live mode, automatic scans start after startup, then run every 15 minutes while the server runs; Pause/Resume persists. Public discovery works without credentials, with smaller scan limits. Open **GitHub account** to sign in through your browser and stay connected across restarts. A [one-time OAuth Client ID setup](docs/github-sign-in.md) identifies ForgeFlow to GitHub; no personal token or client secret is needed. Windows encrypts saved credentials for your account. Environment variables and an existing `gh auth login` remain supported as fallbacks. Never paste tokens into configuration or Operator. The chat's GitHub plugin does not authenticate the running app.

Build and run the dashboard with a single Go process:
```powershell
npm run build
.\bin\forgeflow.exe
```
Open http://127.0.0.1:8080. On Linux use `./bin/forgeflow`. Add `--demo` for illustrative data.

Separate development terminals:
```powershell
go run ./apps/server
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
- AI Operator translates natural language into validated configuration proposals or confirmation cards for contribution controls. Exact Add Rust also has a deterministic shortcut; demo mode uses a deterministic dispatcher.
- Read-only GitHub discovery: bounded requests, credential lookup, rate-limit backoff, memory cache and ETag revalidation.
- Repository/issue evidence, eleven explained quality factors, timestamped snapshots and durable scan history.
- Live Run/Cancel and automatic Pause/Resume controls; demo mode cannot fetch live issues.
- Approved isolated Git clones, pinned branches, input snapshots, real command outcomes and restart recovery. See [workspace preparation](docs/workspaces.md).

- Fresh planner and independent reviewer contexts, persisted plans, real sandboxed test/lint/build commands, bounded fix/review loops and restart recovery.
- Pause/resume/stop/abandon, contribution constraints, workspace folder opening, diff/test/review/report views and observed agent/token usage.
- Temporary configuration proposals expire through a new configuration version only if no newer manual settings superseded them.
- Local commit/PR preparation and a separate approval bound to the commit and PR text before fork, push and PR creation. No auto submission or merge.

## Execution and boundaries
Inspect a real issue, select **Proceed to Contribute**, review fresh inputs and explicitly approve cloning, coding, verification and independent review. Existing preparation-only contributions require **Start coding** approval. Enabling plan approval pauses before edits; approve the plan, then resume. Dependency network access is off by default and can be explicitly enabled when starting/resuming.

Passing commands and an independent APPROVE produce READY. Inspect the diff and report, prepare the local commit/PR, then separately approve submission. GitHub submission requires a connected GitHub account with public repository contribution permissions (or an environment/CLI credential with fork/push/PR permissions). Build-chat plugin authentication does not authenticate the running application. No PR is automatically submitted or merged.

Discovery is a bounded public-repository sample, and quality/acceptance signals are heuristics. Demo records are illustrative. Official remaining Codex plan allowance is unavailable; Usage shows observed local events and a clearly labeled manual entry. Codex effort never changes ranking.

## Persistence and environment
No credentials are required for demo mode or public discovery. Go flags:
- `--demo`: labeled seed mode; default database `data/demo.db`.
- `--root <directory>`: optional explicit ForgeFlow root. By default the server locates it from the executable or working directory. Relative database/frontend paths resolve against that root.
- default live database: `data/forgeflow.db`.
- `--db <path>`: explicit SQLite file. A database cannot switch modes.
- `--listen 127.0.0.1:8080`: numeric loopback bind only.
- `--web apps/web/dist`: built frontend directory.
- `--discovery-interval 15m`: automatic live scan interval; `0` disables scheduling while keeping Run available. Positive values must be at least one minute.

GitHub credentials remain in server memory, outside SQLite, events and the browser. Restart after changing credentials or local runtime settings. Codex uses the installed CLI login; this installation was verified with ChatGPT login. CLI subprocesses ignore user configuration/rules/MCP connections and run with explicit permissions.
Set `contributions_dir` in ignored `configs/local.json`, `FORGEFLOW_CONTRIBUTIONS_DIR`, or `--contributions-dir`. Absolute paths are supported; relative paths resolve against ForgeFlow root. `configs/example.json` is portable. This machine is configured for `S:/StudyResource/TechBoooo/Backend/Contriss`. Existing repository, metadata and artifacts were moved intact; startup appends a relocation event without rewriting historical approvals.
Optional `FORGEFLOW_CODEX_BIN` / `FORGEFLOW_CODEX_MODEL` override CLI resolution/model. Workspaces resolve as `<ConfiguredRoot>/<id>/repo`; `.autopilot` and `artifacts` sit alongside `repo`. Runtime build/dependency caches are ignored inside the isolated repo. ForgeFlow itself is never a contribution workspace. See [repository layout](docs/repository-layout.md).

## Verification
```powershell
go test ./...
go vet ./...
npm --prefix apps/web run build
# with both dev servers running in DEMO mode:
npm --prefix apps/web test
```
Browser tests use installed Edge on Windows. Set FORGEFLOW_TEST_URL to a disposable demo server URL and FORGEFLOW_LIVE_TEST_URL to a live server URL to run all eight checks without changing your main configuration. On Linux run `npx playwright install chromium` in apps/web first. Run browser tests against a disposable demo database if you want to keep normal demo config untouched; tests preserve history and create real config versions.
Browser tests include workflow/approval UI fixtures; live checks are opt-in. To include live checks against an already-scanned disposable live server:
```powershell
$env:FORGEFLOW_LIVE_TEST_URL='http://127.0.0.1:8081'
npm --prefix apps/web test
```
The live Run check performs read-only GitHub requests and appends scan/audit history.
Opt into actual authenticated Codex workflow QA on a disposable local Go repository:
```powershell
$env:FORGEFLOW_REAL_CODEX_TEST='1'
go test ./internal/execution -run '^TestRealCodexWorkflow$' -v -timeout 12m
Remove-Item Env:FORGEFLOW_REAL_CODEX_TEST
```
These tests do not execute the existing detent contribution or submit a remote PR.

## Approval rules
Human approval precedes contribution execution and PR submission. No automatic PR submission/merge. Config changes wait for Apply. Codex effort never influences canonical ranking.

## Structure
```
apps/server           Go HTTP entry point
apps/web              React application and browser tests
internal/api          REST, SSE, same-origin request checks
internal/analyzer     Evidence-derived quality factors and tree inspection
internal/discovery    Bounded scanning, cancellation and scheduling
internal/workspace    Approved external Git clone, snapshots and command audit
internal/codex        Finite CLI sessions and sandboxed command runner
internal/testing      Repository verification command detection
internal/execution    Coding, testing, fixes, independent review and PR gates
internal/github       Bounded REST discovery and approved fork/PR writes
internal/config       Default profile and validation
internal/contributions Persisted lifecycle validation
internal/domain       JSON contracts
internal/estimator    Independent heuristic scope estimate
internal/operator     Structured AI domain actions and proposal approval
internal/ranking      Explained quality scoring
internal/seed         Explicit demo data
internal/storage      SQLite transactions and migrations
scripts               Cross-platform startup and checks
docs                  Architecture, workflow, API, security and milestones
```

See [architecture](docs/architecture.md), [workflow](docs/workflow.md), [API](docs/api.md), [design](docs/design.md) and [milestones](docs/milestones.md).
See [verification](docs/verification.md) for actual check results and desktop/mobile screenshots.

