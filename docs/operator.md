# Arnyx

Arnyx is ForgeFlow’s floating project companion, formerly Operator. Live mode uses the authenticated local Codex CLI already configured for ForgeFlow. No new provider, API key, or fabricated model connection is introduced. An executable being available is distinct from a successful LLM response; the UI reports both honestly. Demo/offline mode remains a limited deterministic inspector.

## Compact awareness

Every request reads persisted current state. The initial projection contains configuration and its version, counts over all contribution/agent records, eight recent contributions, five ranked opportunities, and eight running agent summaries. The current route is a focus hint; a contribution page resolves its real persisted record. Arnyx does not read the activity/event stream, agent transcripts, test output, diffs, issue evidence, or filesystem content into prompts.

The model can request at most two allowlisted lookups: a contribution (current phase, incident summary, review verdict, and the latest five test exit outcomes), an opportunity (bounded summary/ranking), a repository-name search, current configuration, or current agent summaries. Strings are projected and clipped in SQL. Initial state is capped at 20 KB; each lookup at 8 KB. Six short conversation turns, at most 800 bytes each, provide follow-up context. These are byte limits, not claimed model token counts. The visible context size measures the serialized state and short history, excluding model/system overhead. Each lookup may require another finite LLM call.

Chat is retained in memory across panel closing and route changes, up to 24 messages. Clear chat resets it. Reloading the page clears the conversation; only the companion’s position is stored locally. One in-flight request per server prevents competing LLM calls; the whole bounded lookup sequence has a 150-second deadline. Failed calls display a retryable error instead of a pretend response.

## Configuration and controls

Arnyx can read the full application configuration and propose multi-setting or temporary changes. Configuration proposals are validated, version-bound, and never applied automatically. Review the complete diff and choose Apply in Configuration. Temporary changes retain the existing expiration/version safeguards. Credentials and host/runtime settings are not part of this configuration.

Pause, resume, abandon, tests, review, prepare-draft, open-workspace and contribution-instruction requests render confirmation cards bound to persisted IDs. The model cannot execute them. Abandoning still requests explicit confirmation. PR submission remains in the contribution detail with its separate commit/text-bound approval. Automatic PR creation and merging remain forbidden.

The CLI runs in a unique empty request directory under `.cache/arnyx`, with read-only permissions, external tools disabled, plugins/MCP disabled and no network access for project tools. Request directories are cleaned on completion. Audits retain action/session/observed usage and context-size metadata; they are never recycled into model context. No chat message text or model transcript is added to these new assistant audit events.

## Companion

Click the avatar or the header’s Arnyx button to open chat. Drag the avatar to reposition it, or focus it and use arrow keys. Enter opens chat; Escape closes it and restores focus. The panel expands to `/arnyx`; old `/operator` links redirect there and the old chat endpoint remains compatible. The companion follows the current route’s dual-color palette, supports touch, and honors reduced motion and the app’s pause-animation setting.

Avatar: Bottts by Pablo Stanley via [DiceBear](https://www.dicebear.com/styles/bottts/), downloaded into the app with creator/license metadata preserved. It does not call a remote avatar service at runtime.

## Validation

Unit/integration checks cover prompt exclusions, current focused records, bounded history/lookups, configuration and execution approval gates, unsafe action/config rejection, old/new endpoint compatibility, and malformed input. Opt-in `FORGEFLOW_REAL_CODEX_TEST=1` tests exercise the actual authenticated local LLM against temporary databases for state-aware conversation and an expiring multi-setting proposal. They never operate on a real contribution or submit a PR.

Browser checks cover dragging, bounds after resize, keyboard/focus behavior, mobile layout, image loading, short-context request shape, retained/cleared conversation, configuration review, route palettes and reduced motion.

After deploying these source changes, restart the existing ForgeFlow backend once. The UI detects an older backend and explains that restart is needed instead of calling the old, larger-context Operator implementation.
