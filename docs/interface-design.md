# ForgeFlow interface design

The supplied `forgeflow-frontend.zip` is the visual and interaction reference. Its simulated records are reference material; this implementation uses React and TypeScript connected to the existing Go API. Shared tokens and responsive component styles live in `apps/web/src/design.css`.

- New browsers follow system appearance. Light, dark and system preferences persist.
- Palette anchors: `#121D49` navy, `#C38EB4` mauve, `#E1CBD7` blush, `#86A8CF` blue, `#26425A` steel and `#0D1F2D` ink. Rounded panels, soft borders and compact typography follow the ZIP. Inter and JetBrains Mono are self-hosted through Fontsource. Color alone never denotes execution status.
- Overview prioritizes actionable contributions, followed by ranked opportunities and audit activity.
- Contribution details are dedicated pages at `/contributions/<id>` with a back link to the list. Old `/contributions?contribution=<id>` bookmarks redirect to the new page. Direct entry and reload fetch the individual contribution and preserve live polling, evidence and approval controls.
- Workspace details are collapsed by default. Stage indicators show the persisted current state; they do not invent completion percentages.
- The existing execution, tests, independent review and human approval actions retain their API contracts.
- The local emil-design-eng, animate and mobile-native skills inform CSS page/evidence transitions, card lift, button press, drawer motion, transform-based score fills, loading skeletons and ongoing execution indicators. Keyboard-driven page and dialog changes are immediate. Reduced motion disables animation and leaves static activity indicators. No timeline animation repeats on polling.
- Mobile navigation closes even when choosing the current page. Long repository names and paths wrap inside the workspace.

## Screens and evidence

The grouped sidebar and sticky toolbar separate application connectivity from GitHub authentication. Opportunities use a searchable ranked list and inline evidence inspector; compact overview inspection retains its accessible dialog. Contributions have search/state filters. The workspace shows live activity, six evidence tabs, measured command history, a collapsible inspector with copy controls and audit history.

Agents use real session cards with filters, output and explicit stop controls. Activity searches and filters persisted events without modifying them. Configuration retains the versioned JSON editor, proposals, diffs, apply and restore controls alongside applied-profile summaries. Settings groups appearance, workspace, execution, GitHub and approval boundaries. GitHub retains device sign-in, setup and logout. Operator uses an accessible side drawer with its existing controlled actions.

Verification percentages count successful completed command runs, including earlier attempts; they do not assert final approval. Ongoing execution has no guessed completion percentage. Usage distinguishes recorded tokens/session time, heuristic estimates and unavailable allowance.

Mobile uses safe areas, dynamic viewport height and 16px inputs. Browser checks use emulation; real phone hardware has not been validated.

## Assets

- [Phosphor React](https://github.com/phosphor-icons/react): MIT licensed duotone SVG icons, installed locally with individual imports. CPU, stack, compass and branch symbols replace robot/sparkle imagery.
- Inter and JetBrains Mono: Fontsource packages under the SIL Open Font License; licenses accompany dependencies.
- [unDraw](https://undraw.co/license): downloaded Version Control illustration at `apps/web/public/images/version-control.svg`, recolored to blue. Source: `https://42f2671d685f51e10fc6-b9fcecea3e50b3b59bdc28dead054ebc.ssl.cf5.rackcdn.com/illustrations/version_control_9bpv.svg`.
- [GitHub mark](https://github.com/logos): downloaded from `https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png` for account identification.
- Optional public repository-owner avatars include a local branch-icon fallback. Application operation does not depend on their network availability.

## Verification

Run `npm --prefix apps/web run build`. Set `FORGEFLOW_TEST_URL` to an isolated demo backend before `npm --prefix apps/web test`; browser mutation tests must not target the live database. Backend regression check: `go test ./...`. Browser coverage includes discovery, approvals, account setup, execution evidence, live polling, themes, contribution deep links, long data, keyboard tabs and reduced motion. Live GitHub discovery checks are skipped in demo mode.

Visual previews are saved in `artifacts/redesign-*.png` and `artifacts/execution-*.png`. These use illustrative demo or explicit fixture records; no contribution agent was started for UI verification.
