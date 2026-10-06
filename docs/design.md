# Dashboard visual direction

Three directions were compared before implementation, using MagicPath's accessible Default theme as a token reference:
1. Graphite workbench: restrained neutral surfaces, violet selection accent, compact table, persistent navigation and contextual Operator. Best for long monitoring sessions.
2. Observatory: blue telemetry emphasis and charts above the pipeline. Strong monitoring but consumes valuable vertical space before live metrics exist.
3. Issue inbox: light editorial rows and larger issue previews. Good triage, weaker code/test density.

Chosen: graphite workbench. 216px sidebar, compact status bar, open main canvas, four restrained metrics, ranked issue table, lifecycle strip, chronological activity and Operator drawer. Accent denotes interactive selection; green/amber/red denote actual status. A continuous DEMO banner marks all illustrative data.
Use system sans fonts and monospace for identifiers/numbers; small corner radius, clear borders, no gradients. Light mode derives from the same semantic tokens. Keyboard focus, accessible dialogs, honest loading/error/empty states, reduced motion and laptop/mobile layouts are required.

