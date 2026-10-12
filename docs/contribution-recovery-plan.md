# Contribution recovery implementation plan

## Architecture

Keep the existing local control plane, isolated contribution clones, native Codex editing and Docker verification. Repository requirements determine the image and dependencies. Do not introduce a second execution platform.

Separate three authorities: the control plane prepares the environment; Codex edits only the approved workspace; an external trusted signer retains private credentials. A receipt is verified by a real repository command, never accepted on an agent's assertion.

## Milestones

1. Supply bounded, fresh issue discussion to every planning/editing/review context. Invalidate obsolete plans when discussion or human constraints change, preserving the old plan and edits in audit history.
2. Add narrowly scoped hook preparation: the installer runs in the existing unprivileged container with only `.git/hooks` and the bounded `.git/info/lefthook.checksum` file additionally writable. Git configuration, objects and refs remain protected. Subsequent checks see read-only hooks and checksum. Never execute downloaded hooks on the host.
3. Represent submission-time external requirements separately from prerequisites for coding. Run real public verification after tests and independent review; missing signing evidence pauses with actionable instructions. No credentials in recipes and no false READY state. Recovery preserves a review only while the patch is identical.
4. Exercise positive, failure, cancellation, isolation and cleanup paths in disposable repositories. Run backend checks and document actual outcomes.
5. Apply the corrected issue scope to the approved Praetor workspace, address all three verified defects, and run relevant regression tests. Full required repository gates remain mandatory; missing trusted signing authority must remain an explicit incomplete step.

## Dependencies and limits

Docker must be running for isolated setup/verification. Base images and downloads follow the existing authorization. Hooks must install without changing Git configuration. Repositories requiring a different Git setup pause with evidence rather than gaining arbitrary Git writes. External signing requires the repository's trusted party; this change cannot manufacture their private key. Bitbucket service provisioning remains deferred at the user's request. No PR submission or merge is authorized by this plan.

Commit-bound receipts require a trusted external checkout. The runner protects Git history while agents edit; an old receipt for HEAD cannot certify an uncommitted patch. Pipeline 8 exports the reviewed patch, verification report and handoff JSON, and stops explicitly. Importing an externally signed commit is not implemented. Hook-bearing repositories also cannot use the existing host-side PR preparation, which disables hooks. This is a supported handoff boundary, not a claim of automatic end-to-end submission for those repositories.

## Verified implementation

Pipeline 8 keeps fresh issue discussion in bounded workspace evidence, archives obsolete plans on scope changes, installs hooks under the narrower mount, and verifies working-tree evidence after tests and independent review. Recovery retains an approval only for the unchanged patch. Environment logs and external requirements are visible in the dashboard; changed constraints mark the saved plan as needing refresh.

Actual disposable live Codex execution reached READY: four separate agent sessions, four passing verification commands, zero fix iterations and one independent review. Real Docker hook isolation, descendant cancellation and cleanup passed. The exact workspace was removed after exporting the execution, tests, agent records and event snapshot. See [validation evidence](managed-execution-validation.md) for outcomes and the failed attempts retained alongside them.

Windows-mounted Git trees appear owned by root inside Docker. Praetor's bounded Git probes deliberately discard inherited configuration, including ForgeFlow's ownership exception. Its matching image now contains a launcher forwarding arguments to `/usr/bin/git` with only `-c safe.directory=/workspace`. This survives configuration scrubbing and keeps hooks enabled. Actual `make state-audit`, `make hooks` and `make hooks-check` passed. Other repositories can express the same adaptation in their existing image recipe; no application migration is needed.

Docker outages and Python subprocess timeouts stop without source-fix or model-replanning loops. Missing Python subprocess executables request environment repair; an input-file `FileNotFoundError` cannot be misclassified as DNS `ENOTFOUND`. Changing a completion requirement's prose does not rebuild its image.
