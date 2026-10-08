# Contribution workspace refresh

## Direction

Porcelain `#f7f7f4`, white `#ffffff`, stone `#eff0ec`, graphite `#242827`, forest `#25634a`, and amber `#8b5b17`. Dark mode uses charcoal surfaces and soft green accents. Inter remains the interface font; JetBrains Mono is reserved for commands and paths. Green indicates verified success; amber indicates attention; red indicates failure. Primary actions use forest, while navigation and evidence tabs use neutral surfaces.

## Layout and interactions

Repository and issue title lead the page. A compact metadata row exposes the selected Codex model, branch and configuration. A clickable six-stage rail shows completed ticks, current execution, pending work and fixes that were not needed. Below it, a compact execution toolbar leads into the evidence workbench, with live activity and a quieter verification inspector. Resumption approval, extra actions, workspace metadata and historical logs are progressively disclosed.

Stages reflect saved plans and workflow evidence, including repeated test/fix/review cycles. Test ticks are removed when another verification cycle starts. A fix that never ran is labelled Not needed, not completed. Human review stays pending until PR submission. Model identity comes from the contribution's recorded selection or its recorded agent run; unavailable identity stays explicit.

## Implementation milestones

1. Replace the shared theme tokens and remove decorative blue gradients.
2. Recompose the contribution header, stage rail, controls and evidence surfaces while retaining all existing approvals and actions.
3. Improve test-history filtering and output navigation, disclosure of technical details, and live activity.
4. Verify real frontend builds and browser fixtures for completion, reruns, model identity, approval, responsive layout, light/dark appearance and keyboard controls. No contribution is executed by UI verification.

Dependencies remain the existing React, TanStack Query and Phosphor components. No new backend capability or package is needed.

## Verification

The TypeScript/Vite build and 16 browser checks passed, covering persisted model identity and agent fallback, unavailable legacy model IDs, completed/unused stages, reruns and requested changes, test filtering, keyboard tabs, recovery evidence, explicit execution and PR approvals, polling, reduced motion and mobile overflow. Light, dark and mobile captures were visually inspected. A read-only capture of the actual existing contribution confirms the new layout is present at the running frontend; that legacy contribution does not contain a recorded model ID. UI fixtures never execute an external contribution.
