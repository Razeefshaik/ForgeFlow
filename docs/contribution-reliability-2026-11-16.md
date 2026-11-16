# Discovery and contribution reliability — 2026-11-16

Branch `claude/contribution-repair`. Nothing has been pushed, merged or submitted.

## Where things stood

No contribution had been started since the 2026-10-20 round. The only activity was two discovery scans that day: one PARTIAL and one FAILED, forty repositories screened, one issue offered. Discovery had become so strict that there was almost nothing to approve, and the one image every contribution needs had since disappeared from Docker.

## What was wrong, and what changed

| Problem | Evidence | Change |
|---|---|---|
| Discovery searched all open issues on GitHub by most recent update | The candidates were mostly days-old personal repositories | Search repositories that advertise good-first or help-wanted issues, ordered by stars, then read their open unassigned issues |
| Screening excluded on guesses | Of 61 saved assessments, 1 was eligible; the top reasons were a secret named in CI, more than 16 files to read, and a README over 48 KiB | Only an evidenced blocker excludes; uncertain signals are cautions shown at approval ([details](repository-eligibility.md#blockers-and-cautions-2026-11-16)) |
| Build bounds were measured on the slow bind mount | 10 CI minutes, 8 JVM build files and 6 Go modules excluded ordinary projects | 20 minutes, 25 and 12; integration workflows no longer size the build |
| A scan that found nothing was reported FAILED | The 2026-10-20 scan screened 20 repositories without any GitHub error that mattered | FAILED only when no search could be made; PARTIAL only when GitHub cut the scan short |
| "Build Syncthing" was ignored as a "sync" workflow | Its build was sized from a 13-second lint job | Workflow names match on whole words |
| Weak issues were offered | Good-first issues open since 2013, standing threads, template-labelled reports nobody had answered | Invited issues for three years, plain bugs for one; long discussions skipped; an uninvited report needs a second label or a maintainer's reply |
| The verification image was missing from Docker | `forgeflow-verification:go1.26-node22` was absent on 2026-11-16; ForgeFlow only warned, so every contribution would stop at its first check | ForgeFlow rebuilds its bootstrap image at startup and before a command needs it |
| The environment planner chose a base image without Git | A real fixture run: `python:3.11.2-slim-bullseye`, `git: not found`, and the one repair repeated it | Every managed image installs Git and tar itself; the planner is told to prefer the configured local image |
| A correct patch exhausted its fix budget | A real fixture run: the plan ran pytest in the virtual environment, and the detector added `python -m pytest` on the system Python, which has no pytest | A detected command the plan already runs through another path is dropped; a command that cannot start, identically on the base, is set aside with disclosure |
| A project's own rules closed the issue to outsiders | A real run on `cli/cli#14374` stopped before coding: the project accepts outside pull requests only for issues labelled `help wanted` with acceptance criteria | Label rules in contribution instructions are read during screening and applied to each issue at discovery and at approval |
| Ten-minute command deadline | It ended suites that were still passing (2026-10-17) | No command gets less than twenty minutes |

## Evidence

**Live discovery** (real GitHub, the saved sign-in, a consistent copy of the live database; the live database was not written):

- First probe of the new search and rules: 10 of 36 repositories eligible, including gitleaks, helm, authelia, testify, zaproxy, plantuml and manim.
- Four scans before the issue-quality rules: all SUCCEEDED; 105 repositories screened, 27 eligible; the ranked list went from 1 opportunity to 49.
- Three scans with the issue-quality rules: all SUCCEEDED in 3 to 5 minutes each; 95 repositories screened, 24 eligible; 28 issues offered, 23 in the ranked list, from projects such as caddy, scrapy, syncthing, apollo and docling. These scans ran before the contribution-policy rule, so the three `cli/cli` bugs they offered would no longer be offered.

**The verification image** was rebuilt by the new code path on this machine: missing, built in 21 minutes over a slow connection, and not rebuilt on the second call.

**Real pipeline runs on disposable fixtures** (`TestRealCodexWorkflow`: real Codex sessions, real Docker, isolated database), on the final code:

| Fixture | Before the fixes | After |
|---|---|---|
| Python (pytest, unrelated legacy failure) | BLOCKED twice: fix limit reached, then environment setup failed | READY in 194 s, 0 fixes, legacy failure disclosed |
| Gradle 8.10.2 / Java 17 | not rerun before | READY in 584 s, 0 fixes |
| Go | not rerun before | READY in 273 s, 0 fixes |

**One real upstream run** (`TestRealUpstreamWorkflow`, approved by the owner for `cli/cli#14374`): a fresh clone of cli/cli, real Codex, isolated database, no push and no pull request. It stopped after four minutes, before any coding, because the project does not accept outside pull requests for that issue. The stop was correct; offering the issue was the fault, now fixed.

## Not verified

- **No real upstream contribution has reached READY.** The one real run stopped for a policy reason before coding. A second run on an issue the new rules still offer (for example `caddyserver/caddy#6716`) was not authorised and has not been made. Until one completes, "works on real projects" rests on the fixtures.
- The contribution-policy rule is tested against cli/cli's wording. Projects that phrase such rules differently will be missed until seen.
- PR submission was not exercised (no remote writes by design). Some projects (caddy, for one) require AI assistance to be disclosed in the pull request; check the prepared text before submitting.
- Docker must be running. ForgeFlow rebuilds a missing image but does not start Docker.

## Running a real upstream check

```powershell
$env:FORGEFLOW_REAL_UPSTREAM='caddyserver/caddy#6716'
$env:FORGEFLOW_TEST_CONTAINER_IMAGE='forgeflow-verification:go1.26-node22'
$env:FORGEFLOW_TEST_SANDBOX='mxc'
$env:FORGEFLOW_TEST_CONTRIBUTIONS_ROOT='S:\StudyResource\TechBoooo\Backend\Contriss\_up'
$env:FORGEFLOW_TEST_CI_SECONDS='434'
$env:FORGEFLOW_TEST_EVIDENCE='S:\StudyResource\TechBoooo\Backend\Contriss\_up\evidence'
$env:FORGEFLOW_ROOT=(Get-Location).Path
go test ./internal/execution -run '^TestRealUpstreamWorkflow$' -v -count=1 -timeout 170m
```

It clones into a disposable folder, uses an isolated database, and never pushes or opens a pull request. It prints the status, every command and any incident, and writes `workflow.json` to the evidence folder.
