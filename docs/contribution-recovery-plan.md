# Contribution recovery implementation plan

## Architecture

Keep the existing local control plane, isolated contribution clones, native Codex editing and Docker verification. Repository requirements determine the image and dependencies. Do not introduce a second execution platform.

Separate three authorities: the control plane prepares the environment; Codex edits only the approved workspace; an external trusted signer retains private credentials. A receipt is verified by a real repository command, never accepted on an agent's assertion.

## Milestones

1. Supply bounded, fresh issue discussion to every planning/editing/review context. Invalidate obsolete plans when discussion or human constraints change, preserving the old plan and edits in audit history.
2. Add narrowly scoped hook preparation: the installer runs in the existing unprivileged container with only `.git/hooks` additionally writable. Git configuration, objects and refs remain protected. Subsequent checks see read-only hooks. Never execute downloaded hooks on the host.
3. Represent submission-time external requirements separately from prerequisites for coding. Run real public verification after tests and independent review; missing signing evidence pauses with actionable instructions. No credentials in recipes and no false READY state. Recovery preserves a review only while the patch is identical.
4. Exercise positive, failure, cancellation, isolation and cleanup paths in disposable repositories. Run backend checks and document actual outcomes.
5. Apply the corrected issue scope to the approved Praetor workspace, address all three verified defects, and run relevant regression tests. Full required repository gates remain mandatory; missing trusted signing authority must remain an explicit incomplete step.

## Dependencies and limits

Docker must be running for isolated setup/verification. Base images and downloads follow the existing authorization. Hooks must install without changing Git configuration. Repositories requiring a different Git setup pause with evidence rather than gaining arbitrary Git writes. External signing requires the repository's trusted party; this change cannot manufacture their private key. Bitbucket service provisioning remains deferred at the user's request. No PR submission or merge is authorized by this plan.
