# Optional container verification

## Decision

Keep ForgeFlow, SQLite, the dashboard, authentication and Codex editing on Windows. Use the native MXC write boundary for agents and an explicitly configured Linux Docker image for deterministic verification commands. This uses the existing separation between `Runner.Run` and `Runner.Command`; it does not migrate the application or put credentials in a container. Native-only verification remains the default.

The bounded host comparison reproduced the reported failures with Codex CLI 0.162.1 and Git 2.47.0: host Git shell succeeded, MXC rejected MSYS's named-object creation, unelevated execution rejected its signal pipe, and elevated execution failed runtime ACL refresh on the running desktop Node helper. Disabling desktop integration settings did not fix refresh. Native MXC can still be used for ordinary code inspection and editing; this does **not** establish compatibility with arbitrary native MSYS-dependent commands.

Docker Desktop was already installed but stopped. Starting it worked; no cached images were present. WSL Ubuntu existed but had no Linux Go or Node executable available on its command path. Using Docker only for verification avoids migrating the developer environment or repeatedly stopping desktop helpers.

## Architecture and boundaries

`verification_image` configures `CLI.Command` and its Git-shell capability probe. `CLI.Run` retains the native permissions profile, authenticated CLI, event stream, role-based read-only policy and cancellation. It tells contribution agents that required verification is performed separately on Linux. The environment manager verifies native write isolation and container isolation before reporting readiness or selecting a native mode.

Each verification creates one randomly named container with structured process arguments. Its only host mounts are the active independent clone at `/workspace` and that clone's `.git` mounted read-only. It has a read-only root, UID 1000, no capabilities, no new privileges, two CPUs, 2 GiB memory and a 256-process limit. A temporary filesystem and workspace-local Linux caches support builds. No host home, credentials, parent contribution metadata, control-plane directory or Docker socket is mounted. Host environment variables are not forwarded into the container.

Without download approval, verification uses `--network none`. Approved commands use a bridge network. Verification containers always use `--pull=never`; the control plane provisions images separately under the existing download approval. The supplied image contains Go 1.26.1, Node 22.20.0, Python 3, Git and a C compiler. It is a starting environment, not a compatibility guarantee. The repository environment workflow below can choose another versioned Linux base and install tools and libraries for other languages. Tests requiring private services, host resources or Docker-in-Docker remain explicit blockers; privileges and host mounts are not added automatically.

The application records actual output, command, host working directory, container name/image, times and exit code in the existing test records and audit stream. Creation is bounded; after it completes, cancellation kills the attached client and removes the exact owned container, including descendants. Cleanup failure is returned as an error. A backend crash, Docker daemon outage, or creation that completes after its timeout can prevent normal cleanup; inspect the recorded reserved name once Docker settles. Labelled containers remain discoverable with `docker ps -a --filter label=forgeflow.verification=true`. Never globally prune other containers or volumes.

## Build and select

From the project root:

```powershell
docker desktop start
docker build --tag forgeflow-verification:go1.26-node22 scripts/execution-image
$env:FORGEFLOW_VERIFICATION_IMAGE = 'forgeflow-verification:go1.26-node22'
$env:FORGEFLOW_CODEX_SANDBOX = 'mxc'
npm run dev
```

Alternatively, preserve existing fields in `configs/local.json` and add `"verification_image": "forgeflow-verification:go1.26-node22"` and `"codex_sandbox": "mxc"`. A local immutable `sha256:...` image ID may replace the tag. Changing this setting does not resume contributions. In Settings, verify the configured environment, then explicitly resume an approved contribution. There is no automatic fallback, PR submission or merge.

Docker is required only when container verification is selected. Removing `verification_image` restores native verification, including its currently unresolved machine-specific Git limitations. Merely launching the dashboard does not require Docker.

## Repository environment preparation (pipeline 7)

An approved contribution now follows this sequence when container verification is configured:

1. Verify native/container workspace boundaries and fresh issue approval. Probe the configured container's actual tools and Git working directory offline.
2. Give a separate, read-only Codex environment planner the repository and observed inventory. It reads instructions, CI, version declarations, manifests and lockfiles across the repository and returns a structured recipe: base image, input files, image installation commands, workspace dependency commands, prerequisite checks, environment variables and blockers.
3. Validate the recipe and its input paths. Resolve a local base image or pull it only with download approval. The selected base must be Linux without inherited `ONBUILD` instructions. Build a generated Dockerfile from a bounded tar context containing only selected regular files. Commands run in the Docker build, never a host shell. Root package installation is confined to image building; runtime commands use UID 1000.
4. Prove container isolation, install workspace dependencies and run actual prerequisite checks. Preserve the recipe, image IDs, build output, command outcomes and failures in execution records and append-only audit events. A failed prerequisite pauses execution before coding.
5. Give implementation planners, contributors and reviewers a workspace-local snapshot of the environment evidence. `ENVIRONMENT_BLOCKED` requests a bounded, evidence-driven environment replan. It does not authorize changing source to compensate for missing tools, services or credentials.
6. Check recipe inputs again before verification. Changed inputs trigger replanning. Cached images are keyed by the base image ID and executable recipe/input fingerprint; changing explanation text does not count as a repair. Workspace installation and prerequisite checks still run against the current workspace. Each verification command gets a fresh container.

The recipe and container command interfaces have no language allowlist. They can express Go, Java, Python, Node, Rust, .NET, Ruby, C/C++, or another Linux-compatible toolchain, including mixed-language repositories. This is an extensible capability, not a claim that every ecosystem has been tested. The native command allowlist remains unchanged. Required verification and independent review are still mandatory after preparation.

Downloads, system packages and project dependencies are installed only inside the image or active workspace. An environment agent does not receive the Docker socket, host credentials or write access to ForgeFlow or another contribution. The generated recipe permits structured commands, not arbitrary Docker directives. Inputs are capped at 64 files, 4 MiB each and 16 MiB total; recipes and commands are bounded. Do not put secrets in recipes or repository inputs. Credentials must be configured through an explicit supported mechanism, not embedded in logs.

Failed setup can request one changed recipe correction within the existing recovery budget. Invalid model output gets at most one correction with the exact validation error. Repeating the same executable recipe stops. Known incompatible toolchains and missing tools trigger environment repair rather than source fixing; an unchanged or unsuccessful repair stops. Account usage limits stop without automatic retries. An explicitly resumed legacy fix phase caused by an environment failure reruns verification before spending another code-fix iteration.

Limitations: the planner must identify the relevant inputs and write meaningful version/service checks; this is not a deterministic compatibility solver. New requirement files omitted from a saved recipe may require replanning. Provisioning is bounded to 15 minutes, and long builds may need an operator-prepared image. The pipeline does not automatically launch service clusters, obtain private credentials, provision Windows/macOS SDKs, or grant GPUs/privileged access. Such requirements are reported as blockers. Image builds use Docker's builder isolation, which differs from the stricter runtime container limits. Cancelled Docker builds/pulls may leave reusable cache/image layers; exact runtime containers and their anonymous volumes are removed, while images and build cache remain for reuse. No global prune runs.

To run the real, model-free Node/Python provisioning proof (downloads a pinned npm library on its first build):

```powershell
$env:FORGEFLOW_TEST_ENVIRONMENT = '1'
$env:FORGEFLOW_TEST_CONTAINER_IMAGE = 'forgeflow-verification:go1.26-node22'
$env:FORGEFLOW_TEST_EVIDENCE = Join-Path (Get-Location) ('.cache/environment-validation/' + [guid]::NewGuid())
go test ./internal/codex -run '^TestRealGenericEnvironment$' -v -count=1 -timeout 15m
Remove-Item Env:FORGEFLOW_TEST_ENVIRONMENT
```

Restart the backend to load pipeline 7. Existing paused contributions remain paused until explicitly resumed through the app. The execution endpoint and timeline expose the recipe and preparation evidence; provisioning commands are not counted as passing repository tests.

## Validation plan

1. Compare native failures once and retain their exact output.
2. Validate argument boundaries and configuration precedence.
3. In a disposable external clone, prove native editing, real failing then passing Go tests including Git shell startup, native/container write isolation, offline network isolation, descendant cancellation and exact cleanup. Retain JSONL evidence outside the removed fixture.
4. Exercise the persisted execution workflow with separate planner, contributor and reviewer contexts, then run Go regression tests, vet and build. Actual results are recorded in [managed-execution-validation.md](managed-execution-validation.md); this plan is not proof of completion.

To repeat the live proof from the project root (uses authenticated Codex sessions in disposable repositories and never submits PRs):

```powershell
go test -c -o bin/container-validation.test.exe ./internal/codex
go test -c -o bin/container-workflow.test.exe ./internal/execution
node scripts/validate-container-execution.mjs
```

The script reads the configured contributions root, creates a timestamped evidence directory under `.cache/windows-execution-validation`, and stops on the first failed stage. `--workflow-only` repeats only the persisted workflow. Keep earlier evidence directories when diagnosing a failure.

## Pipeline 8 recovery

Fresh issue comments are read before environment planning. Existing plans without a matching discussion/constraint fingerprint are archived in audit events and refreshed while keeping workspace edits. Full comments are supplied as bounded files, not repeated inline in every model prompt. Excessive discussions stop with a scope-budget error instead of being silently truncated.

Recipes add `hook_commands` and `completion_gates`. Hook installation runs in the existing container with only `.git/hooks` and `.git/info/lefthook.checksum` additionally writable. These paths are bounded and reject links; configuration, refs, index and other Git metadata remain read-only. Normal checks see read-only hooks and checksum. Downloads and hook installation never run on the Windows host.

Completion gates declare their instruction source, required evidence, external action, public verification commands and whether the evidence binds the edited files or a commit. Working-tree gates run after tests and independent review. Missing or invalid evidence pauses without recoding; recovery reruns verification and preserves review only for an identical patch. Commit-bound gates export `artifacts/final.patch`, `.autopilot/verification-report.md` and `.autopilot/completion-handoff.json` for a trusted external checkout. An old HEAD receipt never certifies new edits. Signed-commit import is not supported, and host-side PR preparation refuses repositories requiring hooks or external attestation.

The Environment tab shows the actual recipe requirements, setup command output and completion requirements. Setup incidents expose explicit recovery, and a commit handoff does not offer a misleading retry button. Python subprocess timeouts and Docker engine outages pause as infrastructure failures; neither justifies a source-fix iteration.

Verification plans may set `timeout_seconds` to a repository-supported duration up to 7200. Zero retains the default. This covers composite gates across languages, whose child deadlines may exceed a generic command budget. Environment preparation keeps its separate 15-minute limit, and cancellation stays active throughout. A required CLI/helper build may be prepared in the workspace when integration tests impose a short startup deadline; the unchanged tests must still run their own build and checks.

This installation now saves container verification and MXC editing in `configs/local.json`, preserving its contribution root. Its immutable starting image is an inventory/bootstrap choice. Repository manifests, CI and lockfiles select each contribution's toolchains and derived image. Starting the application needs no language version in the terminal:

```powershell
docker desktop start
npm run dev
```

Paused contributions remain paused until explicitly recovered or resumed. See [the recovery plan](contribution-recovery-plan.md) for the verified path and external signing limit.
