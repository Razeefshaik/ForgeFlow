# Execution recovery

ForgeFlow keeps source fixes separate from environment repairs. The control plane owns recovery; an agent must never weaken tests, change Git identity, access credentials, or cross the active workspace boundary to make a check pass.

## Implementation plan and architecture

1. Isolate Go temporary files as well as build/module caches. Validate native Go test filters as regular expressions instead of treating regex alternation as a shell pipeline. Keep batch wrapper protections.
2. Classify failures by evidence: code, transient transport/service, workspace runtime, dependency setup, authentication, tools, capacity, timeout, or execution boundary. Persist the diagnosis, next action, attempts, and scheduled retry time.
3. Use a shared bounded, cancellation-aware recovery budget for verification, GitHub freshness, Codex transport, and rejected planner output. Retry the original operation; preserve every failure. Repair malformed plans through the read-only planner before saving a valid plan or editing code.
4. Show automatic recovery and waiting for retries in the dashboard. For non-repairable setup failures, pause with **Needs input** and a concrete next action rather than invoking the code fixer. Keep genuine isolation/scope violations blocked.
5. Validate classification, retry budgets, cancellation, approvals, retained evidence, planning recovery, and actual Go execution; run the backend and frontend checks on this branch.

No new dependency is required. Incident fields are optional additions to existing JSON records; SQLite migrations and historical events remain unchanged. Retry scheduling runs only in an explicitly approved active execution. Restarting, pausing, or stopping never silently resumes agents.

## Policy

- At most six recovery actions per approved execution session; at most two retries per transient operation. Source fix/review limits remain independent.
- Exponential backoff records a visible retry timestamp. GitHub RetryAt takes precedence; a wait exceeding one minute requires a later explicit recovery instead of occupying an agent slot indefinitely.
- Downloads and dependency installation require existing network approval. Missing npm dependencies may use an existing lock via `npm ci --ignore-scripts`; no lock or partially installed dependencies require inspection.
- Temporary/cache paths are re-created and checked only inside `.forgeflow-runtime`. No ACL edits, cache deletion, Git resets, credential changes, service installation, or test downgrades are recovery actions.
- Authentication, absent tools/services, disk capacity, repeated transport failures, and timeouts require human input. Recovering after the prerequisite is restored performs fresh preflight and required verification.
- Invalid plans are saved separately as rejected evidence; only a validated plan can reach coding. Existing stopped planning runs can reuse a saved planner result if it now validates, while retaining the plan approval gate.
- Unknown failures remain explicit with saved evidence. No finite pipeline can safely repair every failure automatically.

## Development branch

Work is isolated in `feat/intelligent-recovery`. The original checkout, running server, live database, and contribution repositories are not modified by validation. Deploy the branch only after stopping the old server; never run two servers against the same SQLite database.

To use the built branch with this installation's existing database and contribution settings, stop the original `npm run dev` with Ctrl+C, then run from the original ForgeFlow folder:

```powershell
Set-Location '..\ForgeFlow-intelligent-recovery'
.\bin\forgeflow.exe --root '..\ForgeFlow' --web "$PWD\apps\web\dist"
```

Open `http://127.0.0.1:8080`. The root flag loads the original installation's `data/forgeflow.db` and local workspace settings; the frontend comes from this branch's build. This command was not used against the live database during validation. Use **Recover planning** for the existing rejected planner command, and **Recover verification** or **Retry after setup is restored** for an existing setup failure. These actions retain recorded download approval and do not submit a PR.

## Validation on 2026-10-11

- Changed Go files formatted; `go test ./... -count=1`, `go vet ./...`, and server build passed. Follow-up compatibility tests passed after adding legacy generic-incident recovery.
- An actual installed Codex sandbox ran a dependency-free Go module with an invalid inherited GOTMPDIR and native regex alternation. Both selected tests passed, the excluded failing test did not run, and the sandbox isolation probe passed. No model session was used for this check.
- Frontend dependencies installed with `npm ci`; TypeScript and Vite production build passed.
- Browser suite: 57 passed, two opt-in live-discovery tests skipped because no scanned disposable live server was supplied. All six recovery browser tests passed again against the final frontend build.
- Disposable demo and live servers returned healthy API and frontend responses. Automatic discovery was disabled; SQLite files, caches, and screenshots stayed in this worktree or test fixtures.
- Recovery simulations use explicit test runners for agent/API failure protocols. They validate control-plane behavior and approval gates; they do not establish that either real blocked contribution has passed tests or independent review.
