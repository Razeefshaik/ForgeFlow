# Contribution Issue Reactor

## Design and evidence

The reactor sits inside the approved execution workflow, between an agent result or a verification failure and the next workflow decision. It does not create a separate contribution or submit a PR.

The motivating contribution had dependency downloads approved, but nine Go commands failed before tests started while attempting to download the Go toolchain through the discard proxy `127.0.0.1:9`. These setup failures were fed back to the code fixer. A later contributor reported BLOCKED after fixing source because its own verification could not run. Neither event establishes whether the patch is correct.

Data flow: **agent/command evidence → diagnosis → permitted recovery → rerun the real command → code fixes → independent review → human review**.

The current diagnosis appears next to execution controls, with cause, action, evidence links, and recovery count. Resolved diagnoses move into the immutable event timeline. Historical failures must not remain the current status after execution resumes.

## Recovery decisions

| Evidence | Automatic response | Completion evidence |
| --- | --- | --- |
| Approved network plus an inherited loopback discard proxy | Remove only that placeholder from the child environment; keep sandbox network policy and normal proxies | Actual sandboxed command succeeds |
| Connection timeout/reset or temporary dependency service failure | Retry the same command with a short cancellation-aware backoff, within a contribution recovery budget | The retry's actual exit code/output |
| npm dependency missing, absent node_modules, and an existing package lock | Run a bounded `npm ci --ignore-scripts --no-audit --no-fund` in the workspace if downloads are approved; retain install evidence | Original verification command succeeds |
| Agent returns BLOCKED solely because verification could not run, with a patch present | Ask the control plane to run the saved required verification; do not accept the agent's test claims | All required checks pass and independent review approves |
| Assertions, compilation, or repository logic fail | Existing bounded Codex fix loop receives the latest command evidence | Rerun real tests and independent review |
| Missing download approval, credentials, executable, denied filesystem access, unavailable Docker/WSL, changed issue/Git identity | Stop with a structured cause and specific next action; preserve work | Required capability is restored and fresh checks succeed |

The rules use actual runner errors, exit codes, commands, and outputs. Agent summaries are advisory: a verification-related BLOCKED result cannot certify success, and a substantive issue/scope blocker cannot be overridden by a test pass.

## Implementation plan and milestones

1. **Execution transport:** normalize inherited discard proxies only for approved network access. Probe the installed sandbox against a disposable fixture; retain the inside-write/outside-denial gate. Give Go verification a deadline compatible with its declared `-timeout`, with a fixed upper bound.
2. **Diagnosis and recovery:** add a persisted current incident, immutable detected/recovery/resolved events, bounded transient retries and locked npm dependency installation. Detect infrastructure failures before the code-fix loop. Accept a blocked agent's patch only for independent control-plane verification.
3. **Operator controls and UI:** expose diagnosis for existing blocked records, a recovery action using the saved execution/network approvals, and a compact workspace panel. Pause/stop cancels recovery; abandoned/terminal work stays terminal. Explicitly inspect old blocked work before recovering it.
4. **Verification:** unit fixtures for classification and permission enforcement, real sandbox network checks, real Git/Go workflow fixtures for blocked-agent recovery, frontend build, and browser fixtures for recovery states. Model fixtures are labelled; live contribution verification is reported separately.

Dependencies: existing Go execution runner, SQLite JSON records/events, contribution state machine, React/TanStack Query UI, and Codex CLI. No additional service or library is required.

## Limits and control

Three automatic infrastructure actions are allowed per approved execution session; transient retries have an additional per-command limit. Code fixes retain the configured iteration limit. Only exit code zero with no runner error counts as passed. Recovery never drops required commands, rewrites test assertions to pass, resets code, weakens isolation, changes host ACLs, starts privileged Docker/WSL infrastructure, or changes global credentials/proxies. The selected contribution model remains in use for code fixes and review.

Recovery is automatic during approved execution. Recovering an existing stopped contribution is an explicit action, preserves its recorded network permission, and starts with fresh required tests instead of another speculative code fix. The contribution must already have execution approval. Server restart and human stop/pause do not silently resume work.

The installed Codex permission profile remains the execution boundary. See [official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference) for profile network semantics and [security.md](security.md) for ForgeFlow's boundary.
