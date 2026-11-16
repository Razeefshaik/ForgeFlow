# Repository prerequisites before ranking

Discovery screens newly found repositories from their current contents, before ranking an issue or creating a contribution. Abandoned and failed contribution records do not decide the result.

## Architecture and implementation plan

1. Read the current default-branch commit and its complete file tree. Inspect contribution instructions, AGENTS rules, build manifests, CI, Makefiles and referenced local documentation/verification scripts. Read immutable Git blobs and check their content hashes; do not clone or execute repository code.
2. Compare declared requirements with the implemented execution boundary: Linux image provisioning for arbitrary toolchains and dependencies, four CPUs, 4 GiB RAM (the verification container limits), unprivileged verification with a read-only root, no Docker socket, private credentials, trusted signing service, GPU or platform-specific service runner.
3. Record one of three outcomes: eligible (no detected blockers), excluded (an evidenced unsupported requirement), or needs assessment (incomplete/ambiguous evidence). Only eligible repositories proceed to issue enrichment and quality ranking. Eligibility does not use Codex effort or alter quality factors.
4. Persist current assessments and append their evidence to immutable discovery/audit history. Show each excluded repository, reason, source file, line and pinned GitHub link. Reassess on subsequent discovery; do not maintain a permanent repository ban.
5. Freshly recheck requirements at approval time. A changed repository or runner profile invalidates the preview token. Enforce current eligibility in the contribution-creation transaction. Keep already-created workspaces and their audit records intact.

Live ranking requires an eligible assessment for the configured runner profile. Older opportunities remain stored but need a new discovery assessment before entering the live ranked list. An older in-flight scan cannot overwrite newer prerequisite evidence. Demo examples retain their illustrative behavior.

Uses the existing bounded GitHub client, SQLite, discovery/approval services and web query client. No additional dependency, model call, image build, source change or contribution execution occurs while screening.

## Evidence and bounds

Screening reads at most 16 files, 48 KiB per file and 512 KiB total for each repository. The scan retains its three-minute deadline and twelve-second request deadline; its request budget is now 160 to include prerequisite reads. Pinned blob responses use the [GitHub Git blobs API](https://docs.github.com/en/rest/git/blobs#get-a-blob). Public reads require no contribution credentials. The existing HTTP cache may retain metadata for fifteen minutes; approval uses fresh requests.

An unavailable/truncated tree, missing linked requirement, invalid blob identity, unreadable file, file-count limit, oversized evidence, cancellation or request-budget failure cannot become an eligible result. CI service/secret requirements whose local applicability is unclear are held for assessment. Different language/compiler/library versions, ordinary hook installation, optional Docker packaging and normal public dependencies are not reasons for rejection.

Required live services, private test credentials, trusted signing receipts, nested Docker/Compose commands, privileged/system writes, explicit unsupported platform/GPU requirements and declared resource minima above the runner limits are detected before ranking. Rules inspect required verification targets and their Makefile dependencies; unrelated release/deployment recipes do not reject a repository.

This is a static requirements assessment, not successful environment preparation or a test pass. Undeclared requirements, dynamically generated scripts and runtime-only failures may still surface after approval. ForgeFlow continues to prepare the actual repository environment and run real verification before declaring a contribution ready. Short Git-status performance deadlines cannot be proven satisfiable by reading manifests alone.

## Validation

Fresh HTTP repository fixtures contain no contribution history. Tests cover rejecting mandatory signing, private live-service and nested Docker requirements before issue enrichment or ranking; accepting versioned Go, Python, Java and Node requirements; optional Docker packaging; linked verification scripts; system privileges, GPU/resource requirements; unknown CI services; truncated, corrupt, oversized, missing and cancelled evidence; stale approval inputs; reassessment and unchanged scores/audit records. A separate regression proves old abandoned contribution failures do not affect new screening.

No repository was resumed, source-edited, submitted or merged as part of discovery screening. Existing abandoned contributions and their evidence remain untouched.

Actual checks passed: `go test ./internal/prerequisites ./internal/discovery ./internal/storage ./internal/workspace ./internal/api ./internal/ranking`, `go vet ./...`, the server build and frontend production build. The Playwright prerequisite-disclosure test passed in 5.6 seconds with explicit HTTP response fixtures. The new server also started in live mode with an in-memory database and automatic discovery disabled, and its health, opportunity and exclusion endpoints responded successfully. The temporary server and Vite process were stopped; existing services were preserved. These checks prove the screening implementation and its controls, not compatibility of arbitrary live repositories.

## Contributability (2026-10-20)

Prerequisite screening alone answered "does this repository need something ForgeFlow may not supply?" It never asked "can ForgeFlow build, verify and finish a contribution here?", so discovery offered a Backstage scaffold template with no real build and a 17-module Gradle project with a GWT widgetset, and both stalled. Every repository is now screened for contributability at discovery and again when a contribution is approved, cheapest evidence first:

1. **Build shape, from the pinned tree (no extra request).** Excluded: project templates (cookiecutter/copier files, `{{`/`${{` templated paths, scaffolder `skeleton/` with `template*.yaml`); heavy toolchains (GWT/Vaadin widgetsets, Android, Bazel, Pants, Buck, Xcode); more than 8 JVM build files or 6 Go modules; monorepos with more than 15 `package.json` or 10 Python projects; more than 20,000 files; no test files; no supported root build manifest.
2. **GitHub Actions timing (one request).** Only verification workflows count: release, publish, deploy, docs-site, security-scanner and housekeeping workflows, and scheduled runs, are ignored. No successful recent run withholds ranking (needs assessment). The latest three default-branch runs of a verification workflow all failing excludes the repository (`ci_failing`). A median longer than 10 minutes excludes it (`build_too_slow`): measured local verification is several times slower than GitHub's runners (osh-core: 6.4 minutes in CI, over 20 minutes in ForgeFlow's container). The median is saved with the assessment and sizes command deadlines during execution; it is not part of the approval token.
3. **Instructions and manifests (bounded blob reads), as before**, now skipping release, publish and docs workflows, whose deployment secrets are not contribution requirements.

Issues are also filtered: the search asks GitHub to omit assigned and PR-linked issues (`no:assignee -linked:pr`), discovery skips issues with an open PR referencing them, documentation-only issues are recognized from labels (an `enhancement` label no longer hides a handbook) and from titles, and the ranked list hides issues that already have a contribution. Each scan records why issues in eligible repositories were not offered.

Live validation against public GitHub on 2026-10-20 (scans on a copy of the live database): osh-core and the TechDocs template were excluded with three requests each; Skript, the repository ForgeFlow previously completed, was eligible; across three scans 58 repositories were screened, with exclusions such as tinygo (71-minute CI), argo-cd (33-minute CI), openrewrite (36 JVM build files), Android apps, persistently failing CI, signing receipts and private credentials, and the third scan accepted three issues from eligible repositories.

## Blockers and cautions (2026-11-16)

The 2026-10-20 rules passed one repository in forty. Most rejections were guesses, not evidence: any secret named in a CI workflow, more than sixteen files to read, a README over 48 KiB, a link to a file that does not exist, a credential word anywhere in a script. Screening now separates two things.

**A blocker excludes the repository.** It is a requirement that is stated in the contribution instructions or directly executed by a verification recipe, and that ForgeFlow cannot supply:
- trusted signing, private credentials, a live service, Docker inside verification, a platform or GPU, or resources above the runner limits, when stated in contribution instructions (CONTRIBUTING, AGENTS, development or testing documents) or in a development, testing or contributing section of the root README;
- Docker or system-directory writes in a Makefile verification target (`test`, `check`, `lint`, `ci`, `verify`, `unit*` and their dependencies) or in a script such a target runs;
- a CI test job that runs against service containers (`services:`), reported as `external_service`;
- the build-shape and CI findings below.

**A caution does not exclude.** It is recorded with the assessment and shown as a warning when a contribution is approved; the real baseline run decides. Cautions cover: signals from documents that are not the contribution instructions (a release guide, a component manual); scripts that are only named in prose; credential words in recipes (often a local test default); service containers used only by integration or end-to-end jobs; setup that installs system packages or needs root (the image build does this); secondary files left unread when the read limit (24 files, 128 KiB each, 768 KiB in total) was reached; and files too large to read. Contribution instructions, build recipes, CI workflows and root manifests are read first, so the limit falls on nested manifests and linked documents.

Secrets named in a CI workflow are neither: GitHub does not give secrets to pull requests from forks, so a project that accepts outside contributions already passes its checks without them.

**An assessment is incomplete (needs assessment)** only when primary evidence could not be read or verified: no usable tree or commit, an unreadable or corrupt blob, oversized contribution instructions, an interrupted scan, or no successful CI run to learn from.

Build shape and CI:
- Bounds are now 20 CI minutes, 25 JVM build files and 12 Go modules. The earlier bounds (10, 8, 6) were measured when verification ran on the Windows bind mount, about ten times slower than the volume copy used since 2026-10-20.
- Integration, end-to-end, smoke and similar workflows do not size the build or count as failing CI when a unit workflow exists.
- GitHub-managed `dynamic/` workflows and durations over six hours (a re-run long after the first attempt) are ignored.
- Failing CI excludes only when every verification workflow that ran on the default branch keeps failing; a red linter beside green tests leaves a baseline to verify against.
- A tree GitHub truncates, or one over the four MiB read limit, is `repository_too_large`, not a failed scan.

**A project's own issue rules.** Some projects accept outside pull requests only for issues they have marked: cli/cli requires the `help wanted` label and stated acceptance criteria, and forbids issues labelled `core`. Such a rule does not exclude the repository. It is read from the contribution instructions (never a README or CI), saved with the assessment as its policy, and applied to each issue at discovery and again at approval, when labels may have changed.

Live validation is in [the 2026-11-16 report](contribution-reliability-2026-11-16.md).
