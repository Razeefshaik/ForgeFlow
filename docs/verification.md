# Verification - 2026-10-06

## Current execution delivery

- Contribution parent moved to S:/StudyResource/TechBoooo/Backend/Contriss. All 4020 pre-move SHA256 file hashes matched after relocation. Main contributions/ contains only .gitkeep. Source, branch, Git history, snapshots and artifacts were preserved.
- Go formatting, full Go tests and vet passed. Production Go binary and TypeScript/Vite builds passed. Final affected checks and startup verification are recorded below.
- Real authenticated native Codex CLI workflow on a disposable local Go repository passed in 429.31 seconds: read-only planner, contributor correction and positive/negative regression coverage, three actual sandboxed verification commands, independent reviewer APPROVE and persisted READY. Zero fixes were required for that actual model run. No existing external contribution was coded by QA.
- Synthetic model fixtures with actual Git/Go commands exercised failing tests, bounded fixes, reviewer REQUEST_CHANGES, retesting/review, local commit preparation and separate submission approval. Diff evidence remained identical before/after committing new files. Repeated failures stopped at configured bounds.
- Pause cancelled the active task while preserving workspace/evidence; unconfirmed resume/constraint/plan changes were rejected. Explicit plan approval made the saved plan resumable.
- Actual authenticated AI Operator produced an expiring Rust/databases/distributed-systems proposal in 18.57 seconds. It changed no configuration before Apply. A prior malformed proposal was rejected; the input contract now supplies the plain configuration separately from its version.
- Temporary-config integration tests verified restoration as a new version and protection of newer manual settings.
- Edge/Playwright final suite: all eight checks passed in 21.1 seconds against a disposable production demo server and the live development server. This includes both opt-in real GitHub checks. Execution/approval UI fixtures are synthetic, not evidence of real remote PR submission. Desktop/mobile captures: execution-desktop.png and execution-mobile.png; 390px layout had no document overflow and was visually inspected.
- GitHub write tests use a synthetic HTTP server and verify authentication/endpoint allowlists. No real fork/push/PR was created during QA. Remote submission remains gated by explicit user approval and runtime GitHub credentials.
- Native sandbox positive/negative write probe passed. The profile permits host reads; it is a workspace write boundary rather than a confidentiality VM. See security.md.

## Final startup and preservation checks

- The production server serves HTTP 200 at / and /contributions. A regression test covers root/nested SPA routes, assets and missing assets. The root route previously returned 404 and was corrected.
- Live /health returns mode live; /runtime resolves S:/StudyResource/TechBoooo/Backend/Contriss and the installed native Codex CLI. Discovery succeeded with 13 persisted opportunities across 10 repositories. These are a bounded sample, not a complete GitHub catalog.
- Existing detent contribution remains BLOCKED at PLANNING awaiting explicit Start coding approval. WorkspaceRelocated event 79 updates its current location; immutable historical approvals retain their original paths. Branch remains autopilot/issue-4372 and HEAD remains 3e0e0aa9c255ae96d1835f8d45d7cec18dd0fa22. External Git status is clean.
- Before schema updates, the live SQLite files were copied under ignored .cache/before-execution-20261006-214759. SQLite migration history and audit records remain intact.
- Git operations pin validated local configuration outside the agent write root and reject subsequent changes or linked Git metadata; regression tests cover rejection and resumable plan/pause gates. Git whitespace validation passed. Main checkout remains master; changes are uncommitted and no remote push occurred.
- npm run build completed the final Go tests/vet/binary and frontend checks. The live app is started through npm run dev; disposable QA servers are stopped.

## Earlier delivery evidence (historical)

The following records describe earlier phases. Their phase-specific limitations and counts are historical, not the current implementation status.

# Verification — 2026-10-06

## Current discovery verification
- `gofmt`, `go test ./...`: all 29 Go tests passed across ten tested packages.
- Root `npm run build`: Go tests, vet, binary build, TypeScript and Vite completed with exit 0.
- Production bundles: main JS 389.36 kB, opportunities 126.61 kB, CSS 28.09 kB; no size warning.
- With `FORGEFLOW_LIVE_TEST_URL=http://127.0.0.1:8081`, all six browser checks passed against actual Go servers and installed Edge in 44.6 seconds. The default suite runs four demo checks and skips two opt-in live checks.
- New tests cover caching/ETag, credential isolation/precedence, budgets, redirects/size bounds, rate-limit backoff/error redaction, incomplete evidence, quality factors, single-flight cancellation, retained scan history, interruption recovery and durable scheduling controls.
- Live browser checks inspect real GitHub evidence and run another scan while execution stays disabled. Desktop/mobile artifacts were visually inspected, with no document overflow at 390px. Captures wait for completed state and disable animations.
- Git whitespace validation passed. This describes the earlier discovery delivery; see repository-layout.md for subsequent relocation verification.
- Actual server smoke check: pause persisted through a process restart, resume succeeded, and scan history remained intact. Temporary QA servers were stopped after verification.

The initial unauthenticated live scan accepted three real issues across two repositories using 22 GitHub requests. A repeat scan succeeded with zero network requests from cached responses. After rebuilding/restarting, the final browser Run check accepted two issues with 18 requests; five accumulated observations remained visible. Bounded searches preserve older observations rather than deleting unseen issues.

QA databases `data/qa-demo.db` and `data/qa-live.db` retain config versions, scan records and audit events. Test upstreams exercise failure cases without claiming real GitHub failures. No external repository clone, contribution agent/test/review or PR submission occurred.

## Foundation verification history
- gofmt -w apps/server internal
- go test ./... — all 15 tests across seven tested packages passed.
- go vet ./... — exit 0.
- go build -o bin/forgeflow.exe ./apps/server — exit 0.
- npm run build from the repository root — startup/build orchestration script completed all Go checks and the frontend build with exit 0.
- Prettier formatted frontend source, configuration, browser tests and capture script.
- npm --prefix apps/web run build — TypeScript and Vite passed. Main JS 385.71 kB, opportunities chunk 122.69 kB; no bundle-size warning after page splitting.
- npm --prefix apps/web test — 3 browser tests passed against actual Go APIs and installed Edge. Covers filtering/canonical rank, score details, disabled execution gate, Operator proposal waiting for Apply, actual configuration version creation, mobile navigation and light mode.
- node apps/web/scripts/capture.mjs — four screenshots, no page errors and no document overflow at 390px.
- Actual live server at 127.0.0.1:8081 returned mode=live, zero opportunities, zero contributions; /configuration served the built SPA with HTTP 200.
- At initial delivery, supplied SPECS.md/AGENTS.md hashes matched their application copies. The specification has since been normalized to root SPEC.md and application layout guidance added to AGENTS.md.

Tests ran against temporary SQLite files and separate QA demo/live databases. Browser tests preserve audit history and append configuration versions. No application contribution agent, external repository test or PR submission was run.

## Visual artifacts
[Live discovery desktop](../artifacts/discovery-desktop.png)
[GitHub evidence](../artifacts/discovery-evidence-desktop.png)
[Live discovery mobile](../artifacts/discovery-mobile.png)
[Desktop overview](../artifacts/overview-desktop.png)
[Opportunities](../artifacts/opportunities-desktop.png)
[Configuration](../artifacts/configuration-desktop.png)
[Mobile overview](../artifacts/overview-mobile.png)

Desktop and mobile renders were visually inspected. The mobile opportunity table scrolls internally rather than overflowing the page. Artifacts are local generated files, intentionally excluded from Git.

## Environment and dependency recovery
Windows, Go 1.26.6, Node 24.13.0, npm 11.6.2, Playwright 1.63.0 and installed Microsoft Edge.
Initial npm attempts encountered incomplete downloads and stale cached registry metadata (ETARGET). Published versions were checked directly against the registry. A fresh workspace-local cache completed installation; dependencies and package-lock.json pin a consistent set. The incomplete dependency tree remains in .cache/incomplete-node_modules, outside tracked source. No global tool upgrade was performed.

## Delivery boundary
Phase 1 foundation and public discovery in Phase 2 are implemented, with bounded analysis and memory HTTP caching. Deterministic Operator inspection/proposals are included.
Approved isolated Git workspace preparation is implemented. Enforceable repository command sandboxing, Codex coding, real contribution testing/fixing, independent review, AI Operator, final reports and PR preparation remain future milestones. The full acceptance scenario has not been achieved.

## Workspace preparation verification (2026-10-06)

- Go formatting, all 36 Go tests, go vet and Go build passed. Five workspace tests include actual Git clone/branch/commit verification, a full approved preparation pipeline using a synthetic GitHub server and real local Git, stale/closed inputs, exclusive/link boundaries, real failed command outcomes, duplicate approval and restart recovery.
- Frontend TypeScript/production build passed. Five default browser checks passed; two opt-in live checks were skipped in that run. The live evidence browser check then passed separately against the running live server. The approval browser check uses a clearly labeled API fixture, not a live clone.
- Actual npm run dev started live backend at 127.0.0.1:8080 and frontend at 127.0.0.1:5173 (HTTP 200). Automatic public GitHub discovery made 18 requests and accepted two observations. The run was PARTIAL because a separate response exceeded the four MiB bound; its warning remains persisted.
- A real fresh approval preview succeeded for digitaldrywood/detent issue 4372 at commit 3e0e0aa9c255ae96d1835f8d45d7cec18dd0fa22. Preview created zero contributions. No approval was submitted for an external repository during QA; root contributions still contains only .gitkeep.
- Approval desktop/mobile screenshots were generated and visually inspected. Git diff --check passed. Root Git remains the application repository; no external clone or nested application Git repository was created during QA. Changes remain uncommitted.

The live development server is left running for interactive use. The disposable demo QA server was stopped. Real repository cloning requires the explicit in-app approval, and prepared contributions truthfully stop BLOCKED at PLANNING until the Codex adapter exists.


GitHub account verification: synthetic OAuth HTTP tests exercise device polling, pending/slow-down/denial, identity verification, Windows DPAPI encryption, restart recovery, refresh rotation, and logout. Browser fixtures exercise OAuth app setup, the GitHub challenge link/code, connected account after reload, and sign-out. No real GitHub account authorization or PR submission was performed by these tests.
