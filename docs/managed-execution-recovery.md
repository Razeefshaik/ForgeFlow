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
