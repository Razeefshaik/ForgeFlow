# ForgeFlow architecture

## Application root and external workspaces
ForgeFlow's apps, internal packages, scripts, documentation and runtime directories live directly in the main repository root. SPEC.md is the canonical specification, normalized from the identical supplied SPECS.md. The requested ForgeFlow name refers to the specification's Open Source Autopilot product.
The root .git owns the application. External repositories belong only under <ForgeFlowRoot>/contributions/<contribution-id>/repo after Proceed to Contribute. The directory initially contains only .gitkeep. The earlier nested application was relocated intact; its history is retained in archive/forgeflow-foundation and ignored recovery backups.
Startup scripts derive the root from their own location. The server locates it from an explicit --root, its executable location, or its working directory. Relative database/frontend flags resolve against this root. Embedded SQLite migrations remain packaged with the server. The external workspace path helper uses the same absolute project root; it does not create or execute contributions.

## Implemented foundation
React + TypeScript + Vite frontend, Tailwind v4, TanStack Query, TanStack Table, React Router, Lucide, accessible Radix primitives following shadcn conventions. A single Go net/http service owns SQLite migrations, domain services, configuration versions, append-only events and SSE replay. No Docker or microservices.
SQLite uses modernc.org/sqlite (no CGO), foreign keys, WAL, busy timeout and one connection. Domain changes and their events commit in the same transaction. Migration versions and immutable audit triggers are persisted. Both database mode and API mode are fixed at initialization: demo data never leaks into a live database.
Packages: domain defines JSON contracts; ranking accepts only quality components; estimator has a separate output; storage manages transactions; config validates settings; contributions validates lifecycle transitions; operator dispatches allowlisted read/proposal actions; api owns HTTP/SSE; seed exists only in explicit demo mode.
github provides bounded GET-only REST access, credentials, cache and backoff. analyzer converts observed metadata into explained quality factors. discovery snapshots configuration, schedules/cancels scans and persists observations with audit events. See discovery.md for public-only scope and sampling limits.
Config proposals carry a base version. Apply fails on stale versions, makes a new immutable version and emits ConfigChanged. Rollback creates another version. Safety gates cannot be enabled through configuration.

## Events
An ordered SQLite event ID is the SSE cursor. Clients reconnect using Last-Event-ID; replay polls committed records, so there is no in-memory publication race. Heartbeats maintain the connection. History is never overwritten or deleted. UI query caches invalidate on events.

## Execution boundary
The foundation does not launch Codex, clone external repositories, execute contribution tests or submit PRs. Proceed is visibly unavailable until the isolated workspace/execution phases exist. Persisted demo contribution states are explicitly illustrative. No test pass counts, review approvals or model usage are invented.
Later services must allocate contributions/<id>/repo, enforce resolved paths including symlinks, snapshot config/issue/ranking, use structured process arguments with timeouts, and run Codex in only that repo. A working directory alone is not a security sandbox. Contributor and reviewer contexts must be independent.

## API boundary and local security
Loopback bind only, exact same-origin writes with JSON, bounded request bodies, strict decoding, errors without secrets, context-aware SQL, graceful shutdown. Vite proxies API/SSE during development; Go serves the built SPA for a single-origin production-style launch. This local application is not designed for public hosting.
GitHub authentication belongs to the server adapter; Codex authentication is pending. Plugin connections in this chat do not automatically authenticate the running application.

## Delivery sequence
1. Foundation: structure, docs, database, configuration, audit events, lifecycle, explicit demo mode, polished UI.
2. Discovery: authenticated GitHub search/caching, repository and issue analysis, persisted explainable ranking.
3. Dashboard refinement: detailed filters, telemetry, code/test panels.
4. Isolated workspaces and command runner.
5. Codex contributor adapter.
6. Real test detection and bounded fixes.
7. Independent reviewer context.
8. AI-backed Operator using controlled actions (foundation offers deterministic inspection/proposals only).
9. Final reports and human-approved PR preparation/submission.
10. Accessibility, integration coverage and hardening.

## Dependencies
Go 1.26, Node 24 LTS, npm, Git. Backend: modernc SQLite. Frontend lockfile pins installed versions. Browser QA uses Playwright. Framework setup was checked with Context7. MagicPath design tokens informed the documented visual exploration.

Installed TanStack Table v9 provides its documented legacy adapter for v8-style controlled sorting. The foundation uses this explicit adapter and pins the installed version; APIs were also checked against installed declaration files. New code should migrate to v9 feature-based useTable when expanding table capabilities. See https://tanstack.com/table/v8/docs/framework/react/react-table for the earlier API contract.

