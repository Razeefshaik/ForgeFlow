# Discovery search progress

Repeated scans must explore further candidates while retaining prerequisite screening, human execution approval and all observation/audit history. Previously every scan requested the same first six results per language and assessed the same five repositories. Rejected repositories exhausted that allowance; cached repeats refreshed one opportunity without finding new repositories.

## Implementation

Use a bounded page of 25 issues per configured language. Persist each language's page and next issue offset in the existing SQLite metadata, keyed by the search-relevant profile and runner capability fingerprint. Commit progress with completed observations and the finished scan audit. Cancellation, interrupted requests and repository-budget stops leave unprocessed positions for the next scan; an exhausted search wraps to the latest page. GitHub search exposes at most 1,000 results, so 25-result pages are bounded to 40.

Separate the eligible-repository target (two public / five authenticated) from the screening budget (six public / twelve authenticated). Rejected prerequisites therefore do not consume the eligible target. Preserve existing three-minute, 160-request, 16-file and response-size bounds, and stop on rate limits rather than immediately retrying.

Record new versus refreshed opportunities and searched pages/offsets in scan history so users can distinguish exploration from refreshing existing results. Rank with the unchanged quality engine; do not change eligibility requirements to increase counts.

No new runtime dependency, Codex session, clone, repository script or contribution execution is required. Validate distinct later-page repositories, restart durability, partial-page continuation, exhaustion, cancellation/backoff and unchanged audits before delivery. Reference: [GitHub REST search](https://docs.github.com/en/rest/search/search).

## Verified outcomes

The running application's latest three scans each considered 17 candidates, assessed the same five repositories, accepted the same single repository and made zero new network requests. This confirmed repeated first-page sampling and cache reuse, rather than exhaustion of GitHub repositories. Two read-only public GitHub search requests returned 25 issues on each of pages one and two; the second page contained additional repositories. These candidates were not claimed to be eligible or executed.

The regression suite discovers 27 distinct repository fixtures across partial-page scans, a database reopen and entry into page two, with no contribution created. Four rejected fixture repositories do not prevent two eligible repositories from being found in the same public scan. Cancellation and rate limits keep unfinished work at its current position. Progress and observations roll back together on invalid persistence; refreshes are counted separately from new opportunities. Existing quality/effort and prerequisite guards remain tested.

A source-level screening false positive was also reproduced: an example DSN's `sslmode=require` value was read as a prose requirement for private credentials. Environment assignments now separate values from requirement prose; explicit mandatory requirements are still checked. The prerequisite assessment version is now v2, so saved v1 classifications are reassessed before entering current ranking.

Apply these changes by restarting the development command, then selecting Run discovery now. Saved automatic-discovery preferences, earlier observations, audit events and contribution workspaces are preserved.

Backend discovery/storage/GitHub/API/prerequisite/ranking tests, `go vet ./...`, server build and frontend production build passed. Two Playwright checks passed in 15.9 seconds: new/refreshed counts with saved search positions, and excluded prerequisite evidence. Browser responses for these checks are explicit fixtures; the separate public GitHub page reads above are live evidence of additional candidates, not a claim that those repositories passed screening. The temporary browser-test Vite process was stopped; the user's running backend was preserved for an explicit restart.
