# Managed execution recovery

The control plane must distinguish broken execution prerequisites from failed repository assertions. This extension preserves contribution approvals, isolated workspaces, required verification, independent review and immutable audit events.

## Implementation plan

1. Replace forced Windows settings with validated `auto`, `mxc`, `elevated` and `unelevated` modes. Explicit legacy selection disables `prefer_mxc`, so the requested mode actually runs. Never select unrestricted execution.
2. Provide local environment status and asynchronous check/setup endpoints. Setup runs only the installed Codex executable with fixed sandbox-setup arguments after explicit approval. Windows owns its elevation prompt. Serialize setup, bound execution, preserve actual errors and audit every transition.
3. Verify inside-workspace writes, denied sibling/control-plane writes and Git shell startup in a disposable workspace on the configured contribution drive. Save a mode only after all checks pass. Existing contribution approval and dependency policy remain separate.
4. Add Settings controls and a direct recovery link from sandbox incidents. Show actual stages and evidence; never fake percentage completion. Resume contributions through the existing approval workflow.
5. Preserve both assertions and runner timeouts. Investigate named Go failures with bounded focused commands; those diagnostics never substitute for required checks. Ensure Go's own timeout expires before the supervisor so hangs yield stack traces. Route confirmed assertions to investigation while keeping setup failures out of source-fix loops.
6. Validate configuration persistence, approval rejection, setup/probe failures, concurrency, deadline/cancellation, saved legacy diagnoses, API contracts and accessible desktop/mobile controls. Run full Go checks, frontend build and browser tests; execute installed-CLI probes without starting model sessions or publishing PRs.

No recovery can guarantee a working third-party service, credentials or platform-specific test fixture. When intervention is necessary, report the exact unmet prerequisite and offer an actionable recovery flow rather than repeatedly rerunning an expensive failed command.

## Operation

Open **Settings → Sandbox setup**. **Check environment** runs disposable isolation and Git-shell probes without a model session. For Windows setup, select the approval checkbox and click **Set up Windows sandbox**. Approve the native Windows prompt. ForgeFlow starts the installed Codex app-server and sends the fixed `windowsSandbox/setupStart` request for `elevated` mode. It waits for a successful `windowsSandbox/setupCompleted` notification, verifies the resulting policy and saves the working mode in ignored `configs/sandbox.json`. The setup connection never starts a model session or runs repository commands. Other sandbox modes can be tested with **Verify and save mode**. `FORGEFLOW_CODEX_SANDBOX` or `codex_sandbox` in `configs/local.json` overrides the saved mode after restart.

Recovery pipeline version 4 corrects the setup launcher used in version 3: `codex sandbox setup --elevated` is a deployment command requiring an already elevated orchestrator, so launching it from the ordinary ForgeFlow backend fails with `orchestrator_elevation_required`. The [official app-server setup API](https://learn.chatgpt.com/docs/app-server#windows-sandbox-setup-windowssandboxsetupstart) handles interactive administrator approval. ForgeFlow itself continues running as the ordinary user. A setup acknowledgement alone never marks the environment ready; setup completion, isolation and Git shell checks must all pass.

A sandbox incident links to this section with the contribution ID. Once verification passes, **Resume contribution** uses the existing recovery endpoint, preserves download approval and reruns required checks. It never publishes a PR. No working mode is claimed when setup is declined or Git shell startup still fails.

Go verification retains the planned command plus `executed_command` with its resolved executable and timeout. Default Go test-process timeout is ten minutes with a twelve-minute supervisor; explicit durations are respected up to a twenty-eight-minute process limit and thirty-minute supervisor. Focused reproductions use saved failed package/test names, preserve race/build-tag flags, stay inside the active module, and carry `recovery_of`. Assertion failures before a timeout are diagnosed as `test_failure_timeout` with both secondary causes.

If focused checks pass, ForgeFlow retries the complete required command once. A second incomplete run requests the missing fixture/environment information rather than entering a speculative source-fix loop. Verbose command output keeps a bounded beginning and end so the final failure or timeout stack is not discarded; truncated output is explicitly marked.

Windows sandbox behavior follows the [official OpenAI Windows sandbox documentation](https://learn.chatgpt.com/docs/windows/windows-sandbox). Explicit legacy modes disable MXC preference so that a provisioned elevated sandbox is actually selected.
