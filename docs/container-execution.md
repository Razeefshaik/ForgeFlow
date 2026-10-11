# Optional container verification

## Decision

Keep ForgeFlow, SQLite, the dashboard, authentication and Codex editing on Windows. Use the native MXC write boundary for agents and an explicitly configured Linux Docker image for deterministic verification commands. This uses the existing separation between `Runner.Run` and `Runner.Command`; it does not migrate the application or put credentials in a container. Native-only verification remains the default.

The bounded host comparison reproduced the reported failures with Codex CLI 0.162.1 and Git 2.47.0: host Git shell succeeded, MXC rejected MSYS's named-object creation, unelevated execution rejected its signal pipe, and elevated execution failed runtime ACL refresh on the running desktop Node helper. Disabling desktop integration settings did not fix refresh. Native MXC can still be used for ordinary code inspection and editing; this does **not** establish compatibility with arbitrary native MSYS-dependent commands.

Docker Desktop was already installed but stopped. Starting it worked; no cached images were present. WSL Ubuntu existed but had no Linux Go or Node executable available on its command path. Using Docker only for verification avoids migrating the developer environment or repeatedly stopping desktop helpers.

## Architecture and boundaries

`verification_image` configures `CLI.Command` and its Git-shell capability probe. `CLI.Run` retains the native permissions profile, authenticated CLI, event stream, role-based read-only policy and cancellation. It tells contribution agents that required verification is performed separately on Linux. The environment manager verifies native write isolation and container isolation before reporting readiness or selecting a native mode.

Each verification creates one randomly named container with structured process arguments. Its only host mounts are the active independent clone at `/workspace` and that clone's `.git` mounted read-only. It has a read-only root, UID 1000, no capabilities, no new privileges, two CPUs, 2 GiB memory and a 256-process limit. A temporary filesystem and workspace-local Linux caches support builds. No host home, credentials, parent contribution metadata, control-plane directory or Docker socket is mounted. Host environment variables are not forwarded into the container.

Without download approval, `--network none` is enforced by Docker. Approved commands use a bridge network. Images must already exist locally (`--pull=never`); repository commands cannot trigger image downloads. An image is trusted operator configuration and must contain the required toolchain. The supplied image contains Go 1.26.1, Node 22.20.0, Python 3, Git and a C compiler; Java/Maven/Gradle are not included. Extend the image deliberately when a repository requires another toolchain. Tests requiring host services or Docker-in-Docker need separate design, not extra mounts or privileges added automatically.

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
