# Machine Setup Project

## Issue Tracking

This project uses `bd` (beads) for task management. See AGENTS.md for details.

## Planning with kerf

This project uses kerf for structured planning. Before implementing non-trivial
changes (new features, refactors, bug investigations), create a kerf work:

  kerf new <codename>

This creates a work on the bench and shows the process to follow. The jig
(process template) guides you through structured passes -- problem space,
decomposition, research, detailed spec, integration, and tasks.

### Key commands

  kerf new <codename>              Create a new work
  kerf show <codename>             See current state + jig instructions for next steps
  kerf status <codename>           Check current status
  kerf status <codename> <status>  Advance to next pass
  kerf shelve <codename>           Save progress when ending a session
  kerf resume <codename>           Pick up where you left off
  kerf square <codename>           Verify the work is complete
  kerf finalize <codename> --branch <name>  Package for implementation

### When to use kerf

- New features or subsystems: kerf new --jig plan (or spec)
- Bug investigations: kerf new --jig bug
- Trivial changes (typos, one-line fixes): skip kerf, just make the change

### Workflow

1. kerf new <codename> -- read the output, it tells you exactly what to do
2. Follow each pass: write the artifacts, advance status
3. kerf show <codename> -- if you lose context, this shows where you are
4. kerf shelve / kerf resume -- for multi-session work
5. kerf square -- verify everything is complete
6. kerf finalize -- package into a git branch for implementation

Don't skip the planning process. Measure twice, cut once.

## Project Context

Design docs live in `docs/`. The key documents:
- `docs/design-context.md` -- Full history, decisions, reasoning (the "brief a new collaborator" doc)
- `docs/brainstorming.md` -- Detailed examples, CLI mockups, config samples
- `docs/objectives.md` -- Problem statement, goals, design principles, scope
- `TASKS.md` -- Phased task list with status
