# Recovery from observed Windows failures

The October 11 records exposed two different environment failures: Git for Windows
could not create an MSYS signal pipe inside a test, and Go package discovery could
not enter a protected, ignored, empty generated directory. Neither error establishes
a credential failure or a source defect. The main checkout also still used the UI
branch although recovery was merged in a sibling worktree.

Implementation milestones:

1. Reclassify saved command evidence, with explicit sandbox-process and Go package
   discovery signatures ahead of generic assertion/authentication matches. Preserve
   the observed error and original events; never treat a diagnosis as a test pass.
2. Recover an inaccessible generated directory only when it is empty, ignored,
   untracked, below a known output directory, and free of links. Move it into a
   workspace-local quarantine with an audit record, then rerun the unchanged command.
   Never delete files, change ACLs, or exclude packages/tests from verification.
3. After an observed Git shell failure, use short sandboxed capability probes before
   retrying an expensive build (including on resume). Do not require MSYS for unrelated
   repositories. Prefer a newer already-installed CLI and compatible MXC sandbox,
   with the existing restricted-token fallback; verify actual permissions and networking.
4. Add build identity to runtime/health and show checkout identity on development
   startup so running an old checkout is visible.
5. Reproduce both saved failures in regression fixtures, test guarded recovery and
   unchanged verification, exercise the real installed sandbox, and build/test the
   backend and frontend. Existing contribution execution approvals and PR gates remain.

No new dependency is required. Recovery stays inside the active workspace and is
bounded by the existing session budget. Persistent OS restrictions, real credentials,
external services and genuine scope/isolation failures still require specific input.

## Findings and validation

- Saved database records were inspected through a separate SQLite backup. Updated
  API diagnosis identifies `sandbox_process` for Bitbucket and `generated_path_access`
  for Praetor without rewriting the old events or executing contributions.
- A real sandbox Go check passed with the required native regex filter. Writable
  workspace and denied sibling/application writes were verified.
- The actual approved dependency endpoint returned HTTP 200. The same endpoint was
  denied with networking disabled. These probes did not use a model session.
- Git for Windows 2.47's MSYS shell still fails in local sandbox probes. The legacy
  sandbox reports a signal-pipe error; explicit MXC reports a named-object access
  denial. A successful simple MXC command does not prove Git compatibility. ForgeFlow
  now records this distinction and avoids repeated long builds or credential advice.
- Windows sandbox options and prerequisites are described in the
  [official Codex Windows sandbox documentation](https://learn.chatgpt.com/docs/windows/windows-sandbox).
  This change does not provision an elevated sandbox or change machine-wide ACLs.
- Go formatting and the full `go test ./... -count=1` suite passed, including guarded
  quarantine, legacy diagnosis, persistent shell failure and timeout-evidence tests.
  Frontend TypeScript/Vite build passed. Browser suite: 59 passed, two opt-in live
  discovery tests skipped. Tests used disposable databases; the real contributions
  have not been resumed and are not being claimed as completed.
