# Managed recovery validation — 2026-10-11

The initial implementation was validated on `feat/managed-execution-recovery` and merged into `master` at `4d80cde`. The checks below record that implementation; the version 4 launcher correction has additional validation at the end of this report.

## Automated validation

- `go test ./... -count=1`: all packages passed.
- Targeted execution tests after the final recovery changes: passed, including focused assertion reproduction, complete-suite retry, exhausted recovery, dual failure evidence and prevention of source fixes for sandbox failures.
- Final Codex/API tests: passed, including explicit approval, mode allowlist, failed/declined setup, concurrent operations, cancellation, failed audit, incomplete probes, configuration precedence, corrupt managed settings and bounded output retaining final errors.
- `go vet ./...`: passed.
- `go build -o bin/forgeflow-recovery-final.exe ./apps/server`: passed.
- `npm --prefix apps/web run build`: TypeScript and Vite passed.
- Browser suite: **61 passed, 2 skipped**. Skips require opt-in live GitHub discovery. The environment and execution browser tests were rerun after subsequent changes and passed.
- Desktop/mobile setup controls were inspected, including approval, disabled/running states, saved check output and resume navigation. The design hook's existing gradient-text finding was fixed with the established accent color; no findings were suppressed.

## Installed Codex CLI and isolated application checks

No model sessions, administrator setup, contribution edits or PR submissions were started during validation.

- Actual sandboxed Go fixture: regex alternation selected the intended two tests; excluded test did not run. Workspace-local runtime paths and negative write probes passed.
- Actual sandboxed failed assertion followed by a hanging test: retained the assertion, `panic: test timed out after 250ms`, and real Go stack traces. The failed command remained failed.
- Approved public dependency endpoint returned HTTP 200. The same network-denied policy reported ENOTFOUND.
- A disposable environment check on the configured contribution drive passed workspace writes and denied writes to sibling/control-plane paths. Git shell startup then failed with `NtCreateDirectoryObject(\\BaseNamedObjects\\msys-2.0S5-1888ae32e00d56aa): 0xC0000022`. That mode was not marked ready or saved.
- An isolated SQLite copy of the live database exposed the new runtime endpoints and recovery version 3. Saved Praetor evidence now reports `test_failure_timeout` with both assertion and timeout causes and the named failing test. Bitbucket evidence reports `sandbox_process`.

## Remaining machine-specific verification

The elevated Windows setup flow is implemented and its failure/approval paths are tested with controlled adapters. Real elevated provisioning and Git compatibility still require the user to approve the Windows elevation prompt through Settings. Neither contribution was resumed during validation. The already-running live backend needs a restart to load these changes.

Missing credentials, platform-specific services and genuine isolation violations remain explicit intervention points. Focused diagnostic passes never substitute for complete verification and independent review; no unrestricted execution fallback is offered.

## Interactive setup launcher correction — pipeline version 4

A real setup attempt in version 3 failed with `orchestrator_elevation_required`. The CLI deployment command requires an already elevated process. The corrected launcher uses the supported app-server `windowsSandbox/setupStart` method for interactive provisioning, waits for the corresponding completion notification, then runs the existing isolation and Git-shell probes before selecting the mode.

- `go test ./internal/codex ./internal/project ./internal/api -count=1`: passed. New protocol/process regressions cover successful completion, completion arriving before acknowledgement, acknowledgement without completion, declined UAC, contradictory success/error, unsupported RPC, initialization failure, malformed/oversized events, unexpected tool approvals, process startup failure and cancellation.
- `go vet ./internal/codex ./internal/project ./internal/api`: passed.
- `go build -o bin/forgeflow.exe ./apps/server`: passed.
- Installed Codex CLI `0.162.1`: the exact app-server launch arguments initialized successfully. Its exported schema includes the setup request/completion types, and a request with a deliberately invalid mode was rejected by the real setup method's enum validation. This capability check did not start provisioning or a model session.
- The running version 3 backend was left in place while Praetor was executing tests. A backend restart is required to load version 4. Actual UAC approval, successful provisioning and Git compatibility remain machine-specific checks to perform through the corrected Settings action; automated fixtures do not establish those results.

## Runtime file-lock diagnosis — pipeline version 5

Real app-server setup attempts reached native permission refresh and failed with Windows error 32. ForgeFlow's bounded current-attempt diagnostic identified `node_repl.exe` rather than presenting only `helper_unknown_error: setup refresh had errors`. The two identified Node helpers were stopped after explicit approval. The next attempt identified `VCRUNTIME140_1.dll`, loaded by Codex's desktop-control helper; the desktop app restarted that helper immediately after it was stopped. The setup refresh therefore remains incomplete while those Codex clients are open.

Verified selection of the installed elevated mode also triggered the same native refresh failure. Diagnostics now cover this verification path, retain the failed check and original cause, report `needs_input`, and do not select or save an unverified mode. The regression uses a fresh public helper-log fixture, a failed isolation check and assertions against accidental mode adoption.

An actual explicit MXC selection passed offline workspace writes and denied writes to the sibling and control-plane paths, then failed Git shell startup with `NtCreateDirectoryObject(\\BaseNamedObjects\\msys-2.0S5-1888ae32e00d56aa): 0xC0000022`. MXC was not saved. No contribution was resumed and no model session or PR publication was started. The next native setup attempt must run through ForgeFlow in an ordinary browser after the Codex desktop app and affected VS Code sessions are closed; stopping automatically restarted helpers repeatedly is not a recovery.

- `gofmt` and `git diff --check`: passed for the changed files.
- `npm run build`: passed, including every Go package's tests, `go vet ./...`, the server binary build, frontend TypeScript and Vite production build. The execution package's existing real-command fixtures took approximately five minutes. An extra verbose diagnostic run was stopped once it confirmed progress; the original complete suite passed.
- A running version 5 server exposed the configured external contributions root and the current build revision through `/api/runtime`. Native elevated verification is still refused until the runtime file lock is released; automated regression success does not establish a working native sandbox.

## Native editing with container verification — 2026-10-12, pipeline version 6

The original five uncommitted files were preserved. The initial patch is retained locally at `.cache/windows-execution-validation/pre-existing.patch`; no existing contribution or live database record was resumed or replaced.

### Independently reproduced causes

The bounded disposable host comparison in `scripts/probe-windows-execution.mjs` used installed Codex 0.162.1 and Git 2.47.0. Host Git shell exited 0. Explicit MXC exited 66 with `NtCreateDirectoryObject(\\BaseNamedObjects\\msys-2.0S5-1888ae32e00d56aa): 0xC0000022`; explicit unelevated mode exited 66 with `couldn't create signal pipe, Win32 error 5`. Elevated mode failed before Git startup. Its fresh helper log at `2026-10-11T18:08:07Z` reported error 32 opening `node_repl.exe` for runtime ACL validation. The desktop helper processes were independently observed running, and the desktop-control process had the reported `VCRUNTIME140_1.dll` loaded. Disabling configured desktop integration settings did not fix the refresh. No helper was killed or global ACL changed during this investigation.

Docker Desktop was installed but stopped. It started successfully. Ubuntu WSL existed but had no Linux Go or Node executable available on its command path. The selected implementation therefore keeps native MXC editing/authentication and adds optional, explicit Docker verification; it does not migrate ForgeFlow. Native Git/MSYS compatibility remains unresolved. See [container verification](container-execution.md).

### Live disposable proof

`TestRealContainerVerification` passed in **141.14 seconds** using the real native CLI and `forgeflow-verification:go1.26-node22`. Local evidence: `.cache/windows-execution-validation/proof-2026-10-11T20-53-39.870Z/container-validation.jsonl` and `boundary-edit-test-cancel.log`.

- Native MXC allowed writes in the active repository and denied sibling/control-plane writes. Container checks proved UID 1000, allowed workspace writes, denied root and `.git` writes, an absent Docker socket, and loopback as the only offline interface.
- The baseline `go test -v ./...` really failed with `addition incorrect`, while its Git-shell test passed.
- An authenticated native Codex session changed `sum.go` from subtraction to addition and added negative-number coverage in `sum_test.go`. The session ID and actual event stream were retained. No container received authentication files.
- The same complete Go suite then passed: `TestAdd`, `TestAddNegativeNumbers`, and `TestGitShell`. Both original and executed command arguments, times, working directory, output, exit status, image and container name are recorded.
- A real Node process spawned a delayed child. Cancellation was exercised only after the readiness file appeared. The runner returned cancellation, removed the container, and the child's delayed write did not occur.
- The exact disposable `.forgeflow-validation-228757124` directory was resolved, checked against the configured Contriss root, and removed. JSONL logs and edited-file contents remain outside the deleted fixture.

Two earlier failed proof attempts are also retained. First-time image unpacking delayed Docker create beyond the initial 30-second deadline; a minimal unmounted container subsequently ran successfully, and the late-created container was explicitly removed. The adapter now retains context timeout errors and allows bounded two-minute creation. The next attempt exposed Docker tmpfs's non-executable default: Go compiled but could not execute its test binary. Adding `exec` to the private `/tmp` tmpfs fixed that exact failure; it did not change host write access.

Focused container-boundary, explicit-configuration and infrastructure-classification regressions passed. `go test ./... -count=1` passed every package (the execution package took 306.514 seconds). `go vet ./...` and `go build -o bin/forgeflow-container.exe ./apps/server` passed. No PR was submitted or merged.

### Persisted live workflow

`TestRealCodexWorkflow` passed in **188.04 seconds** and reached **READY**, using distinct authenticated planner, contributor and reviewer sessions. The actual execution recorded two verification commands, zero fix iterations and one review cycle. `workflow.json` retains the execution record, test runs, agent records and audit events; `persisted-workflow.log` records the passing test. Both are in the same successful proof directory above. This used an isolated SQLite database and disposable external repository, not the live application database or existing contributions. The test removed its fixture after exporting evidence.

The validated local image ID is `sha256:b7fad75ae17f99be50e6b750e3a225de6e7979eb290a9aba524d3f72de33756e`. The unused full-execution experiment image was removed. The source/configuration adapter and `bin/forgeflow-container.exe` are ready; live `configs/local.json` and the running backend were intentionally not changed. Enable the documented optional configuration and restart the backend before resuming a real contribution through its existing human approval gate. This proves the disposable Go workflow, not compatibility with every external repository or missing Java/service toolchain.

## Repository-driven environments — 2026-10-12, pipeline version 7

Environment preparation now accepts repository-specific Linux toolchains, libraries and installation commands through a validated, language-independent recipe. It supplies actual container inventory and preparation evidence to the agents, separates known environment failures from source failures, and requires a changed executable recipe before retrying a failed environment. Existing work, including commit `b21f7f3`, was preserved. No existing contribution was resumed and no PR was submitted or merged during this validation.

### Automated checks

- `go test ./... -count=1`: every package passed; output is in `.cache/environment-validation/regression-final.log`.
- Subsequent targeted tests passed after the final fingerprint and failure-classification changes. They cover missing libraries, incompatible toolchains, provider usage limits, assertion text containing environment errors, structured environment repair requests, and rerunning the exact required command after repair without a source-fix iteration. An unchanged repair stops; both failed and recovered command records remain linked. Output: `.cache/environment-validation/targeted-final.log`.
- `go vet ./...`: passed. `go build -o bin/forgeflow-environment.exe ./apps/server`: passed. `git diff --check`: passed.

### Real Docker proof

`TestRealGenericEnvironment` passed in **57.36 seconds**. The disposable fixture installed pinned npm library `semver@7.7.2` into a derived image, then ran actual Node checks and Python unit tests offline. Prerequisite and isolation checks also passed. Reusing the same inputs worked offline; changing the manifest to require Node 999 produced a blocked environment instead of accepting the previous image. An explicit private-service blocker prevented commands from running.

Cancellation was exercised during a real workspace setup command after its readiness signal. Its delayed child did not survive to write its output. The runner returned cancellation, removed its containers, and preserved the Git boundary. Go test cleanup removed the disposable workspace; a subsequent filesystem check confirmed it was absent, and no labelled verification containers remained. Reusable image/build layers were retained intentionally.

Evidence: `.cache/environment-validation/generic-completed.log` and `.cache/environment-validation/generic-completed/generic-environment.jsonl`. Earlier failed attempts remain in separate evidence directories. One exposed BuildKit treating a bare configuration SHA in `FROM` as a registry reference; the implementation now uses a locally pinned base tag checked against its immutable image ID.

### Live model boundary

The pipeline-7 persisted live Codex workflow **has not passed end to end**. The first attempt exposed insufficient environment inventory and invalid recipe settings; inventory probes and one bounded validation correction were added. The next attempt stopped during planning because the Codex account reported its usage limit. Its execution/audit evidence remains at `.cache/environment-validation/live-with-inventory/workflow.json`. Usage exhaustion now pauses without automatic retries or code-fix iterations. The pipeline-6 editing/review proof above remains historical evidence, not proof of the new environment-planner stage.

Restart the backend to load pipeline 7. A fresh disposable live workflow is still required after account capacity is available. Automated recovery fixtures and real Node/Python provisioning do not establish universal ecosystem compatibility; platform SDKs, external services and private credentials can still require operator input. See [container execution](container-execution.md) for configuration, repeatable checks and limitations.

## Container preflight deadline correction — 2026-10-12

The Bitbucket contribution's latest approved resume stopped at container isolation before environment planning, coding or repository verification. Its saved error was `context deadline exceeded`, and the named container was cleaned up. The running backend reported recovery version 7. No new source-fix iteration was started by that attempt.

Inspection found conflicting deadlines: the execution caller allowed 45 seconds for native plus Docker preflight, while Docker creation independently allowed two minutes. Settings also allowed only 60 seconds for the container check. A slow create could finish after either caller's deadline, preventing the actual isolation command from starting. The saved failure lacks per-stage timings, so the exact reason that creation was slow is not established.

The CLI now declares its preflight budget: 45 seconds for native verification plus three minutes for container creation, the short probe and cleanup. Execution honors that budget; Settings uses the same container budget. Native verification keeps its own 45-second deadline. Parent cancellation still prevents the probe from running, and cleanup retains its independent bounded context.

- Focused deadline, isolation-argument and infrastructure-failure regressions passed. The execution fixture proves the runner receives its declared budget and a failed preflight starts zero agents and consumes zero source-fix iterations.
- `TestRealContainerPreflight` passed in **4.14 seconds** against the configured local image. It proved real workspace writes, denied root/Git writes, UID 1000, offline networking and absent Docker socket in a disposable fixture. Test cleanup removed the fixture. Output: `.cache/preflight-validation/docker.log`.
- `go vet ./internal/codex ./internal/execution` and `go build -o bin/forgeflow-preflight.exe ./apps/server` passed. `git diff --check` passed.

The running backend was not restarted and the contribution remains paused. Restart ForgeFlow to load the correction before explicitly recovering the contribution through its existing approval gate. This successful disposable preflight does not establish that the contribution's environment setup, code or full tests will pass.

## Environment planner context correction — 2026-10-12

The next Bitbucket resume passed preflight and stopped at environment planning. Its recipe selected `golang:1.27.1-bookworm` but also listed replacing Go 1.26.1 as an operator blocker. It treated the offline inventory's network flag as conflicting with the current approved download policy. No image build or repository verification occurred.

Inventory evidence now carries the execution's download authorization, while probes remain offline. Agent evidence explicitly includes current authorization, and the environment planner receives the approved issue and human constraints. Instructions distinguish installable dependencies from operator blockers and require evidence for mandatory contribution checks rather than indiscriminately treating release-only jobs as prerequisites. A saved blocked recipe requests a fresh plan on explicit resume; an unchanged executable recipe still stops without repeating preparation.

The repository's `AGENTS.md` and `CONTRIBUTING.md` independently confirm live Bitbucket testing is a contribution gate. That remains unsupported by the current single-container runner and cannot be replaced with a unit-only pass. These context corrections do not supply Docker-backed services or native platform runners and do not prove the contribution is ready.

Regression fixtures verify offline inventory does not revoke authorization, the approved issue reaches the planner, and resuming an unchanged blocked recipe replans without repeating setup. Existing environment and verification-repair regressions passed. No contribution was automatically resumed, and its patch and audit history remain preserved.

## Recovery flow validation — 2026-10-12, pipeline 8

The implementation keeps native MXC editing and Docker verification. It adds fresh, bounded issue-discussion evidence, plan invalidation on scope changes, protected hook preparation, explicit external completion requirements and recovery that preserves an independent approval only for an unchanged patch. The dashboard shows preparation logs, external requirements and stale-plan status. Existing application changes and contribution history were retained. No PR was submitted or merged.

### Actual disposable proof

`TestRealCodexWorkflow` passed in **309.64 seconds**. Four separate authenticated sessions performed environment planning, implementation planning, code editing and independent review. The execution reached **READY** with four passing commands, zero source-fix iterations and one review cycle: `go test ./...`, `go vet ./...`, an offline repeat of the tests and a formatting check. The exported record includes the actual diff, commands, agents and an event snapshot. A filesystem check confirmed that its disposable workspace was removed.

Evidence: `.cache/flow-recovery/live-pipeline8-exported.log` and `live-pipeline8-exported/workflow.json`. This is a disposable repository proof, not a READY claim for Praetor. An earlier live run also reached READY, but the test failed to export its record because the evidence directory was missing. Its log is retained at `live-pipeline8.log`; the harness now creates the directory before consuming sessions and exports the complete event stream for future runs. The successful run above predates the optional per-command duration field; that backward-compatible field has separate validation/deadline regressions.

`TestRealHookPreparationIsolationCancellation` passed in **104.62 seconds**. Hook installation could write `.git/hooks` and `.git/info/lefthook.checksum`, while Git config and other info files remained read-only. Ordinary verification could not modify either prepared path. Cancellation removed the container and its delayed child; test cleanup removed the disposable directory. Evidence: `hooks-final.log`. Earlier failed attempts and their exact causes remain in the same evidence directory.

### Backend, browser and budget guards

Every Go package passed across `backend-final.log`, `backend-remaining.log` and the focused API/storage/contribution runs. After adding command budgets, `budget-final.log` passed execution, Codex, API and storage tests again. Focused completion-report, external-receipt, discussion and timeout regressions passed. `go vet ./...`, server build and frontend production build passed. The browser recovery/environment evidence test passed in **8.9 seconds**. Final source checks are recorded alongside these files under `.cache/flow-recovery/`.

Known Docker engine outages stop without another model-generated environment plan or an attempted pull masquerading as a missing image. A Python subprocess missing an executable requests setup repair; `FileNotFoundError` cannot become a DNS `ENOTFOUND` retry. Python `TimeoutExpired` and Node `spawnSync ... ETIMEDOUT` pause verification without consuming source fixes. Plans may declare `timeout_seconds` up to 7200 for repository-supported long gates; zero preserves existing defaults. Native and container validators reject negative or excessive budgets. Image setup remains bounded separately to 15 minutes.

### Praetor preparation and remaining requirements

The selected workspace is still at approved base `ede0d5d55f1845ebb6b9d08238255c7e38471571`. The patch fixes missing replacement suffix packages, nested self-target identity checks and report self-target refusal. The complete `internal/needs` race suite and report/MCP regressions passed. Positive fixtures scan the consumer separately from its framework, and new negative fixtures exercise both direct and nested self-target reports.

Repository evidence selected Go **1.27.1**, Node **24**, Python **3.14**, Lefthook **2.1.14**, Semgrep **1.179.0** and the hash-locked hook linters. Actual hooks installation, hooks checking and state audit passed. Git ownership was independently reproduced: Windows bind mounts appeared owned by UID 0, while verification used UID 1000; Praetor's Git probes discard ForgeFlow's inherited ownership exception. An image-local launcher now forwards arguments to `/usr/bin/git` with only `-c safe.directory=/workspace`, retaining normal hooks and read-only metadata.

Two full `make verify-all` attempts are retained. The first matching-image run failed two C++ scans at their repository-defined 20-second deadlines. Exact follow-up scans completed in **9.56** and **7.56 seconds**. A sequential full run passed the scanner and subsequent gates, then stopped at the VS Code setup test's 120-second CLI-build deadline. A bounded CLI-build probe completed in **83.87 seconds**; after preparing that build, the unchanged focused VS Code setup test passed. The recipe now prepares that required CLI build in `.forgeflow-runtime`, and all preparation checks passed. This evidence supports cache preparation and avoiding concurrent heavy verification; it does not establish the precise cause of every earlier slowdown.

**The complete Praetor gate has not passed, and independent contribution review and a trusted signed receipt remain required.** Focused passes do not replace it. Its commit-bound receipt cannot certify an uncommitted patch. The reviewed-patch handoff is supported; importing externally signed commits and host-side PR preparation for hook-bearing repositories are not supported. The application pauses instead of forging keys, changing trust anchors, bypassing hooks or repeatedly recovering that boundary.

The actual commands, matching environment recipe, prerequisite outcomes and unreviewed patch are saved in the contribution's `.autopilot/manual-validation-2026-10-12/` and `artifacts/manual-issue299.patch`. The live database received append-only command/audit records and updated human constraints; the obsolete plan is marked for refresh. The contribution remains **PAUSED**, and no live coding worker was restarted. Bitbucket service provisioning remains deferred to another laptop as requested.

The repository's development MCP wrapper additionally timed out running `git status --porcelain` at its 10-second bound, and `state sync` reached its separate Git-status deadline. These are retained in `praetor-real-mcp-report.log` and `praetor-checkpoint.log`; ledger tasks remain open. A diagnostic using Git's documented [minimal stat checks](https://git-scm.com/docs/git-config#Documentation/git-config.txt-corecheckStat) returned the correct six changed/untracked paths in a 10.42-second complete container command, but it was not adopted as a proven repair. Short metadata deadlines on this Windows bind mount remain a repository-specific setup limit. They are now classified as timeouts, rather than authorizing source fixes or weakening provenance checks.

A separate fresh native MCP transport proof passed without altering the repository's development wrapper: the server was compiled from a before/after-verified source fingerprint, and initialization confirmed that exact version. Real JSON-RPC calls against disposable consumer/framework fixtures accepted the distinct consumer and rejected direct and nested self-target reports. The fixture was removed. Evidence: `praetor-mcp-transport-proof.log`. This proves the tool behavior, while the wrapper and ledger deadlines above remain unresolved.

The final server executable also started successfully against an isolated proof database on port 18081. `/api/health` reported live mode, status `ok` and recovery version 8. The proof server was stopped afterwards; live application data and paused contributions were not used for that startup check. Evidence: `startup-health.json`.

Local Docker/MXC defaults are saved in `configs/local.json`; the immutable bootstrap image does not select a contribution's language versions. Start Docker, then run `npm run dev`. Explicit recovery refreshes scope and reuses the preserved patch and preparation evidence. Python provisioning uses uv's [documented managed Python installation](https://docs.astral.sh/uv/guides/install-python/), with a pinned installer and interpreter in the recorded recipe.
