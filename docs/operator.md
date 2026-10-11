# Arnyx

Arnyx uses the authenticated local Codex CLI configured for ForgeFlow, with no separate API key. CLI availability is distinct from a successful task; errors and real activity appear in chat. Demo/offline mode remains a deterministic configuration inspector.

## Project and laptop folders

In live mode choose **Inspect files**, select the ForgeFlow project or enter an absolute path to another local folder, and ask your question. Codex has shell and file tools with read-only filesystem permissions. It can inspect source, diagnostics and logs rather than just repeat a status summary. Build/test commands that write files require an approved editing task.

Choose **Make changes** to request implementation or verification. A card shows the exact request, resolved folder and network setting. **Approve changes in this folder** starts a workspace-write session; only that folder is writable. Approval cannot be replayed or silently applied to a different request. Network/dependency downloads are selectable and default off. Local CLI authentication is reused.

Contribution repositories under the configured contributions root are inspectable, but editing them uses the contribution page's constraints, execution approval, tests and independent review workflow. Arnyx does not create a second coding agent in an active contribution. It does not push, rewrite Git history, submit/merge PRs, modify the live database, change OS settings or read credentials/browser profiles.

Tasks run in the background for up to 15 minutes. Chat polls actual activity and displays **Stop Codex task**. At most two tasks run concurrently; overlapping folders and concurrent tasks in one conversation are rejected. Reports, failures, cancellation and observed session/usage metadata persist. Server restart records running tasks as interrupted; edits never silently resume. Codex execution errors never become successful reports.

## Previous chats

**Previous chats** lists saved conversations by title and update time. Open a conversation to read it again. **New chat** preserves the old conversation. The browser remembers the selected ID; messages live in local SQLite and survive browser/server restarts. Earlier memory-only chats were never saved and cannot be reconstructed.

Browse 100 messages and 50 conversations per page, with controls to load older pages. Workspace tasks send at most seven clipped recent turns, a bounded current-state snapshot and the request. Focused contribution evidence includes the workspace, latest agent excerpt and two recent test excerpts. Tools can inspect more evidence with targeted reads; full activity streams/history are never pasted into prompts. User messages are capped at 2000 bytes.

## ForgeFlow controls

Choose **ForgeFlow controls** for configuration and contribution actions. This retains the structured assistant: two bounded lookups, six short prior turns, a 150-second deadline and a tools-disabled directory under `.cache/arnyx`.

Configuration changes produce validated, version-bound proposals. Review and Apply in Configuration. Contribution controls produce explicit confirmation cards. PR submission keeps its separate commit/text approval; automatic submission/merging remain forbidden. Saved replies remain visible, but historical confirmation buttons are not replayed.

## Companion and validation

The floating companion and `/arnyx` share conversations. Keyboard focus, dragging, mobile layout, reduced motion and route colors remain supported. Old `/operator` routes and chat requests remain compatible. Restart the backend once to load version 2; an older backend shows a restart notice.

Tests cover reopening, pagination, bounded context, approval/replay protection, contribution isolation, cancellation/failure and restart recovery. Opt-in `FORGEFLOW_REAL_CODEX_TEST=1` tests authenticated file inspection and an approved edit in a temporary fixture, never a real contribution. Browser checks cover history after reload, new chat, approvals, activity and responsive layout.

Avatar: Bottts by Pablo Stanley via [DiceBear](https://www.dicebear.com/styles/bottts/), stored locally with creator/license metadata.
