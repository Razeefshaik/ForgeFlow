# Application root and repository relocation

ForgeFlow itself lives directly at the main project root. External repositories use `<ConfiguredContributionsRoot>/<contribution-id>/repo`, created only after approval. This installation uses `S:/StudyResource/TechBoooo/Backend/Contriss`; the application contributions/ directory contains only .gitkeep.

```text
ForgeFlow/
  .git/
  apps/server/
  apps/web/
  internal/             backend modules, embedded migrations and tests
  docs/
  scripts/
  data/
  artifacts/
  bin/
  contributions/.gitkeep
  AGENTS.md
  SPEC.md
  README.md
  go.mod
  go.sum
  package.json
```

## Preserved implementation
Moved apps, internal, docs, scripts, data, artifacts, bin, dependency/cache directories, .gitignore, Go module/sums, package.json and README from the obsolete nested application. Before edits, SHA-256 checks verified all 73 tracked/untracked source files and all 25 database/artifact/binary files after relocation. Existing node_modules, lockfile, dist and browser test outputs were moved intact. Embedded migration paths remain unchanged.

Root AGENTS.md and its nested copy were byte-identical. Root SPECS.md and nested SPEC.md were byte-identical. The root specification was renamed SPEC.md without changing its bytes. Identical nested copies remain in ignored recovery storage. AGENTS.md retains its safety rules and now states the application/external-workspace boundary.

## Git history
Retained the original root .git, master branch and origin remote. Nested commit 56fa5f0 is reachable through local branch `archive/forgeflow-foundation`; its complete history is also in `.cache/restructure/nested-history.bundle`. The old nested Git metadata is preserved in a verified ZIP and `original-nested-git-metadata` archive directory there, rather than deleting recoverable metadata. The archive directory is not named .git and is not an active nested repository.

The obsolete gitlink was removed from the root index. No history was reset or pushed. The main repository has one active .git; future external clones may intentionally have their own .git directories. Recovery storage is ignored and stays outside contributions.

## Path rules
Startup/build scripts derive the application root from their script file location. The server uses `--root` when specified, otherwise its executable location and working directory. Relative --db/--web arguments resolve against the root; absolute overrides are preserved. Data defaults are data/demo.db or data/forgeflow.db. Migrations are embedded, not loaded from the process working directory.

Screenshot capture derives root/artifacts from its module path. External paths use `contributions.WorkspacePathAt(configuredRoot, id)` with validated IDs. Preparation/execution separately check resolved links, exclusive allocation and sandbox permissions. Current paths may change through audited relocation; historical approval paths remain intact.

## Run
From the main repository root, run `npm run dev` and open http://127.0.0.1:5173. `npm run dev` uses live discovery; use `npm run dev:demo` for illustrative data. On a fresh checkout run `npm run setup` first. The moved installed dependencies can be reused after this relocation.

## Relocation verification — 2026-10-06
- Go formatting completed. Root `npm run build` passed all 31 Go tests, vet, the server binary build, TypeScript and the Vite production build.
- `npm ls --depth=0` validated the moved frontend dependency tree; reinstall was unnecessary.
- Actual `npm run dev` from the root served frontend HTTP 200 on port 5173 and demo backend health on port 8080, using root data/demo.db. The owned dev session was then stopped.
- Existing browser suite: four passed, two opt-in live checks skipped. The live evidence browser check was additionally run against the preserved QA live database and passed. The network-triggering live Run check was not repeated for this filesystem relocation.
- Binary smoke checks launched from apps/web and .cache, with relative database flags. Both located the main root; the live SPA /configuration returned HTTP 200, and no apps/web/data or .cache/data directory was created.
- The relocated live database retained all three embedded migration records, 17 audit events, one config version and five saved opportunities. QA demo browser changes append versions/events rather than deleting history.
- SPEC.md retains its original SHA-256. Pre-edit move verification matched 73 source files and 25 runtime/artifact files. Previous PNGs also have recovery copies under .cache/restructure/original-screenshots before browser captures were refreshed.
- Directory scan found only root .git. contributions contains only .gitkeep. The obsolete workspace/gitlink is removed. Root master/origin and both root history and imported nested history remain available; no reset, push or PR occurred.
- Git diff whitespace check passed. Temporary QA/dev processes were stopped after checks. No relocation blockers remain; application execution/review/PR pipeline limitations are unchanged.
