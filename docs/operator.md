# Operator

Live Operator uses an authenticated, fresh read-only Codex CLI context and structured output. It receives current configuration, contributions, agent records and up to twenty ranked opportunities. The domain dispatcher accepts only documented actions; model-provided shell commands or paths are not accepted. Demo mode uses deterministic inspection and Add Rust.

Configuration requests return a validated full proposal and diff. Nothing changes before Apply. Temporary requests include an expiration; expiry restores the prior configuration through a new version only if no newer settings superseded it.

Contribution pause/resume/abandon/test/review/prepare requests return confirmation cards bound to a persisted contribution ID. Control execution requires the user's click. PR submission remains in the contribution detail with a distinct commit/text-bound approval. Unknown or ambiguous targets return clarification rather than an arbitrary command.

Operator messages/responses and CLI session/usage observations are audited. Official plan allowance is unavailable. Runtime CLI login is separate from build-chat plugin connections.
