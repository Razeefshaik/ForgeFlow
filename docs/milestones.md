# Milestones and continuation

## Completed foundation
Architecture/workflow docs, isolated branch, React dashboard, Go API, embedded SQLite migrations, config versions/proposals, immutable event history/SSE, lifecycle validation, explicit demo mode, independent ranking/effort domains and deterministic Operator.
Real backend tests, vet/build, frontend production build and three browser tests pass. See verification.md for evidence and screenshots.

## Completed discovery
Read-only public GitHub REST adapter, environment/GitHub CLI authentication, bounded scans, cache/ETag revalidation, rate-limit backoff, repository/issue evidence and eleven explained quality factors. Durable scan history/snapshots, Run/Cancel and automatic Pause/Resume. Real public scans and live browser evidence/Run checks passed; see discovery.md and verification.md.

## Next: isolated workspaces and command runner
Allocate one workspace per approved contribution, snapshot issue/profile/ranking/plan and revalidate before starting. Resolve paths including junctions/symlinks. Require an enforceable sandbox, structured process arguments, deadlines, cancellation and real exit codes. Keep execution disabled until that boundary is implemented.

## Then
Workspace manager → sandboxed command runner → Codex contributor adapter → real testing/fix bounds → independent reviewer → AI Operator → final report/diff/PR preparation → human submission.

Important: Go store Transition exists as a domain primitive, not a public endpoint allowing a user to skip verification. READY must eventually require persisted real test/reviewer evidence in the orchestration service. No raw state-setting API should be introduced.

