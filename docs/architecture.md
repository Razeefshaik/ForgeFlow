# ForgeFlow architecture

## Application root and external workspaces
ForgeFlow's apps, internal packages, scripts, documentation and runtime directories live directly in the main repository root. SPEC.md is the canonical specification, normalized from the identical supplied SPECS.md. The requested ForgeFlow name refers to the specification's Open Source Autopilot product.
The root .git owns the application. External repositories belong only under <ConfiguredContributionsRoot>/<contribution-id>/repo after Proceed to Contribute. The directory initially contains only .gitkeep. The earlier nested application was relocated intact; its history is retained in archive/forgeflow-foundation and ignored recovery backups.
Startup scripts derive the root from their own location. The server locates it from an explicit --root, its executable location, or its working directory. Relative database/frontend flags resolve against this root. Embedded SQLite migrations remain packaged with the server. The external workspace path helper uses the same absolute project root; it does not create or execute contributions.

## Runtime

Go modular monolith, embedded SQLite migrations, React/TypeScript/Vite dashboard. Discovery reads bounded public GitHub data and persists evidence before independent quality ranking and effort estimation. Ranking excludes effort.

Workspace preparation binds reviewed inputs to human approval, then allocates an independent clone under the configured external root. Execution owns finite native Codex sessions, deterministic actual verification and bounded fixes/review. SQLite records each phase, agent/test outcome and event; metadata exports support inspection. Fresh review contexts have no contributor conversation history.

The domain dispatcher applies validated configuration proposals and contribution controls. AI Operator produces structured actions and human confirmation cards. Temporary configuration expiry never overwrites newer manual settings. CLI and GitHub credentials belong to runtime services, not chat plugins.

PR preparation makes a local reviewed commit. Submission is separately human-approved and bound to commit/title/body; remote writes use a fixed fork/push/PR flow. No raw state setter or merge endpoint exists. See workflow.md, security.md and api.md.
