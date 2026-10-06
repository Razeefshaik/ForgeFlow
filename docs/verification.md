# Verification — 2026-10-06

## Current discovery verification
- `gofmt`, `go test ./...`: all 29 Go tests passed across ten tested packages.
- Root `npm run build`: Go tests, vet, binary build, TypeScript and Vite completed with exit 0.
- Production bundles: main JS 389.36 kB, opportunities 126.61 kB, CSS 28.09 kB; no size warning.
- With `FORGEFLOW_LIVE_TEST_URL=http://127.0.0.1:8081`, all six browser checks passed against actual Go servers and installed Edge in 44.6 seconds. The default suite runs four demo checks and skips two opt-in live checks.
- New tests cover caching/ETag, credential isolation/precedence, budgets, redirects/size bounds, rate-limit backoff/error redaction, incomplete evidence, quality factors, single-flight cancellation, retained scan history, interruption recovery and durable scheduling controls.
- Live browser checks inspect real GitHub evidence and run another scan while execution stays disabled. Desktop/mobile artifacts were visually inspected, with no document overflow at 390px. Captures wait for completed state and disable animations.
- Git whitespace validation passed. All changes remain inside the active contribution repository.
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
- Original SPECS.md and AGENTS.md hashes match their copies here.

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
Isolated external workspaces, enforceable command sandboxing, Codex execution, real contribution testing/fixing, independent review, AI Operator, final reports and PR preparation remain future milestones. The full acceptance scenario has not been achieved.

