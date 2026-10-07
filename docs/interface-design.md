# ForgeFlow interface design

The frontend uses the Apple Design and Emil Design Engineering project skills as guidance. Shared tokens and responsive component styles live in `apps/web/src/design.css`, loaded after the existing functional stylesheet.

- Light is the default for new browsers. Existing saved theme preferences remain respected.
- Neutral surfaces, restrained blue accents, system typography and consistent rounded controls establish the visual system.
- Overview prioritizes actionable contributions, followed by ranked opportunities and audit activity.
- Contribution links use `/contributions?contribution=<id>` so a workspace survives reload and can be bookmarked.
- Workspace details are collapsed by default. Stage indicators show the persisted current state; they do not invent completion percentages.
- The existing execution, tests, independent review and human approval actions retain their API contracts.
- Pointer-opened dialogs have a short entrance transition. Keyboard navigation and reduced-motion preferences disable it.
- Mobile navigation closes even when choosing the current page. Long repository names and paths wrap inside the workspace.

## Verification

Run `npm --prefix apps/web run build` and `npm --prefix apps/web test` with the frontend and an isolated demo backend running. Browser coverage includes discovery, approvals, account setup, execution evidence, live polling, mobile theme changes, contribution deep links, reload behavior, long data and reduced motion. Live GitHub discovery checks are skipped in demo mode.

Visual previews are saved in `artifacts/redesign-*.png`. These use illustrative demo data; no contribution agent was started for UI verification.
