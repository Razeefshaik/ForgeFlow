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
