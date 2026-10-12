# Repository prerequisites before ranking

Discovery screens newly found repositories from their current contents, before ranking an issue or creating a contribution. Abandoned and failed contribution records do not decide the result.

## Architecture and implementation plan

1. Read the current default-branch commit and its complete file tree. Inspect contribution instructions, AGENTS rules, build manifests, CI, Makefiles and referenced local documentation/verification scripts. Read immutable Git blobs and check their content hashes; do not clone or execute repository code.
2. Compare declared requirements with the implemented execution boundary: Linux image provisioning for arbitrary toolchains and dependencies, two CPUs, 2 GiB RAM, unprivileged verification with a read-only root, no Docker socket, private credentials, trusted signing service, GPU or platform-specific service runner.
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
