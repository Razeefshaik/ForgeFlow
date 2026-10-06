# Approved workspace preparation

Live opportunities now have an enabled Proceed to Contribute action. It first fetches fresh GitHub issue/repository/default-branch metadata and a bounded competing-PR search, bypassing the discovery cache's freshness window. The preview shows the current issue body, configuration version, base commit, branch and warnings. No workspace exists at this point.

The user must explicitly check approval and select Approve and start contribution. The server fetches the inputs again and compares the reviewed token. Changed input returns 409 and requires another preview. Closed issues and private, archived, disabled or renamed repositories cannot proceed. Assignees and possible competing work are warnings requiring human consideration, not claims of guaranteed suitability.

Approval is persisted with the contribution in one transaction before any clone. Configuration changes during approval are rejected. Repeating a successful approval returns the same contribution. A second active contribution for the same issue is rejected. Preparation concurrency follows the configuration snapshot's contributor limit.

Paths:
```
<ConfiguredContributionsRoot>/<id>/
  repo/                           independently cloned external repository
  .autopilot/
    approval.json                 reviewed inputs and timestamp
    metadata.json                 initial contribution record
    issue.json
    repository.json
    configuration-snapshot.json
    inspection.json               bounded instruction/manifest text
    commands.jsonl                actual Git command outcomes
    events.jsonl                  exported contribution audit history
    empty-hooks/
    empty-template/
  artifacts/                      reserved for subsequent tests/reviews/diffs
```

The manager creates an exclusive directory, refuses an existing directory and checks the contributions directory's resolved link boundary. It constructs a fixed public GitHub HTTPS remote from a validated owner/repository name; callers cannot supply arbitrary remotes, directories or commands. Git clone starts without checkout and uses empty hooks/templates, disabled credential helpers, no submodules, clean Git configuration environment and no symlink checkout. The branch is checked out at the approved commit, then HEAD is verified. Instructions/manifests are read as bounded text; they are not executed.

Git commands use structured process arguments, a five-minute preparation deadline, bounded output, child-process cancellation and real exit codes. SQLite stores command start/finish and lifecycle events; commands.jsonl stores completed commands. Audit history is authoritative in SQLite even when a filesystem write fails. Partial clones are retained for inspection. A restart blocks interrupted preparation rather than silently rerunning it.

After approved preparation, execution-approved contributions enter the Codex workflow; preparation-only records await explicit Start coding approval. The configurable root defaults to root/contributions for portability. This installation uses S:/StudyResource/TechBoooo/Backend/Contriss; ForgeFlow source stays in the main application root. Existing repo, metadata and artifacts were moved together, with 4020 SHA256 file matches verified. A persisted relocation event updates current workspace metadata without changing immutable approval history.

See workflow.md for planning, real verification, fixes, independent review, reports and PR approval. See security.md for the actual sandbox boundary.

Validation uses synthetic GitHub HTTP fixtures and actual local Git repositories, including a complete approved preparation path, duplicate approvals, changed commits, closed issues, exclusive/link boundaries, real failed commands and restart recovery. The browser approval test mocks the API contract and is explicitly not evidence of a live external clone.
