# Contributable repositories only — 2026-10-20

After the 2026-10-17 repair, two real contributions were resumed and both stopped again, each further than before. This round fixes why ForgeFlow kept choosing work it could not finish, and the execution problems those runs exposed. Branch `claude/contribution-repair`; nothing has been pushed, merged or submitted.

## What the 2026-10-17 runs showed

| Contribution | Reached | Stopped by | Underlying cause |
|---|---|---|---|
| Engineer Handbook #10 (Backstage SAM template) | setup with one automatic repair, plan, baseline, finished patch | `binary untracked file requires human inspection` | The native contributor ran its test with host Python, leaving `tests/__pycache__/*.cpython-313.pyc`. The repository is a scaffold template, and the issue is documentation. |
| osh-core #21 | setup, plan, baseline | `./gradlew test` killed at exactly 10 minutes, twice, with tests still passing | 10-minute default deadline. A Gradle wrapper "Downloading" line was misread as a network stall. 17 Gradle build files plus a GWT widgetset, all on the Windows bind mount. |

Both repositories should never have been offered: discovery screened only for forbidden requirements, never for whether ForgeFlow could build, verify and finish the work.

## Changes

**Discovery offers only contributable repositories** (see [repository eligibility](repository-eligibility.md#contributability-2026-10-20)):
- Build shape from the pinned tree: templates, heavy toolchains (GWT, Android, Bazel, Pants, Buck, Xcode), more than 8 JVM build files or 6 Go modules, large monorepos, no tests, no supported build manifest.
- GitHub Actions evidence: needs a successful recent verification run; persistently failing CI is excluded; a median longer than 10 minutes is excluded. The median is stored to size execution deadlines.
- Prerequisite reads skip release, publish and docs workflows, whose deployment secrets had held even Skript out of ranking.
- Issues: the search omits assigned and PR-linked issues; competing PRs, documentation-only issues (also by title, and even when labelled enhancement) and issues that already have a contribution are not offered.
- Each scan reports why issues were not offered. Signed-in scans may make 400 requests. The "public unauthenticated" label after a startup token refresh is fixed.

**Execution:**
- Verification and dependency setup run on a **synced copy of the workspace in a Docker volume**. Measured on osh-core, the identical clean compile took **2 min 15 s in a volume and 23 min 42 s on the bind mount**. Only changed files are sent, ignored build outputs persist, and nothing a command writes reaches the host clone.
- Generated caches (`__pycache__`, `*.pyc`, `.pytest_cache`, `node_modules` and similar) are excluded locally, Python bytecode writing is disabled, and a binary or oversized new file is listed in the patch instead of stopping it.
- Deadlines: Gradle and Maven get 30 minutes; with CI evidence, 4 × median + 10 minutes, capped at 2 hours. A timeout counts as a download stall only when downloads appear at the end of the output.
- A detected `test` run is skipped when a planned full `build`/`check`/`verify` already runs the tests.
- A finished command is no longer failed because Docker was slow to remove its container.

## Evidence

- **Real eligibility decisions against live GitHub:** osh-core excluded (GWT, 17 JVM build files) and the TechDocs template excluded (template, no build manifest), three requests each; Skript, the repository ForgeFlow previously completed, eligible; medici-finance/assay (25 Go modules), SysAdminDoc/HushGram (Android) and josejavierml03/api-tareas (no CI) were rejected, although the old discovery had ranked them 71–79.
- **Three real discovery scans** with the user's settings and saved GitHub sign-in, on a consistent copy of the live database (the live database was not written): 58 repositories screened, with exclusions including tinygo (71-minute CI), argo-cd (33-minute CI), openrewrite (36 JVM build files), Android apps, persistently failing CI, signing receipts and private credentials. The third scan accepted three issues from eligible repositories.
- **Real Docker tests** of the verification copy: host edits, deletions and additions synced; ignored build outputs and recipe files kept; stray outputs removed; Git works in the copy; no writes reach the host; 3,000 small writes in 10 s including sync and container start.
- **Real Codex end-to-end runs on the verification copy:** see "Final validation" below.

## Final validation

**Real Codex end-to-end runs on the verification copy** (`TestRealCodexWorkflow`): real Codex sessions, real Docker, an isolated database and disposable repositories under the contributions root, which are removed afterwards. The runs leave no containers, and no volumes other than the two belonging to the user's existing contributions.

| Fixture | Result | Notes |
|---|---|---|
| Python (pytest download, unrelated legacy failure) | READY in 506 s | venv in the runtime volume; legacy failure accepted as pre-existing, reviewed and disclosed; 4 sessions, 392,643 / 4,048 tokens observed |
| Gradle 8.10.2 / Java 17 (no wrapper) | READY in 711 s | the first generated recipe's image build failed, and the automatic correction produced a working recipe 4 minutes later; baseline `gradle test` 24 s with a 30-minute deadline; 5 sessions, 393,759 / 4,914 tokens |
| Go | READY in 199 s (rerun) | the first run paused: its readiness check passed, but Docker, saturated by a concurrent 23-minute benchmark, took over 30 s to remove the exited container, and that was reported as a failed prerequisite; fixed in `499d98e`, then the rerun passed; 4 sessions, 230,529 / 2,520 tokens |

**Suites at the final code** (`fa44955`, docs-only changes after it): `go vet ./...` and the server build passed; `go test -p 1 ./... -count=1 -timeout 45m` passed for all 16 packages (execution 555 s, codex 116 s); the frontend production build passed; the Playwright suite passed 70 tests, with 2 opt-in live checks skipped, against a fresh disposable demo server and database. Earlier browser runs on a reused demo database, while Docker was saturated, showed three timing/state flakes that passed on rerun.

## Recommendations for the existing contributions

- **Engineer Handbook #10** and **osh-core #21** are now classified as not contributable (a scaffold template with a documentation issue; a 17-module Gradle build with a GWT widgetset). Abandon both in the app. Their workspaces, evidence and history are kept, and abandoning removes their two runtime volumes.
- Run **discovery** after starting ForgeFlow. Saved repositories are reassessed under the new profile, and the ranked list shows only repositories with evidence that they can be built and verified. Expect a short list at first; it grows with each 15-minute scan.
- Optional: in Configuration, a minimum of about 50 stars removes many personal repositories that rarely merge outside PRs. This is a configuration choice and was not changed.

## Not verified

- No new real contribution was started on an upstream repository (that needs your approval), so a full upstream run from the new ranked list to READY has not been observed yet. The three disposable end-to-end runs exercised the same pipeline.
- PR submission was not exercised (no remote writes by design).
- Node/npm repositories have no real end-to-end fixture; `node_modules` now lives in the fast verification copy, but that path is covered only by the copy's unit and Docker tests.
