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

## Running the updated checkout

The initial recovery work was merged into master. Follow-up fixes are on
`fix/recovery-diagnostics` in the main ForgeFlow folder. Starting an old branch in
another worktree still starts old application code. Development startup now prints
the checkout, revision, and recovery version; `/api/health` and `/api/runtime` expose
the executable's build identity. Stop the existing dev command before starting:

```powershell
npm run dev
```

Open `http://127.0.0.1:5173`. Use **Recover planning** for rejected planner commands,
**Inspect path and retry** for generated directory access failures, and **Retry after
setup is restored** for external prerequisites. Existing approval gates still apply.
See [observed Windows failure recovery](recovery-diagnostics-plan.md) for the follow-up
architecture and validation. No migration rewrites historical diagnoses; stopped
executions can display a newer diagnosis derived from their original test record.

## Additional recovery rules

- A denied Go package-discovery path is eligible for quarantine only when it is an
  empty, ignored, untracked directory below a recognized output root, without links.
  The original directory and ACL are preserved under `.forgeflow-runtime/quarantine-*`,
  with source/destination audit events. Required commands and package scope are unchanged.
- Known Git shell failures trigger short capability probes before another build,
  sharing the normal recovery budget. Probe results never count as repository tests.
- Persist runner errors alongside partial output, including deadline cancellation.
  Report missing CGO/C compiler prerequisites without removing race checks.
- No issue-related diff after verification needs a scope decision, not repeated
  code fixing or an empty PR. A successful build alone does not prove the issue solved.
- Explicit Codex binary settings are honored. Default Windows resolution compares
  the npm/PATH binary with existing Codex app-server releases, without downloading or
  installing software. Compatible CLIs prefer MXC; the isolation probe remains mandatory.

## Validation on 2026-10-11

- Changed Go files formatted; `go test ./... -count=1`, `go vet ./...`, and server build passed. Follow-up compatibility tests passed after adding legacy generic-incident recovery.
- An actual installed Codex sandbox ran a dependency-free Go module with an invalid inherited GOTMPDIR and native regex alternation. Both selected tests passed, the excluded failing test did not run, and the sandbox isolation probe passed. No model session was used for this check.
- Frontend dependencies installed with `npm ci`; TypeScript and Vite production build passed.
- Browser suite: 57 passed, two opt-in live-discovery tests skipped because no scanned disposable live server was supplied. All six recovery browser tests passed again against the final frontend build.
- Disposable demo and live servers returned healthy API and frontend responses. Automatic discovery was disabled; SQLite files, caches, and screenshots stayed in this worktree or test fixtures.
- Recovery simulations use explicit test runners for agent/API failure protocols. They validate control-plane behavior and approval gates; they do not establish that either real blocked contribution has passed tests or independent review.
