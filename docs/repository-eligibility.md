# Repository eligibility before ranking

ForgeFlow excludes repositories with saved, unresolved environment prerequisites from discovery enrichment, the current ranked opportunity list, and approval of new workspaces. This is an eligibility decision before quality ranking; Codex effort never changes the score.

## Architecture and implementation plan

1. Derive exclusions from current persisted execution diagnoses joined to their repository identity. Use compact fields rather than loading plans, diffs, logs or historical events. Exclude repository-specific setup failures and environment-diagnosed verification deadlines; ordinary source failures, usage limits, cancellations and a globally stopped Docker daemon do not reject a repository.
2. Check exclusions before repository enrichment or issue ranking. Save each skipped repository and its reason with the discovery run and an audit event. Filter previously discovered issues dynamically without deleting their observations or altering scores.
3. Recheck eligibility during approval preview and inside the transaction that creates a contribution. Display excluded repositories and links to the existing contribution on Opportunities.
4. Verify exclusion, early network savings, approval enforcement, preservation of scores/history, and restoration after the original incident is resolved.

This uses the existing SQLite execution records, GitHub discovery service, approval workflow and web query client. No new service, dependency, model session or repository script is needed. Existing paused contributions and their workspaces remain available for diagnosis and explicit recovery.

## Bounds and interpretation

An exclusion is temporary: clearing the incident through the existing recovery flow restores eligibility. Abandoning a contribution alone does not fix its prerequisites. If several contributions record unresolved setup requirements for the same repository, each must be resolved. Names are matched without case sensitivity.

Repositories with no recorded blocker remain discoverable, with their environment unverified. This guard does not prove that an unseen repository can run: discovery currently inspects metadata and file names, not the complete contribution requirements. A future read-only prerequisite assessment should inspect pinned contribution instructions and required services before selection, with unknown requirements distinguished from proven blockers. It must not reject repositories based on language, a mention of Docker, or guessed resource needs. Environment preparation and real verification still precede coding under the approved execution workflow.
