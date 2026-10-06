# Live GitHub discovery

Start `npm run dev` (or its `dev:live` alias). Automatic discovery starts after startup unless a saved Pause preference disables it. Open Opportunities and select Run discovery now for a manual scan. Public searches need no credentials. Authenticated public scans use GH_TOKEN, then GITHUB_TOKEN, then `gh auth token --hostname github.com` with a three-second timeout. Tokens never enter the browser, configuration, database or events. Private repositories are rejected; this phase supports public discovery only.

## Lifecycle and bounds
A scan snapshots the applied profile version before starting. Only one scan runs at a time. Run returns HTTP 202 immediately; work uses the server context, not the request context. Cancellation retains fully analyzed observations. Bounds: three minutes per scan, 80 network requests, twelve seconds per request, four MiB per response. Reads are serial and do not follow redirects or execute repository content.

Search covers up to six highest-weight languages and six recent open issues per language, matching up to eight configured preferred labels using OR. Candidates are interleaved across languages. Metadata filters reject archived/disabled/private repositories, insufficient stars, unmatched primary languages and stale pushed activity. Label rules conservatively exclude frontend/documentation-only issues; label-derived difficulty must match the configured list. Domains affect ranking as keyword preferences, rather than hard exclusions.

Unauthenticated scans enrich up to two repositories, accepting two issues each. Authenticated scans enrich up to five, accepting three each. These are sampling limits, not complete coverage. Each run records snapshot version, outcome, request count, considered/accepted counts and warnings.

Automatic scans default to fifteen-minute intervals, with the first scheduled scan immediately after startup. Run starts one immediately. Pause/Resume persists a preference and audit event. A zero interval disables automatic scheduling regardless of the stored preference. Cancel is separately audited and does not pause future scheduled scans. The server must remain running; no operating-system task is installed.

## Evidence
Repository metadata includes topics, language, stars, forks, archive state and default branch. The latest default-branch commit SHA identifies the file tree checked for CONTRIBUTING, AGENTS, code of conduct, build manifests, CI and tests. A truncated/unavailable tree preserves unknown absence. Presence does not prove successful execution. Source file contents are not fetched or executed.

Release sampling checks the latest five releases for a stable publication. Issue details include body, labels, timestamps, state, assignees and comment count. The first twenty comments provide a member/owner/collaborator participation sample. The latest thirty updated closed PRs provide an external-merge sample. Samples are biased and cannot establish merge probabilities or response times.

Open PR search uses repository scope and issue number. Results require explicit `#number` or canonical issue URL references with numeric boundaries. Truncation prevents ruling out competition; known referencing PRs still reduce the score. Unlinked work, cross-repository PRs, quoted references and duplicate issues require human inspection.

## Cache and limits
Response bodies are cached in memory for fifteen minutes by full URL and a memory-only credential-scope hash. Expired entries use conditional ETag reads. Cache limits: 256 entries and sixteen MiB total bodies. Cached evidence may be fifteen minutes old. Observations/history survive restarts; HTTP cache does not.

Rate-limit responses use Retry-After or X-RateLimit-Reset to block network requests until retry time, displayed in the UI. A limited scan is PARTIAL or FAILED. Credential changes require restarting. Upstream error bodies and network exception strings are withheld from public errors. API conventions were checked against [GitHub search](https://docs.github.com/en/rest/search/search) and [REST best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api).

## Persistence and interpretation
DiscoveryStarted, IssueRanked, DiscoveryFinished and schedule/cancellation events commit with domain writes. IssueRanked includes the full observation, preserving old evidence when current opportunity records refresh. Finished runs are immutable. Restart marks unfinished runs INTERRUPTED. A completion persistence failure pauses automatic work and leaves a recoverable unfinished record.

Issues absent from a later bounded search remain visible. They may no longer be open or suitable under newer profiles. The UI shows timestamp/version and GitHub links. Execution stays disabled until isolated workspaces, revalidation, sandboxing, real tests and independent review exist. Discovery does not clone, invoke Codex, create contributions, push or open PRs.
