# Praetor issue #299 verification — 2026-10-12

The earlier conclusion that this contribution needs no change was too broad. The root self-target refusal is implemented, but issue #299 explicitly tracks additional unresolved cases.

## Scope

Inspected the approved base `ede0d5d55f1845ebb6b9d08238255c7e38471571`, original issue description, current issue state/comments and the commit introducing the self-target guard. The live GitHub issue remains open. No contribution files, database state or Git history were changed during this check.

The [issue status comment](https://github.com/cordanaLLM/praetor/issues/299#issuecomment-6026896963) says the root guard landed and lists three remaining items. The [implementing commit](https://github.com/cordanaLLM/praetor/commit/4c7d0694b7501431d89e2d8beaff0796af668eab) also states that suffix-existence checking was not fixed.

## Source findings at the approved base

- `internal/needs/migration_analysis.go`: `analyzeMigrationWith` calls `refuseSelfTarget` before scanning. The guard compares filesystem identity and the target's root `go.mod` against the framework module. Existing regressions cover identical paths, symlinks, separate checkouts, module-path selections and distinct module prefixes.
- `internal/needs/migrate.go`: `rewriteImportPath` appends the upstream suffix to the mapped framework package. It receives no package inventory and does not check whether the resulting package exists.
- `internal/needs/migration_analysis.go`: the identity guard reads only the root manifest. A repository whose root is a consumer but whose nested module has the framework identity can pass that guard before nested projects are scanned.
- `internal/needs/extract.go`: `ScanRepoWithFramework` directly scans and applies framework coverage without the self-target refusal. `cmd/standardsctl/needs.go` calls it from `runNeedsReport`.

The issue's alternative requirements for safe self-analysis do not disappear simply because migration application is separately blocked by version/API admission. Publishing a false candidate or readiness score remains observable behavior.

## Diagnostic validation

Evidence and diagnostic scripts are retained at `.cache/issue299-verification/`. A matching `golang:1.27.1-bookworm` image was downloaded. The test copy is exported from the approved commit into an owned disposable directory under the configured contributions root. It includes the relevant source dependencies rather than unrelated release/devcontainer assets.

Tests are run inside an unprivileged container with a read-only root and Git metadata, two CPUs, a 2 GiB memory limit, no capabilities, no Docker socket and no host credential mounts. They are issue-specific diagnostics, not a signed `make verify-all` receipt or READY result. Added diagnostic cases assert the observed defect; their passing result means the bug was reproduced.

The initial full archive export exceeded its 60-second bound. The selective export initially included paths mentioned only in synthetic test strings; filtering nonexistent imports corrected that diagnostic setup. These setup failures are not repository-test results.

Runtime outcomes are appended after the actual commands complete. No tests are reported as passing from source inspection alone.

### Actual result

The focused command completed with exit code **0**. First compilation plus execution took **119.63 seconds**; the Go test package itself reported **0.663 seconds**. Existing root self-target regressions passed, including the Linux symlink case. Three additional diagnostic tests reproduced:

1. A proposed import `example.org/fork/db/pgx/pgxpool` when that destination directory does not exist.
2. `ScanRepoWithFramework` accepting the framework as its own target and returning a readiness report.
3. A distinct consumer root containing a nested module with the framework identity passing the root guard and reaching the scanner.

Evidence: `focused.log` and `results.json`. The earlier selective export omitted four local packages outside `internal/`; adding those packages from the same approved commit resolved that diagnostic setup error. Its original output remains in `focused-export-failure.log`.

The owned test container was removed successfully (`cleanup_exit=0`). Disposable export directories were removed with their resolved paths checked against the configured contributions root; records remain in `cleanup.json`. The active contribution workspace remains unchanged. The matching Go image remains cached for subsequent approved verification.

**Conclusion:** retain issue #299 as actionable. Its existing root-identity fix is verified by focused tests, while the three documented follow-up gaps are still reproducible at the approved base. Full repository gates, signed receipt and independent contribution review were not run, and the contribution was not marked READY. The saved no-change plan does not cover the issue's remaining scope and should be revised before further contribution execution.
