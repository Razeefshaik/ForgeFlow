# Contribution pipeline repair — 2026-10-17

Branch `claude/contribution-repair`, based on `73f467a`. Every contribution started after the Docker environment pipeline was introduced stopped before any code was written. The one contribution that reached a PR (Skript #8383, 2026-10-07) predates that pipeline. This document records why, what changed, and what was actually verified. Historical reports were treated as claims; every result below was produced in this session.

## Starting state

- `master` = `origin/master` = `73f467a`; only the handoff note was untracked. ForgeFlow was not running.
- Go 1.26.6, Node 24.13.0, Git 2.47.0, Codex CLI 0.162.1 (ChatGPT login), Docker Desktop engine 29.7.2 (8 CPUs, 7.2 GB).
- `configs/local.json` pointed `verification_image` at `sha256:b7fad75…`, **which no longer existed in Docker**.
- Live database (read-only inspection): osh-core #21 and Engineer Handbook #10 both `PAUSED`/`NEEDS_INPUT`, incident `environment_blocked`, phase `PLANNING`, no plan, no edits.

## Root causes, with evidence

| # | Severity | Cause | Evidence |
|---|---|---|---|
| 1 | Critical | Configured verification image deleted. Containers use `--pull=never`, so every contribution would fail its first preflight with a generic "container creation failed". | `docker image inspect sha256:b7fad75…` → `No such image`. |
| 2 | Critical | Dependency installs ran on the Windows bind mount, ~100× slower for small files on Docker Desktop. | Measured: 3,000 file writes took 26 s on the bind mount and 0.24 s on a volume. TechDocs' pip install downloaded everything in seconds, then sat in "Installing collected packages" until the 15-minute budget expired (14 m 43 s). |
| 3 | High | The single automatic recipe correction never ran after a resume. `prepareEnvironment` used one `force` flag both for "replan because the saved environment is blocked" and for "this call is already the repair". | osh-core: the resumed recipe's Gradle init script failed with `Could not get unknown property 'allprojects'`, and the contribution paused with no correction attempt. A regression test reproduces it and fails on `73f467a` with the same saved message. |
| 4 | High | Provisioning timeouts were saved as a bare `context deadline exceeded`. | TechDocs incident evidence was exactly those 25 characters. |
| 5 | High | Any failure that already existed upstream (flaky or environment-specific tests, lint debt, suites needing absent services) could never pass, so the contributor looped until the fix limit. | Detected commands are always required; nothing compared post-change failures with the base. |
| 6 | High | Dead ends: an issue edited after approval blocked every resume; a contribution at the fix/review limit hit it again on every resume; resuming silently revoked download approval (the box always started unchecked). | Code inspection, reproduced by regression tests. The download checkbox explains the "blocked offline, then resumed with downloads" pattern in both stalled records. |
| 7 | Medium | Workspace preparation (full clone) had a 5-minute deadline; a timed-out preparation cannot be resumed. | The 10-minute pull of a 300 MB image on this connection shows the clone budget was unrealistic for medium repositories. |
| 8 | Medium | Container limits (256 tasks, 2 GB, 2 CPUs) were exhausted by ordinary JVM builds (threads count as tasks), and eligibility screening hardcoded the same limits. | `--pids-limit 256` in container arguments. |
| 9 | Low | The environment-planner prompt said HOME was `/tmp`; the container sets a workspace path. | Text mismatch noted in the handoff, confirmed. |

## Changes (one commit each)

1. Per-workspace Docker volume for container HOME, caches and dependency environments at `/workspace/.forgeflow-runtime/linux`: initialized offline with only `CAP_CHOWN`, never shared, removed when a contribution is abandoned or a probe workspace is disposed, and never pruned. Container limits are now 4096 tasks, 4 GiB, 4 CPUs and a 2 GiB `/tmp`; eligibility screening reads the same constants.
2. Provisioning errors name the command, exit code or duration, and include head/tail output. The preparation budget is now 30 minutes.
3. Environment repair logic: a resumed blocked environment still gets one evidence-driven correction. Infrastructure or permission failures rerun the saved recipe without a model call. The planner sees the saved failing command and output (including for older records), the correct HOME, the fast volume location, and guidance to prefer repository dependency commands over generated scripts.
4. The run backstop is raised from 2 to 6 hours; per-agent and per-command deadlines are unchanged.
5. Missing image: preflight, inventory and `npm run dev` give the exact rebuild command. `configs/local.json` now uses the stable tag `forgeflow-verification:go1.26-node22` (rebuilt from `scripts/execution-image`). The previous value is kept in the session scratchpad.
6. Baseline verification: the required commands run once on the untouched base before coding. A post-change failure is accepted as pre-existing only with proof:
   - its failing tests are a subset of the base's (parsed for Go, pytest, unittest, Gradle, Surefire, Jest, Cargo and TAP), or
   - no tests were collected in either run, or
   - for unparsed lint-style output, every error line already appeared and neither run shows a build failure.

   Acceptance is never allowed for a failure named by the issue or diff, or located where the patch edits. Accepted failures keep their exit codes and are disclosed to the reviewer, in the report, in the PR text and in the Tests tab.
7. Issue edited after approval: the edit is saved and shown; continuing requires an explicit "Approve updated issue", which invalidates the plan. The original approval record is never rewritten.
8. Fix/review limit: an explicit human resume grants one fresh bounded budget (audited as `ExecutionBudgetRenewed`).
9. The resume panel starts from the saved download approval, still visible and changeable.
10. Clone/preparation deadline raised from 5 to 30 minutes.
11. `npm run dev` checks Docker and the verification image before starting. `npm run build` gives `go test` 30 minutes.
12. The real-Codex workflow test runs on Go, Python and Gradle fixtures. Recovery pipeline reported as version 10.
13. Baseline relatedness locates JVM tests from their source package (Gradle prints simple class names, as the Gradle run showed: `junit:ArithTest > addsPositiveNumbers`), unittest module ids and Jest `FAIL <file>` lines.
14. Read-only diagnosis (version 7) of older environment incidents states what recovery will now do (rerun or corrected recipe) and offers **Recover environment**, without rewriting saved incidents.

## Validation

### Verified (this session, real commands)

- **Real end-to-end contributions** (`TestRealCodexWorkflow`), each with real Codex sessions (environment planner, planner, contributor, independent reviewer), real Docker verification, an isolated SQLite database and a disposable repository:
  - Go: READY in 220 s; baseline found the issue's own failing test, which was then fixed (not accepted); 392,161 input / 3,519 output tokens observed.
  - Python: READY in 220 s; the planner placed the venv in the runtime volume (pytest install: 2 s); an unrelated legacy failing test was accepted as pre-existing, challenged and approved by the reviewer, and disclosed in the PR text; 358,798 / 4,067 tokens.
  - Gradle/Java 17 (no wrapper): READY in 476 s; JDK and Gradle 8.10.2 provisioned, dependency preparation 15 s; 289,012 / 3,535 tokens.
  - After each run: zero ForgeFlow containers and zero runtime volumes left; the disposable fixtures were removed.
- Real Docker tests (no model): workspace write, root/Git write denial, UID 1000, offline networking, absent Docker socket; HOME/cache persistence across fresh containers; per-workspace isolation; 3,000 small writes in 2.2–5 s including container startup; missing-image diagnosis.
- Regression tests reproduce the osh-core repair bug and the infrastructure-replan waste; both fail on `73f467a` and pass now.
- Browser tests run only against a disposable demo server on port 18082 with its own database; the live database was never used. New tests cover the issue-update approval, the preserved download approval, baseline labelling and recovery of older environment incidents.
- Frontend production build (`tsc -b && vite build`) and `go vet ./...` pass.
- The current backend started against a consistent SQLite backup of the live database (port 18083, discovery disabled). It reported live mode, recovery pipeline 10 and the configured image tag, and served both stalled contributions. Their diagnosis now offers **Recover environment** with the corrected explanation (screenshot taken from that copy). The live database file was not modified.
- Full backend suite, one clean command each (`go test -p 1 ./... -count=1 -timeout 45m`): this branch, 16 of 16 packages passed (execution 310 s; baseline runs roughly double that package's real Git/Go fixture work); original `73f467a` in a separate worktree, 16 of 16 passed (execution 143 s). At the final commit, `go vet ./...`, the server build and the execution, api and codex packages passed again.
- Browser suite at the final commit against the disposable demo server: 69 passed, 2 opt-in live checks skipped.

### Failing or corrected during validation

- `interface.spec.ts` "running state…" failed on the existing code: it predates the Environment tab (added 2026-10-12) and expected Tests directly after Plan. The test now walks the real tab order.
- The first full backend run in this session was contaminated (the execution package compiled after edits began) and hit Go's default 10-minute limit while a Docker image build loaded the machine. `TestGoUsesWorkspaceTemporaryDirectoryAndNativeFilter` failed once under that load; it passes in isolation on both the original commit (27 s) and this branch (22 s). That run's log is retained.

### Unverified

- **osh-core #21 and Engineer Handbook #10 were not resumed.** Execution requires your approval in the app. Their saved failures map directly to causes 2–4 and 6 above, but whether their full repository suites pass is unknown until they run.
- Praetor #299 (signed-receipt completion gate, verification timeouts) and Bitbucket CLI #777 (external services) were left untouched, as deferred.
- PR preparation and submission to GitHub were not exercised (no remote writes by design).
- Node/npm repositories: `node_modules` still lives on the bind mount (slow for very large installs). Only Go, Python and Gradle were proven end to end.
- Native agent shells are limited by Windows' 260-character path limit when workspaces are nested deeply. Agents recovered via `\\?\` paths in testing. Enabling Windows long paths (an administrator setting) removes this; workspaces under `Contriss` are short enough in normal use.

## Recovering the two stalled contributions

1. `npm run dev` (Docker Desktop running). The startup line should show `Container verification image: forgeflow-verification:go1.26-node22` and `recovery pipeline v10`.
2. Open each contribution. If the issue was edited upstream, review and approve the update first.
3. Use **Retry after setup is restored** (or **Resume execution**; download approval is now pre-selected from the saved approval). The planner receives the saved failure (the Gradle init-script error for osh-core; the stalled pip install for TechDocs) and plans a corrected recipe; dependency environments now go to the fast volume.
4. The required suite then runs once on the untouched base before any coding, so an environment that cannot run the suite stops before Codex edits code.

No PR is submitted or merged automatically; READY still requires your review, and PR submission its separate approval.
