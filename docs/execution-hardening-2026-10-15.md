# Execution hardening — 2026-10-15

Branch: `codex/execution-hardening`, based on `ed5f959`. The implementation selectively incorporates the 13 reviewed Revision 3 source changes and fixes the remaining review findings. Bundled hooks, skills, configuration and executables were not copied. Existing application history, contribution workspaces and human approval boundaries are preserved.

## Implemented behavior

- Repository-detected checks and planned checks remain additive. Review always follows complete current verification. A changed patch cannot reuse an old passing command as proof of readiness.
- Test-side changes to the working diff allow one recorded verification repeat. Repeated mutation stops with an explicit incident. Failed-batch signatures include command arguments and every failed run, normalize durations and persist the no-progress streak across recovery.
- Constraint updates invalidate both the saved plan and its approval. A fresh plan is required before continuing.
- Setup evidence is classified before source assertions. Quoted diagnostic lines are excluded from classification independently, so an assertion in one test does not hide a real missing dependency, permissions error, ownership error or network failure in another. Original output remains in audit evidence.
- Diagnosis version 6 reinterprets old saved failed commands without rewriting history. The UI fetches the current read-only diagnosis for stopped contributions instead of relying on an obsolete client version threshold. Recovery still requires explicit human approval.
- Container HOME and caches persist only inside the active isolated workspace. Git trust remains limited to `/workspace`, with matching runtime/system stat settings. Unreadable generated directories remain in place unless emptiness can be established.
- Arnyx tasks and chat record measured model duration, excluding approval wait, and usage aggregation includes their recorded tokens/sessions with null guards. Unknown historical task durations are not reconstructed from wall-clock waiting time.
- Discovery history orders equal timestamps by insertion order, so rapid successive scans consistently expose the latest result without altering saved timestamps or audit history. The full suite exposed this existing clock-tick collision; a deterministic equal-timestamp regression reproduced it before the fix.

The previously broken false-READY test now resumes its fixture through PLANNING before entering CODING. An independent regression additionally runs real Go tests on a good patch, breaks it before resumed review and verifies actual failed and passing reruns. Mixed-log and version-five history regressions protect the corrected diagnosis path. Build metadata advertises recovery pipeline 9.

## Validation

Focused diagnosis, setup, command-retention, mutation and approval regressions passed (`.cache/execution-hardening-focused.log`). Discovery and storage packages passed after the timestamp-order fix (`.cache/execution-hardening-discovery-after.log`); the failing reproduction is retained in `execution-hardening-discovery-before.log`.

The frontend production build, server build and `go vet ./...` passed. All 12 browser checks for recovery, setup evidence and explicit approval passed against an owned demo server with an in-memory database on port 18082 (`.cache/execution-hardening-browser.log`). The server was stopped afterwards. No live contribution has been resumed and no PR has been submitted or merged.

`go test -p 1 ./... -count=1` completed: every package except discovery passed, including execution (281.508 seconds), Codex (87.942 seconds), operator, API, storage and workspace. The discovery failures revealed the equal-timestamp ordering issue above; after fixing it, the complete discovery and storage packages passed in 4.484 and 6.884 seconds. Thus every backend package has a passing result across the full run and the repaired-package rerun, rather than a claim that the original full command exited successfully. See `.cache/execution-hardening-backend.log` and `execution-hardening-discovery-after.log`. The additional container HOME unit checks passed (`execution-hardening-container-home.log`); their real-Docker counterpart was explicitly skipped. The final server rebuild and static checks for the subsequently changed packages also passed.

The live Docker persistence/preflight check is **not verified on this machine**. A single Desktop startup attempt crashed before creating the engine pipe: its backend could not access/remove `C:/Users/<HOST>/AppData/Local/Docker/run/sailor-ingest.sock`. Evidence is retained in `.cache/execution-hardening-docker-start.log`. No socket deletion, factory reset, image cleanup or repeated startup was attempted. Unit checks preserve workspace HOME contents, reject an existing repository file as HOME, and verify that a second workspace does not inherit installed files. `TestRealContainerHomePersistence` provides an explicit opt-in check across two fresh offline containers after Docker is repaired; it does not invoke a model.
