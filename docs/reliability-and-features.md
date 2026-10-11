# Reliability and feature additions — 2026-10-10

## Why these changes

ForgeFlow already has execution controls, an issue reactor, preserved evidence and explicit human gates. This iteration addresses concrete lifecycle defects and adds two user-facing features that support reliable long-running work. No external PR was created to validate this iteration.

## Research and priorities

| Feature | Evidence and ForgeFlow gap | Delivery |
| --- | --- | --- |
| System health and storage warnings | GitHub exposes setup steps so people can diagnose environment failures. ForgeFlow lacked a consolidated view of local service, workspace and storage readiness. | Implemented in Settings; refresh and 30-second polling; unavailable data never appears healthy. |
| Editable PR draft with approval renewal | GitHub PR templates emphasize change descriptions and issue context. ForgeFlow previously sent its internal report as the public body and did not let the user edit it in the app. | Implemented title/description editor; optimistic revision check; old approval invalidated after changes. |
| Durable checkpoints and explicit restart recovery | OpenHands maintains persistent conversation state and resume behavior. ForgeFlow already persisted evidence, but phase changes and execution records could commit separately. | Implemented atomic workflow/execution/event commits and legal multi-step retry transactions. |
| Repository PR template assistance | GitHub supports default and multiple repository templates; a generic body cannot satisfy every project. | Next priority: bounded template preview from the approved base, preserve required questions and leave attestations to the user. Not implemented here. |
| PR follow-up inbox | GitHub separates CI checks, merge blockers, approvals and review state. ForgeFlow currently ends its workflow at PR_OPENED. | Next priority: read-only CI/review/closed/merged monitoring with actionable links; unknown remote state must stay unknown. Not implemented here. |
| Draft-versus-ready submission | GitHub drafts delay code-owner review requests and cannot be merged. | Later: bind the draft/ready choice to the approved submission token. Not implemented here. |

These priorities are an assessment of the inspected repository, not claims that every competing product has each feature.

Primary sources checked on 2026-10-10:
- [GitHub session setup visibility](https://github.blog/changelog/2026-03-19-more-visibility-into-copilot-coding-agent-sessions/)
- [GitHub PR templates](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/creating-a-pull-request-template-for-your-repository)
- [GitHub PR stages, checks and merge status](https://docs.github.com/en/pull-requests/reference/pull-requests)
- [OpenHands persistent conversation state implementation](https://github.com/OpenHands/software-agent-sdk/blob/main/openhands-sdk/openhands/sdk/conversation/state.py)
- [SQLite connection hook contract for the installed driver](https://pkg.go.dev/modernc.org/sqlite@v1.60.1)

## Reliability changes

- Commit contribution state, execution evidence and audit events in one transaction. Invalid retry chains roll back entirely. Validate boundaries/concurrency before changing persisted state.
- Serialize PR preparation/submission, execution, retries, controls and constraint edits for each contribution. Refuse new operations during shutdown. Internal automatic local preparation is allowed only inside its owning execution worker; it never submits.
- Recover a worker panic into a blocked contribution, finalize agent failure, release the active task and reviewer slot, and preserve files/evidence. Unexpected workspace preparation failure retains partial clones. Persist failures are logged instead of silently discarded.
- Restart recovery inspects every running agent, including records beyond the latest 200 shown by the UI. Completed contribution states are retained.
- Reapply SQLite foreign-key enforcement and busy timeout whenever the pool opens a replacement connection.
- Enable Git long-path support during Windows workspace preparation and controlled Git operations. This fixes the reproduced pack-file failure under long directory paths.
- Before a submission retry, query existing PRs by head/base and match exact head SHA and approved title/body. Reconcile the existing PR without another fork, push, POST or issue-open requirement. Conflicting remote content requires inspection.
- Development launches put temporary/build files on the project drive. User profile storage is still checked separately because external tools can keep state there.

## Public PR text and human controls

Public drafts use a separate builder with change summary, changed files, latest recorded result per verification command, risks and issue reference. They do not copy internal reports, model labels or quality scores. Common generated-authorship claims are removed from generated summaries and rejected when saving/preparing/submitting a draft. Technical topics such as an LLM API regression remain valid. No invented human-authorship sentence is inserted. Internal audit records remain truthful. ForgeFlow does not currently post automated PR comments.

The editor is available only at READY or PR_PREPARED. Saving is local and audited. Concurrent edits return HTTP 409 rather than overwrite a newer draft. Submission requires explicit approval for the exact saved commit and text, and the server recomputes this binding. A legacy draft containing attribution must be edited before preparing/submitting; it is never silently rewritten after approval. No upstream disclosure or attestation has been automatically completed or removed.

## API additions

- `GET /api/health`: backward-compatible status/mode; HTTP 503 when the database is unavailable.
- `GET /api/health/details`: fresh, non-cached local checks and actual disk bytes; warnings return 200, required service failure 503. Tool presence is not sign-in verification. Disk threshold: 512 MiB.
- Execution GET includes `pr_revision`, derived from exact saved commit/title/body, including older records.
- `POST /api/contributions/{id}/pr-text`: `{ title, body, revision }`. No push or remote submission; stale revisions return 409. Existing explicit submission approval remains required.

## Validation scope

Regression tests cover checkpoint rollback, restarted agents beyond the display limit, replacement connection settings, worker panic cleanup, incompatible operations, PR text separation, stale drafts, renewed approval and recovering an existing remote PR without writes. Existing workflow tests execute real Git and small Go verification commands with synthetic agent responses in disposable workspaces. Browser fixtures cover health warnings/unavailability, desktop/mobile layout and exact approval renewal. Live paid/model sessions and real remote pushes/PR creation are not exercised.

Machine constraint: C: had approximately 54 MiB free during the audit. No unrelated user files were deleted. This remains a practical risk for tools storing state in the user profile; use System health to inspect current free space.
