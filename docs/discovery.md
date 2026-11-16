# Live GitHub discovery

Start `npm run dev` (or its `dev:live` alias). Automatic discovery starts after startup unless a saved Pause preference disables it. Open Opportunities and select Run discovery now for a manual scan. Public searches need no credentials. Authenticated public scans use the saved browser sign-in, then GH_TOKEN, GITHUB_TOKEN, or `gh auth token --hostname github.com` with a three-second timeout. See [GitHub account setup](github-sign-in.md). Browser login updates the running client and clears the former credential’s rate-limit backoff. Tokens never enter the browser, configuration, database or events. Private repositories are rejected; this phase supports public discovery only.

## Lifecycle and bounds
A scan snapshots the applied profile version before starting. Only one scan runs at a time. Run returns HTTP 202 immediately; work uses the server context, not the request context. Cancellation retains fully analyzed observations. Bounds: eight minutes per scan, 500 network requests when signed in (160 otherwise), twelve seconds per request, four MiB per response. Reads are serial and do not follow redirects or execute repository content.

**Repositories first (2026-11-16).** Discovery used to search all open issues on GitHub by most recent update. That returned mostly days-old personal repositories: forty screened repositories produced one opportunity. It now searches repositories. For each of the six highest-weight languages it runs two searches, ordered by stars: repositories with open good-first issues (`good-first-issues:>0`) and repositories with open help-wanted issues (`help-wanted-issues:>0`), both restricted to public, unarchived repositories pushed within the configured activity window and with at least the configured stars. Each search has a saved page and position; scans continue from unprocessed results, progress survives restart and commits atomically with observations. See [discovery progress](discovery-progress.md).

Candidates are interleaved across the searches. Lists, tutorials and study material (by name or topic) are skipped: they pass every build check, but a contribution there is an edit to a list. A repository excluded under the current rules is not screened again for seven days, and one withheld for incomplete evidence for one day. Every other repository is screened for [contributability](repository-eligibility.md).

For each contributable repository discovery reads its open, unassigned issues (the hundred most recently updated, then the first two preferred labels by name if too few matched). An issue is offered when it:
- carries a preferred label, matched on label variants (`kind/bug`, `type: bug` and `good-first-issue` count; `debug` does not);
- has no assignee and is not locked;
- is not labelled as unready (blocked, question, duplicate, needs information or triage, discussion, proposal and similar);
- is not excluded by the profile (frontend-only, documentation-only) and matches the configured difficulty;
- has at least 80 characters of description;
- is invited (good first issue, help wanted and similar), or is a plain bug or enhancement opened within the last year;
- is open to outside contributors under the project's own stated rules (for example a required `help wanted` label);
- has no open pull request that mentions it, read from the issue timeline.

Invited issues are offered first, then by preferred-label order and most recent update. The scan's warnings report how many issues each rule held back.

Signed-in scans look for issues in up to eight contributable repositories while screening at most thirty, offering three issues per repository; unauthenticated scans two, eight and two. Each run records snapshot version, outcome, request count, considered/accepted counts, newly discovered versus refreshed opportunities, search positions and warnings.

**Outcome.** A scan that screened repositories and found nothing new to offer has SUCCEEDED. PARTIAL means GitHub cut the scan short (rate limit, rejected sign-in, network failure). FAILED means no search could be made at all. Reaching the request allowance, a repository too large to list, or one repository's missing resource is not a failure.

Automatic scans default to fifteen-minute intervals, with the first scheduled scan immediately after startup. Run starts one immediately. Pause/Resume persists a preference and audit event. A zero interval disables automatic scheduling regardless of the stored preference. Cancel is separately audited and does not pause future scheduled scans. The server must remain running; no operating-system task is installed.

## Evidence
Repository metadata includes topics, language, stars, forks, archive state and default branch. The latest default-branch commit SHA identifies the file tree checked for CONTRIBUTING, AGENTS, code of conduct, build manifests, CI and tests. A truncated/unavailable tree preserves unknown absence. Presence does not prove successful execution. Bounded prerequisite source files are read at their pinned blob identities; repository code is never executed during discovery.

Release sampling checks the latest five releases for a stable publication. Issue details include body, labels, timestamps, state, assignees and comment count. The first hundred timeline events provide a member/owner/collaborator comment sample. The latest thirty updated closed PRs provide an external-merge sample. Samples are biased and cannot establish merge probabilities or response times.

Competing work is read from the issue timeline: an open pull request that mentions the issue, from this repository or a fork, means the issue is not offered. A timeline longer than one hundred events cannot rule competition out, and the opportunity says so. Work that never mentions the issue and duplicate issues require human inspection.

## Cache and limits
Response bodies are cached in memory for fifteen minutes by full URL and a memory-only credential-scope hash. Expired entries use conditional ETag reads. Cache limits: 256 entries and sixteen MiB total bodies. Cached evidence may be fifteen minutes old. Observations/history survive restarts; HTTP cache does not.

Rate-limit responses use Retry-After or X-RateLimit-Reset to block network requests until retry time, displayed in the UI. A limited scan is PARTIAL or FAILED. Credential changes require restarting. Upstream error bodies and network exception strings are withheld from public errors. API conventions were checked against [GitHub search](https://docs.github.com/en/rest/search/search) and [REST best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api).

## Persistence and interpretation
New repositories are screened from pinned contribution instructions, manifests, CI and verification scripts before issue enrichment and ranking. Unsupported and incomplete prerequisite assessments are excluded, with their reasons and source links shown in Opportunities and GET /api/repository-exclusions. Assessments are audited and refreshed by discovery; approval freshly rechecks requirements. Abandoned contribution history does not determine this decision. A static assessment is not an execution guarantee. See [repository eligibility](repository-eligibility.md).

DiscoveryStarted, IssueRanked, DiscoveryFinished and schedule/cancellation events commit with domain writes. IssueRanked includes the full observation, preserving old evidence when current opportunity records refresh. Finished runs are immutable. Restart marks unfinished runs INTERRUPTED. A completion persistence failure pauses automatic work and leaves a recoverable unfinished record.

Issues absent from a later bounded search remain visible. They may no longer be open or suitable under newer profiles. The UI shows timestamp/version and GitHub links. Execution stays disabled until isolated workspaces, revalidation, sandboxing, real tests and independent review exist. Discovery does not clone, invoke Codex, create contributions, push or open PRs.
