# ForgeFlow visual system

One neutral foundation, six workflow-specific accent families. Dark is the
first-use default; existing light and system preferences remain available.
Fonts are self-hosted Inter and JetBrains Mono.

## Ownership

- tokens.css: semantic themes, palette, spacing, radius, elevation and motion.
- ../style.css: Tailwind, reset, typography, forms and accessibility foundations.
- shell.css: navigation, ambient lighting, toolbar and responsive shell.
- components.css: shared controls, dialogs, data rows, feedback and Operator.
- pages.css: dashboard bento layouts and secondary page composition.
- workspace.css: execution stages, live activity, evidence and recovery.
- motion.css: shared interactions, presence, activity and reduced-motion rules.
- ../design.css: imports only. Do not append competing theme overrides.

## Color roles

| Family          | Purpose                                               |
| --------------- | ----------------------------------------------------- |
| Cosmic Orchid   | Brand, Overview, discovery, featured contribution     |
| Ember Rose      | Human decisions, permissions, review findings         |
| Aurora Jade     | Completed stages and recorded successful verification |
| Velvet Garnet   | Configuration, Operator and secondary context         |
| Obsidian Gold   | Measured usage and allowance context                  |
| Electric Indigo | Agents, execution and technical activity              |

Page accents are assigned by the application's data-page attribute. Semantic
status colors take precedence over a page accent. Portal dialogs use root
theme tokens; permission and Operator surfaces use their workflow palettes.

Most content belongs on neutral surfaces. Use matte statement surfaces for
consequential decisions or measured summaries. Put ambient lighting behind
the workspace or at surface edges, never over readable text. Elevation
distinguishes featured work, inspectors and overlays from supporting data.

## Elevation and lighting

Neutral panels use the shared surface. The surface-elevated variant is reserved
for the featured contribution, configuration editor, appearance panel, account
panel and Operator. Its surface-light variable follows the relevant workflow
palette. Execution and opportunity inspectors use the same elevation tokens.
Supporting lists and historical agent runs remain neutral. Verified review
evidence gains Jade treatment only when recorded approval and contribution
state agree. Matte decision and usage cards stay free of gradient fills.

Ambient lighting belongs behind the main canvas and uses the current page's
paired palette. It is static, smaller on mobile, and removed when reduced
transparency is preferred. Do not apply glowing borders to every panel.

Do not add decorative charts, simulated completion percentages or inferred
model identities. Existing explicitly labeled demo mode remains supported.

Closed mobile navigation is inert. Open navigation traps focus and restores
its opener. Dialog headings remain visible while bodies scroll. Motion
respects reduced-motion preferences. On narrow screens, outstanding decisions
precede monitoring. Detailed tables and terminals scroll in their own regions.

## Motion

Motion tokens in tokens.css control shared 100–240ms feedback. motion.css is
imported last so interaction rules have one owner. Animate transforms and
opacity; sidebar width is the deliberate exception for its layout transition.
Keep ambient blur static. Only clickable contribution cards lift on hover;
featured surfaces gain a restrained outline, not continuous movement.

Keyboard navigation and command search are immediate. Native selects and
details use progressive CSS enhancement and keep their normal semantics in
browsers without the relevant CSS support. Radix owns modal focus and exit
presence. Non-modal tooltip/backdrop presence is handled by ui/presence.tsx.
Tooltips appear after a short initial delay, skip that delay for adjacent
controls, remain hoverable, and never intercept touch activation.

Activity dots indicate an unknown completion percentage. The live process
indicator pulses only when the server reports a running agent. Numeric data
changes directly to its recorded value; never interpolate invented metrics.
Reduced motion removes transitions, shimmer and pulses. New motion checks
live in tests/motion.spec.ts; test fixtures never run production contributions.

## Implementation touch points

All paths below are relative to apps/web. Backend files, API contracts and
package dependencies are unchanged by this redesign.

- Shell: src/App.tsx.
- Pages: src/pages/Overview.tsx, Opportunities.tsx, Contributions.tsx,
  System.tsx and Login.tsx. System.tsx owns Agents, Activity, Usage and Settings.
  Configuration is restyled by the shared page system.
- Shared components: src/components/ContributionStages.tsx, ExecutionPanel.tsx,
  Operator.tsx, icons.tsx, primitives.tsx and ui/dialog.tsx.
- Foundations: src/style.css and src/design.css.
- Design modules: src/styles/tokens.css, shell.css, components.css, pages.css
  and workspace.css, with this README documenting their ownership.
- Verification: tests/design-system.spec.ts.

Existing routes, mutation handlers and human approval requirements remain in
place. Test fixtures are isolated from production product data; real-data
visual checks use a separate database snapshot with browser mutations blocked.
