# MASTER BUILD PROMPT — OPEN SOURCE AUTOPILOT

You are the principal engineer, product engineer, and UI engineer responsible for building this project end-to-end.

Build a production-quality local application called:

**Open Source Autopilot**

This is a personal control plane for discovering, ranking, executing, monitoring, testing, reviewing, and preparing open-source contributions.

Do not treat this as a demo, toy dashboard, static prototype, or collection of disconnected scripts.

The final product must be a functional system with a polished interactive dashboard, persistent state, live agent activity, isolated contribution workspaces, GitHub integration, Codex integration, configuration management, testing/review workflows, and an embedded natural-language Operator assistant.

Do not submit pull requests automatically.

The human user must control the two important gates:

1. whether an issue should become a contribution;
2. whether completed code should eventually be submitted as a pull request.

Everything between those gates may be automated.

---

# 0. IMPORTANT WORKING RULES

Before coding:

- inspect the current repository/workspace completely;
- do not destroy or rewrite existing useful work;
- create an architecture document;
- create an implementation plan;
- identify dependencies;
- create milestones;
- then implement them sequentially.

Do not stop after creating a plan.

Continue implementing the product.

Use the available plugins/tools where beneficial:

- @GitHub for GitHub repositories, issues, pull requests, branches, metadata and repository activity;
- @Context7 for current documentation for frameworks/libraries rather than relying on outdated APIs;
- @Figma for dashboard design exploration and implementation fidelity if available;
- @MagicPath for dashboard/interface concept exploration if available;
- @tldraw when useful for architecture/workflow diagrams.

For the dashboard specifically, spend meaningful effort on design.

It must feel like a modern developer control plane combining ideas from:

- GitHub Actions
- Linear
- Vercel
- Kubernetes dashboards
- modern observability tools
- modern AI coding interfaces

Do NOT make it look like a generic Bootstrap admin dashboard.

Prefer a sophisticated dark-first developer aesthetic, excellent information density, restrained colors, good typography, subtle borders, meaningful status colors, responsive layouts, and smooth interactions.

Accessibility and light mode should still work.

---

# 1. CORE PRODUCT PHILOSOPHY

The application has this high-level workflow:

DISCOVERY
    ↓
ANALYSIS
    ↓
RANKING
    ↓
HUMAN SELECTION
    ↓
"PROCEED TO CONTRIBUTE"
    ↓
DEDICATED CONTRIBUTION WORKSPACE
    ↓
CODEX ANALYSIS
    ↓
CODEX IMPLEMENTATION
    ↓
REAL BUILD / TEST / LINT
    ↓
FIX LOOP IF NECESSARY
    ↓
INDEPENDENT CODE REVIEW
    ↓
FIX LOOP IF NECESSARY
    ↓
FINAL REPORT
    ↓
HUMAN REVIEW
    ↓
PREPARE PR
    ↓
HUMAN SUBMITS PR

Discovery and ranking should happen without consuming expensive Codex reasoning whenever possible.

Codex should primarily be used for:

- understanding complex repositories;
- planning an implementation;
- writing code;
- writing tests;
- debugging failures;
- interpreting difficult test failures;
- reviewing contributions.

Normal deterministic software should handle:

- GitHub searching;
- basic filtering;
- metadata extraction;
- ranking arithmetic;
- running commands;
- executing tests;
- persistence;
- scheduling;
- monitoring;
- event logging;
- reports.

---

# 2. HUMAN APPROVAL GATES

There are exactly two major human gates.

## Gate 1 — Start Contribution

The system continuously finds and ranks opportunities.

The user reviews them in the dashboard.

Nothing may modify an external repository merely because an issue ranks highly.

Coding starts ONLY when the user presses:

**Proceed to Contribute**

That action creates an isolated contribution workspace and begins the contribution workflow.

## Gate 2 — Submit Contribution

After coding/testing/review finishes, the contribution reaches:

**READY FOR HUMAN REVIEW**

The application may prepare:

- branch;
- commit;
- patch;
- suggested PR title;
- suggested PR body;
- test evidence.

But DO NOT automatically submit the PR.

The final action must be explicitly initiated by the user.

Keep `auto_create_pr = false` by default.

Never auto-merge.

---

# 3. TECHNOLOGY STACK

Use this stack unless an existing project structure provides a strong reason not to.

## Frontend

- React
- TypeScript
- Vite
- Tailwind CSS
- shadcn/ui
- TanStack Query
- TanStack Table
- React Router
- Monaco Editor / diff viewer
- Recharts or an equivalent mature lightweight charting library
- Lucide icons

Use current stable APIs verified through @Context7.

## Backend / Control Plane

Use Go.

Prefer:

- standard `net/http` or Chi;
- clean package architecture;
- REST APIs;
- Server-Sent Events or WebSockets for live activity;
- background worker orchestration;
- explicit domain/service/repository boundaries without unnecessary enterprise abstractions.

Do not use microservices.

This is a local-first personal application. A modular monolith is appropriate.

## Storage

Start with SQLite.

Design database interfaces cleanly enough that PostgreSQL can be adopted later.

Use migrations.

## Git / Workspace Execution

Use:

- Git
- GitHub CLI where appropriate
- GitHub API
- isolated filesystem workspaces
- child processes with strict working directories

Docker may be used for repository tests when required, but Docker must not be mandatory merely to launch the dashboard.

## AI execution

Integrate Codex through the most appropriate supported local mechanism available in the environment.

Prefer a persistent integration mechanism when live streaming/interaction is needed and a bounded CLI-style execution mechanism for finite background jobs.

Abstract Codex behind an interface such as:

CodexRunner
- Start()
- Resume()
- SendInstruction()
- Cancel()
- GetStatus()
- StreamEvents()

Do NOT tightly couple application business logic to one raw CLI invocation format.

---

# 4. REPOSITORY STRUCTURE

Use a structure approximately like:

open-source-autopilot/
    apps/
        web/
        server/

    internal/
        discovery/
        ranking/
        analyzer/
        estimator/
        contributions/
        agents/
        testing/
        review/
        github/
        config/
        events/
        reporting/
        operator/
        storage/

    contributions/

    configs/

    data/

    docs/
        architecture.md
        workflow.md
        ranking.md
        codex-estimation.md

    scripts/

    .github/

Each contribution must receive its own isolated workspace.

Example:

contributions/
    temporal-8231/
        repo/
        .autopilot/
            metadata.json
            issue.json
            repository.json
            configuration-snapshot.json
            plan.md
            events.jsonl
            commands.jsonl
            review.json
            final-report.md
        artifacts/
            final.patch
            test-output.txt
            review-report.md

Another issue must use another directory:

contributions/
    etcd-19283/
        repo/
        ...

Never allow one contribution agent to accidentally modify another contribution's repository.

Never execute Codex in the Open Source Autopilot control-plane repository when it is supposed to work on an external contribution.

The working directory passed to Codex must explicitly be:

`contributions/<contribution-id>/repo`

---

# 5. CONTRIBUTION LIFECYCLE

Implement a proper persisted state machine.

States:

DISCOVERED

ANALYZING

RANKED

SELECTED

PREPARING

ANALYZING_REPOSITORY

PLANNING

CODING

TESTING

FIXING

REVIEWING

READY

PR_PREPARED

PR_OPENED

PAUSED

BLOCKED

FAILED

ABANDONED

State transitions must be validated.

Do not allow impossible transitions.

Every transition must create an event.

---

# 6. DISCOVERY ENGINE

The Discovery Engine continuously searches for suitable open-source opportunities.

Initial interests should be configurable rather than hardcoded.

Default profile:

Languages:
- Go
- Java
- Python

Domains:
- backend
- distributed systems
- databases
- infrastructure
- AI tooling

Preferred issue labels:
- good first issue
- help wanted
- bug
- enhancement

Default exclusions:
- frontend-only issues
- documentation-only work

Search GitHub using appropriate APIs.

Collect at minimum:

- owner/repository;
- issue number;
- title;
- description;
- labels;
- language;
- repository topics;
- stars;
- forks;
- repository archived status;
- last commit;
- release activity;
- issue creation/update times;
- assignees;
- discussion/comment count;
- maintainer participation;
- matching open PRs;
- likely competing contribution;
- contribution guidelines availability;
- CI availability;
- test infrastructure presence.

Respect GitHub rate limits.

Cache results appropriately.

Do not repeatedly query unchanged data unnecessarily.

---

# 7. REPOSITORY ANALYZER

Analyze candidate repositories without Codex whenever possible.

Check:

- repository active or stale;
- archived status;
- CONTRIBUTING.md;
- AGENTS.md;
- CODE_OF_CONDUCT;
- build system;
- language;
- CI;
- tests;
- recent commits;
- recent releases;
- external PR acceptance;
- maintainer responsiveness;
- issue clarity;
- possible duplicate work;
- existing PR for issue;
- issue already assigned;
- issue potentially obsolete.

Produce explainable analysis.

Every score shown in the UI must have a reason.

---

# 8. RANKING ENGINE

Ranking determines:

**How good is this contribution opportunity?**

Ranking MUST NOT include Codex token usage, Codex allowance usage, estimated AI cost, or expected AI effort.

This requirement is absolute.

Codex usage estimation is informational only.

Never incorporate it into:

- contribution score;
- ranking order;
- merge probability;
- career score;
- opportunity quality score.

Suggested ranking factors:

- language / skill match;
- domain match;
- repository health;
- maintainer activity;
- issue clarity;
- merge probability;
- learning value;
- career / portfolio value;
- contribution usefulness;
- difficulty suitability;
- competition / existing PR risk.

Return an overall score from 0–100.

Also preserve all component scores.

The UI must show why an opportunity received its score.

---

# 9. CODEX USAGE / EFFORT ESTIMATOR

This is a separate subsystem.

Purpose:

Before the user presses "Proceed to Contribute", estimate how expensive/difficult the contribution may be for Codex.

This DOES NOT CHANGE THE RANKING.

Estimate based on signals such as:

- repository size;
- relevant module size;
- probable files involved;
- issue description complexity;
- architectural breadth;
- language complexity;
- build complexity;
- test complexity;
- integration test requirements;
- concurrency/distributed-state involvement;
- debugging uncertainty;
- likely number of coding iterations;
- expected reviewer/fix cycles;
- historical contribution data.

Return:

- effort category:
  VERY_LOW
  LOW
  MEDIUM
  HIGH
  VERY_HIGH

- estimated coding iterations range;
- estimated context size;
- estimated relevant files;
- estimated test/debug complexity;
- confidence score;
- explanation.

If an honest estimate of actual Plus allowance percentage cannot be obtained programmatically, DO NOT fabricate it.

Instead display something like:

"Estimated Codex Effort: Medium"

and optionally:

"Approximate allowance impact: unavailable / user calibration required"

Design the estimator so historical completed jobs can later calibrate this.

Store estimated vs actual observed metrics for every contribution.

Over time calculate personalized historical statistics.

Example:

Estimate:
MEDIUM

Expected cycles:
3–6

Expected relevant files:
8–20

Confidence:
74%

Reasons:
- multi-module Go repository;
- existing tests available;
- issue scope fairly clear;
- concurrency code involved.

Again:

**THIS VALUE MUST NEVER ALTER CONTRIBUTION RANKING.**

It may be displayed, filtered, or separately sorted by the user.

---

# 10. PROCEED TO CONTRIBUTE

Every ranked opportunity must contain:

**Proceed to Contribute**

When pressed:

1. create a unique contribution ID;
2. create a dedicated project directory;
3. save the issue snapshot;
4. save repository metadata;
5. snapshot the active Autopilot configuration;
6. clone the external repository into `repo/`;
7. create a dedicated Git branch such as:
   `autopilot/issue-8231`;
8. inspect repository contribution instructions;
9. create an execution record;
10. launch Codex inside ONLY that project's `repo/` directory;
11. stream lifecycle events to the dashboard.

Never reuse another contribution workspace.

---

# 11. CODEX CONTRIBUTOR AGENT

Codex Contributor receives:

- issue;
- issue discussion;
- repository metadata;
- CONTRIBUTING instructions;
- AGENTS instructions;
- active constraints;
- dedicated working directory.

Its job:

1. understand repository architecture;
2. identify relevant code;
3. understand the issue;
4. inspect related tests;
5. inspect similar implementations;
6. generate a concise implementation plan;
7. implement the smallest correct change;
8. avoid unrelated refactoring;
9. add meaningful tests;
10. run appropriate verification;
11. debug failures;
12. keep an audit trail.

The contributor may abandon or mark BLOCKED if the issue becomes invalid, already solved, ambiguous, impossible, or unexpectedly inappropriate.

It must not force a contribution merely because the user started one.

---

# 12. CONTRIBUTION PLAN

Before editing code, store:

`.autopilot/plan.md`

Include:

- problem summary;
- likely root cause;
- affected modules;
- likely files;
- implementation strategy;
- tests to add/run;
- risks;
- unknowns.

Default behavior:

User has already pressed Proceed, so Codex may automatically continue from plan to implementation.

Add a configuration option:

`require_plan_approval`

Default:
false

If enabled, pause after planning.

---

# 13. TESTING ENGINE

Testing must be real execution, not an LLM claiming tests pass.

Detect project ecosystem.

Examples:

Go:
- go test ./...
- go vet ./...
- repository-specific commands

Java:
- ./mvnw test
- ./mvnw verify
or Gradle equivalents

Python:
- pytest
- configured lint/type tools

Node:
- package-defined test/lint/build scripts

Respect CONTRIBUTING instructions.

Store:

- command;
- working directory;
- start/end time;
- exit code;
- stdout/stderr;
- passed/failed counts if parseable.

Display all test runs in the dashboard.

If tests fail:

TESTING
    ↓
FIXING
    ↓
Codex Contributor
    ↓
TESTING

Bound retry loops.

Do not infinite-loop.

---

# 14. INDEPENDENT REVIEW AGENT

The same Codex conversation that wrote the implementation should not simply declare its own code correct.

Launch a separate review context.

Reviewer receives:

- original issue;
- repository instructions;
- implementation plan;
- git diff;
- modified files;
- relevant surrounding files;
- test results.

Review:

- correctness;
- issue compliance;
- missing edge cases;
- regressions;
- concurrency;
- security;
- performance;
- compatibility;
- repository conventions;
- test quality;
- unnecessary modifications;
- API changes;
- architecture concerns.

Reviewer output:

APPROVE

or

REQUEST_CHANGES

Every finding must have:

- severity;
- file;
- line/range if possible;
- explanation;
- recommended fix.

If REQUEST_CHANGES:

REVIEWING
    ↓
FIXING
    ↓
Contributor
    ↓
TESTING
    ↓
REVIEWING

Bound this loop.

If the reviewer repeatedly rejects the same issue, surface it to the human.

---

# 15. FINAL REPORT

When review passes, generate a final report.

Include:

Repository

Issue

Contribution ID

Original contribution ranking

Estimated Codex effort

Observed execution metrics

Files changed

Lines added/deleted

Implementation summary

Tests run

Results

Review summary

Known risks

Commands executed

Branch

Suggested commit message

Suggested PR title

Suggested PR description

Status:

READY FOR HUMAN REVIEW

The dashboard must make this report pleasant to inspect.

---

# 16. DASHBOARD — PRODUCT QUALITY IS CRITICAL

Build a visually excellent dashboard.

Do not make the dashboard an afterthought.

Use plugins/design tools if available before settling on the final interface.

The primary desktop layout should contain:

- persistent sidebar;
- top status bar;
- main workspace;
- accessible Operator assistant;
- realtime indicators.

Responsive behavior should work for laptop-sized screens.

---

# 17. DASHBOARD NAVIGATION

Include approximately:

Overview

Opportunities

Contributions

Agents

Activity

Usage

Configuration

Settings

Operator

Avoid clutter.

---

# 18. OVERVIEW PAGE

Show useful live metrics such as:

Repositories scanned

Issues discovered

High-quality candidates

Active contributions

Testing contributions

Reviewing contributions

Ready for review

Opened PRs

Agent health

Discovery status

Current configuration profile

Recent activity

Opportunity pipeline

Contribution pipeline

The page should answer:

"What is my system doing right now?"

within a few seconds.

---

# 19. OPPORTUNITIES PAGE

This is one of the most important pages.

Show a sophisticated sortable/filterable ranking table/cards.

Columns/details:

Rank

Repository

Issue

Language

Domain

Contribution score

Merge probability

Difficulty

Maintainer activity

Learning value

Career value

Estimated Codex effort

Estimate confidence

Status

Actions

Default sort:

Contribution score.

Remember:

Codex effort MUST NOT influence that score.

Allow optional user sorting/filtering by effort without changing canonical ranking.

Each opportunity has:

View Analysis

Ignore

Watch

Proceed to Contribute

Clicking an opportunity opens detailed analysis.

---

# 20. OPPORTUNITY DETAIL

Show:

Issue summary

Repository health

Contribution score

Score breakdown

Why it ranked here

Maintainer information

Repository activity

Duplicate/open PR check

Likely files/modules

Difficulty

Risk factors

Estimated Codex effort

Effort explanation

Estimate confidence

Proceed button

Make information understandable rather than dumping raw metadata.

---

# 21. CONTRIBUTIONS PAGE

Show cards/table with live state:

Temporal #8231 — CODING

etcd #19283 — TESTING

Gitea #8281 — READY

TiDB #11382 — BLOCKED

Kafka #3382 — ABANDONED

Each opens the Contribution Workspace view.

---

# 22. CONTRIBUTION WORKSPACE VIEW

Tabs:

Overview

Plan

Files / Changes

Tests

Review

Timeline

Report

Workspace

## Overview

Show:

status;

progress;

issue;

ranking snapshot;

Codex estimate;

branch;

elapsed time;

files changed;

tests;

review;

current agent action.

## Files / Changes

Use Monaco or equivalent.

Show:

file tree;

changed files;

side-by-side diff;

unified diff;

additions/deletions;

selectable files.

## Tests

Show every test run.

Example:

Build ✓

Unit Tests 1842 / 1842 ✓

Integration Tests 93 / 93 ✓

Lint ✓

Race Detector ✓

Previous attempts should remain visible.

## Review

Show findings grouped:

Critical

Major

Minor

Suggestion

Allow:

Send Back to Contributor

Ignore Finding

Open Code

## Timeline

Detailed realtime activity such as:

14:21 Agent started

14:22 Repository analyzed

14:24 Read retry.go

14:25 Search RetryPolicy

14:27 Modified retry.go

14:28 Added retry_test.go

14:30 Tests failed

14:32 Investigating failure

14:38 Tests passed

14:40 Reviewer started

14:44 Reviewer requested changes

Do not expose hidden chain-of-thought.

Only expose legitimate operational observability:

- actions;
- tool calls;
- file access;
- summaries;
- decisions;
- commands;
- outputs;
- state changes;
- user-visible agent notes.

---

# 23. AGENT MONITOR

Create a dedicated live page.

For each agent show:

Agent ID

Agent type

Contribution

Repository

Current state

Current action

Runtime

Files read

Files changed

Commands run

Test attempts

Last event

Controls:

Inspect

Pause

Resume

Stop

Do not provide fake activity.

Everything shown must originate from actual events.

---

# 24. EVENT SYSTEM

Everything important must generate a structured event.

Examples:

AgentStarted

AgentStopped

RepositoryScanned

IssueDiscovered

IssueAnalyzed

IssueRanked

ContributionCreated

WorkspaceCreated

RepositoryCloned

BranchCreated

PlanCreated

FileRead

FileModified

CommandStarted

CommandCompleted

TestStarted

TestFailed

TestPassed

ReviewStarted

ReviewFindingCreated

ReviewPassed

ContributionBlocked

ContributionReady

ConfigChangeProposed

ConfigChanged

ContributionPaused

ContributionResumed

ContributionAbandoned

PRPrepared

PROpened

Persist events.

Stream them to the frontend via SSE/WebSocket.

Use events to power Activity and Timeline views.

---

# 25. OPERATOR CHAT ASSISTANT

The dashboard must contain a high-quality embedded chat assistant called:

**Operator**

Operator is the natural-language control interface to Open Source Autopilot.

It can answer questions such as:

"What are my agents doing?"

"Why was this issue ranked #4?"

"Why was Kubernetes #123 rejected?"

"What failed in the Temporal contribution?"

"Which contributions are ready?"

"What are my current search languages?"

It can also accept commands such as:

"Start looking for Rust projects."

"Add C++."

"For the next two weeks prioritize ML infrastructure."

"Stop searching Spring Boot."

"Only show medium difficulty issues."

"Pause all contributors."

"Resume the Temporal contribution."

"Abandon this contribution."

"Retry the tests."

"Send this back to reviewer."

"Do not let this contribution modify the public API."

"Prepare the PR but don't submit it."

---

# 26. OPERATOR TOOL MODEL

Operator must NOT have arbitrary unrestricted shell access.

Expose controlled domain operations.

For example:

getConfig

proposeConfigChange

applyConfigChange

getAgentStatus

pauseAgent

resumeAgent

stopAgent

getOpportunity

changeOpportunityState

startContribution

getContribution

pauseContribution

resumeContribution

abandonContribution

addContributionConstraint

runTests

requestReview

preparePR

getActivity

getUsageMetrics

Operator must map language to these operations.

Do not let Operator directly execute arbitrary shell commands.

---

# 27. NATURAL-LANGUAGE CONFIGURATION

Example user message:

"For the next two weeks include Rust, focus on databases and distributed systems, and stop searching frontend issues."

Operator should return a proposed diff.

Example:

languages:
+ Rust

domains:
+ databases
+ distributed-systems

exclude:
+ frontend

duration:
  until: ...

Then show:

Cancel

Apply Changes

Do not silently mutate important project configuration.

Validate changes.

Persist configuration versions.

---

# 28. CONFIGURATION VERSIONING

Configuration should be versioned.

Example:

config v37 → v38

Changed by:
Operator / User

Reason:
"Focus on Rust distributed systems this weekend"

Allow rollback.

Store a snapshot of configuration inside each contribution when it starts.

This ensures historical reproducibility.

---

# 29. CONFIGURATION MODEL

Support configuration approximately like:

profile:
  languages:
    Go: 1.0
    Java: 0.8
    Python: 0.7

  domains:
    distributed-systems: 1.0
    backend: 1.0
    databases: 0.9
    ai-infrastructure: 0.8

repositories:
  min_stars: 0
  require_recent_activity_days: 60

issues:
  preferred_labels:
    - good first issue
    - help wanted
    - bug

  difficulty:
    - easy
    - medium

agents:
  max_scouts: 4
  max_contributors: 2
  max_reviewers: 2

contributions:
  require_plan_approval: false
  auto_prepare_pr: false
  auto_create_pr: false
  auto_merge: false

codex:
  max_fix_iterations: 5
  max_review_cycles: 3

Do not hardcode these values throughout the application.

Use proper configuration services.

---

# 30. CODEX USAGE PAGE

Create a Usage page.

However, be honest about data.

Do not fabricate exact ChatGPT Plus usage if OpenAI does not expose it programmatically.

Display separately:

Observed local execution metrics:
- Codex sessions;
- durations;
- model requests if observable;
- context/token data if actually available;
- number of iterations;
- contributor cycles;
- reviewer cycles.

Estimated effort:
- initial estimate;
- actual complexity observed;
- estimate accuracy.

If official remaining plan usage can be retrieved safely, display it.

Otherwise provide a clearly labeled place where the user may manually enter or update remaining allowance information.

Never show guessed data as official.

---

# 31. CHAT + CONFIG UI

Operator chat should be visually integrated into the dashboard.

Desktop idea:

main dashboard content | Operator side panel

Allow expanding Operator into a larger view.

Messages affecting configuration should render structured cards/diffs instead of plain text only.

For destructive actions such as abandoning active contributions, request confirmation.

---

# 32. LIVE CONTROL

From dashboard:

Pause Contribution

Resume Contribution

Stop Agent

Abandon Contribution

Retry Tests

Request Review

Send Back to Contributor

Prepare PR

Open Workspace

These controls must work.

Do not build decorative buttons with no implementation.

---

# 33. GITHUB SAFETY

Never:

- auto-submit unknown changes;
- auto-merge;
- modify upstream default branches;
- force push external repositories;
- expose credentials in logs;
- run untrusted repository scripts outside a controlled context without considering safety.

Keep GitHub tokens/secrets out of database/event logs.

Use environment variables or appropriate credential helpers.

---

# 34. COMMAND EXECUTION SECURITY

External repositories are untrusted code.

Create a command execution abstraction.

Track:

- command;
- args;
- working directory;
- timeout;
- environment;
- result.

Use timeouts.

Prevent directory traversal outside the contribution workspace.

Do not interpolate arbitrary strings into shell commands when structured process arguments can be used.

Design for future container sandboxing.

---

# 35. DATABASE

Define entities roughly covering:

repositories

issues/opportunities

ranking_results

contributions

contribution_files

agents

agent_runs

events

test_runs

review_runs

review_findings

config_versions

codex_estimates

execution_metrics

reports

pr_records

Use appropriate indexes.

Do not store giant raw repository contents in SQLite.

Store filesystem references when appropriate.

---

# 36. API

Create clean endpoints approximately covering:

/api/overview

/api/opportunities

/api/opportunities/:id

/api/opportunities/:id/proceed

/api/contributions

/api/contributions/:id

/api/contributions/:id/pause

/api/contributions/:id/resume

/api/contributions/:id/abandon

/api/contributions/:id/tests

/api/contributions/:id/review

/api/contributions/:id/prepare-pr

/api/agents

/api/events

/api/config

/api/config/proposals

/api/config/history

/api/operator/chat

/api/usage

Use appropriate HTTP semantics.

Document the API.

---

# 37. UI DESIGN REQUIREMENTS

The dashboard must be polished enough that I enjoy leaving it open while agents work.

Prioritize:

clear hierarchy;

developer-focused aesthetics;

dense but readable information;

great tables;

good empty states;

skeleton loaders;

error states;

tooltips;

keyboard accessibility;

responsive layout;

status badges;

subtle motion;

live indicators;

excellent diffs;

timeline readability;

command/test output readability.

Use a consistent design system.

Avoid excessive gradients.

Avoid giant marketing-style cards.

Avoid excessive rounded cards inside rounded cards.

Avoid childish icons.

Avoid neon cyberpunk styling.

Prefer serious developer-tool quality.

---

# 38. DESIGN FIRST

Before finalizing the frontend:

Use available design capabilities such as @Figma or @MagicPath to explore the interface.

Create/compare at least 2–3 strong dashboard directions internally.

Choose the one best suited to:

- long monitoring sessions;
- information density;
- agent observability;
- contribution review;
- code/test inspection.

Then implement consistently.

Do not randomly redesign every page independently.

---

# 39. EMPTY / DEMO DATA

Implement a development/demo seed mode.

I should be able to run the dashboard before connecting GitHub/Codex and see realistic sample data.

But clearly identify seeded/mock data in development mode.

Production mode must use real persisted events.

---

# 40. LOCAL DEVELOPMENT EXPERIENCE

Provide a simple startup workflow.

Prefer something like:

make dev

or

./scripts/dev.sh

or an equally convenient cross-platform solution.

Document:

requirements;

environment variables;

GitHub authentication;

Codex authentication;

frontend startup;

backend startup;

database migrations;

testing.

The application should be practical to run locally on Windows with WSL/Linux tooling where required.

---

# 41. TESTS FOR OPEN SOURCE AUTOPILOT ITSELF

Add tests for:

ranking;

configuration changes;

state transitions;

workspace creation;

path isolation;

Codex effort estimator;

event persistence;

Operator tool dispatch;

GitHub parsing;

API handlers;

critical frontend interactions where practical.

The ranking test must explicitly prove:

Changing Codex usage/effort estimate does NOT change the opportunity ranking score.

This is a non-negotiable regression test.

---

# 42. IMPLEMENTATION ORDER

Build iteratively in this order:

Phase 1
Foundation
- repository structure
- Go backend
- React frontend
- SQLite
- migrations
- config
- event system

Phase 2
Discovery
- GitHub integration
- repo analyzer
- issue analyzer
- ranking
- opportunity dashboard

Phase 3
Dashboard quality
- polished overview
- opportunity detail
- filtering/sorting
- realtime event infrastructure

Phase 4
Contribution workspaces
- workspace manager
- clone
- branch
- metadata
- isolated command runner

Phase 5
Codex integration
- contributor runner
- streaming
- plans
- state transitions

Phase 6
Testing
- detection
- execution
- logs
- retry/fix cycle

Phase 7
Reviewer
- independent review
- findings
- fix/review loop

Phase 8
Operator
- chat UI
- tool interface
- config proposals
- apply/rollback

Phase 9
Reporting / PR preparation
- final report
- diff viewer
- PR text generation
- GitHub preparation
- HUMAN submit gate

Phase 10
Polish
- accessibility
- responsive design
- error handling
- onboarding
- docs
- integration tests

Do not attempt to fake later phases with static UI.

---

# 43. ACCEPTANCE SCENARIO

The final system should support this exact scenario:

1. I launch Open Source Autopilot.

2. Discovery finds open-source issues.

3. Dashboard shows ranked opportunities.

4. I see:

Repository:
etcd

Issue:
#12345

Contribution Score:
94 / 100

Difficulty:
Medium

Merge Probability:
High

Estimated Codex Effort:
Medium

Confidence:
78%

5. Codex effort has NOT affected the 94/100 ranking.

6. I inspect the issue.

7. I press:

Proceed to Contribute

8. System creates:

contributions/etcd-12345/repo

9. Repository is cloned there.

10. A dedicated branch is created.

11. Codex runs only inside that repo directory.

12. Dashboard updates live:

PREPARING

ANALYZING

PLANNING

CODING

TESTING

13. Tests fail.

14. Dashboard displays the failure.

15. Codex receives the failure and fixes it.

16. Tests pass.

17. Independent Reviewer starts.

18. Reviewer requests changes.

19. Contributor fixes them.

20. Tests run again.

21. Reviewer approves.

22. Contribution becomes:

READY FOR HUMAN REVIEW

23. I can inspect:

plan;

files;

diff;

tests;

review;

timeline;

final report.

24. System prepares a suggested PR title/body.

25. It DOES NOT submit automatically.

26. I make the final decision.

This entire flow must work.

---

# 44. OPERATOR ACCEPTANCE SCENARIO

While the system is running I type:

"I want to contribute to Rust too. Add Rust, prioritize databases and distributed systems, and stop looking for frontend issues."

Operator should:

1. understand the request;
2. read current config;
3. create a structured proposed diff;
4. display affected settings;
5. wait for Apply;
6. validate;
7. create a new config version;
8. emit ConfigChanged;
9. update discovery/ranking behavior;
10. preserve history.

I should then be able to say:

"Why did you rank this issue #3?"

and receive an explanation grounded in stored ranking components.

I should also be able to say:

"Pause the Temporal contribution."

and the actual contribution pauses while preserving workspace/state.

---

# 45. QUALITY BAR

Do not consider this project finished because:

- the server compiles;
- a dashboard page renders;
- static fake cards exist;
- Codex can be launched once.

The system is finished only when the workflow is coherent and usable end-to-end.

Prioritize robustness over premature complexity.

Avoid unnecessary distributed infrastructure.

This is a local-first application for one primary user.

Use modular architecture but do not overengineer.

---

# 46. DOCUMENTATION

Maintain:

README.md

docs/architecture.md

docs/workflow.md

docs/ranking.md

docs/codex-estimation.md

docs/security.md

docs/operator.md

README should explain:

what the system does;

how to run it;

how GitHub auth works;

how Codex integration works;

where contribution workspaces are stored;

what actions require human approval.

---

# 47. WORKING BEHAVIOR WHILE BUILDING

As you implement:

- run formatters;
- run tests;
- run builds;
- inspect errors;
- fix them;
- do not leave known compile failures;
- do not silently disable failing functionality;
- do not replace difficult features with fake placeholders without clearly marking them;
- keep TODOs only where external constraints genuinely block implementation.

After each major milestone, briefly record:

Completed

Tests run

Remaining

Architecture changes

Then continue.

---

# 48. FINAL DELIVERY

When the implementation is complete, provide:

1. architecture summary;
2. directory structure;
3. exact startup commands;
4. environment variables;
5. current implemented features;
6. any external setup still required;
7. tests/build results;
8. screenshots or visual verification of major dashboard pages if your environment permits;
9. known limitations;
10. next recommended improvements.

Most importantly:

Build the application.

Do not merely explain how to build it.