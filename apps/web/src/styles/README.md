# ForgeFlow visual system

One neutral foundation, six workflow-specific accent families. Dark is the
first-use default; existing light and system preferences remain available.
Fonts are self-hosted Manrope, Space Grotesk and JetBrains Mono. The two new
variable Latin fonts and their OFL licenses live in public/fonts.

## Ownership

- tokens.css: semantic themes, palette, spacing, radius, elevation and motion.
- ../style.css: Tailwind, reset, typography, forms and accessibility foundations.
- shell.css: navigation, ambient lighting, toolbar and responsive shell.
- components.css: shared controls, dialogs, data rows, feedback and Operator.
- pages.css: dashboard bento layouts and secondary page composition.
- workspace.css: execution stages, live activity, evidence and recovery.
- lighting.css: borderless card edges and pointer-positioned, hover-only rims.
- motion.css: shared interactions, presence, activity and reduced-motion rules.
- ../design.css: imports only. Do not append competing theme overrides.

## Route palettes

| Palette | Screens | Pair |
| --- | --- | --- |
| Cosmic Orchid | Overview | Orchid violet + luminous pink |
| Ember Rose | Opportunities, GitHub account | Coral rose + warm apricot |
| Aurora Jade | Contributions, Settings | Emerald mint + ice cyan |
| Velvet Garnet | Activity, Configuration, Operator | Garnet pink + peach |
| Obsidian Gold | Usage & effort | Champagne gold + copper |
| Electric Indigo | Agents, contribution workspace | Indigo blue + electric cyan |

App.tsx maps routes to a palette, including nested contribution paths. A layout
effect publishes data-palette on the document before paint. tokens.css owns the
six pairs on that attribute, so sidebar, toolbar, content, tooltips and Radix
portals inherit the same palette. Dark/light/system appearance stays separate.
Semantic status green, amber and red remain independent from decorative color.

## Elevation and lighting

The neutral charcoal base is shared by every screen. Both route colors appear
in the canvas ambient light, sidebar atmosphere, active navigation, brand mark,
primary actions and focal cards. Radial edge lighting leaves readable interiors;
soft dark shadows distinguish floating focal surfaces
from supporting lists. Configuration, opportunities and execution share this
treatment. Small summary tiles repeat the screen pair rather than introducing
unrelated palettes. Existing statement cards use a pale matte route accent and
dark text, with no gradient fill.

Ambient blur stays constant while its layer drifts through transforms, and is
smaller on mobile. Existing reduced-motion and
reduced-transparency preferences remain supported. No decorative chart, metric,
execution result or model identity is fabricated. Closed mobile navigation is
inert; open navigation traps focus and restores the opener.

## Motion

Motion tokens in tokens.css control shared 100–240ms feedback. motion.css is
imported last so interaction rules have one owner. Animate transforms and
opacity; sidebar width is the deliberate exception for its layout transition.
Never animate blur. Cards have no visible perimeter at rest. A masked paired
gradient rim and colored shadow appear only for fine-pointer hover; the light
tracks the pointer without tilting the reading surface. Touch has no sticky rim.
VisualEffects.tsx observes page cards and plays a one-time, staggered 650ms
transform/opacity reveal when they enter the viewport. It never hides content
before observation, never replays existing cards on query refresh, and leaves
nested evidence still. Keyboard input finishes these reveals immediately.

The overview's transparent chrome/glass sculpture is decorative, not a metric.
Its source is generated artwork, optimized to a self-hosted WebP in public/images;
no third-party request is made at runtime. Gentle floating, orbital and particle
motion is suspended offscreen and in hidden tabs. Pointer perspective affects
only the artwork. Hero text enters in a short stagger; icons respond to hover.
The sidebar's Pause animations control persists across routes and reloads.
Paused and reduced-motion states disable motion while preserving Radix's
zero-duration exit events and restoring focus correctly.

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
place. Regression tests use a disposable demo database. Real-data screenshots
read the running local app with browser API mutations blocked.
