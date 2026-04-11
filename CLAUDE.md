# adze — Machine Configuration Tool

## Overview

adze shapes a raw machine into a configured, ready-to-use state. Declarative YAML config,
dependency-aware execution, bidirectional sync between config and machine state.

- Language: Go (single binary, no runtime deps)
- Config: YAML 1.2
- Binary: `adze`
- Env var prefix: `ADZE_`

## Issue Tracking

This project uses `bd` (beads) for task management. See AGENTS.md for details.

## Specs

Formal specifications live in `specs/`. There are 12 spec documents:
- `specs/config-schema.md` — Config file format, validation, error codes
- `specs/include-merge.md` — Include resolution, deep merge, dedup
- `specs/step-primitive.md` — Step interface, lifecycle, batch semantics
- `specs/secrets-system.md` — Secret validation, masking, prompting
- `specs/dag-resolver.md` — Dependency graph, Kahn's algorithm, skip propagation
- `specs/platform-adapters.md` — darwin/ubuntu/generic adapters
- `specs/step-library.md` — 22 built-in steps, registry
- `specs/resume-recovery.md` — Stateless resume, failure propagation, logging
- `specs/cli-surface.md` — Commands, flags, output modes, exit codes
- `specs/render-engine.md` — Bash script generation
- `specs/bootstrap.md` — install.sh, release binaries
- `specs/init-doctor.md` — Machine scanning, AI context dump

**Specs are the source of truth.** Implementation must match specs exactly.

## Planning with kerf

This project uses kerf for structured planning. Before implementing non-trivial
changes (new features, refactors, bug investigations), create a kerf work:

  kerf new <codename>

### Key commands

  kerf new <codename>              Create a new work
  kerf show <codename>             Show current state + jig instructions
  kerf status <codename>           Check current status
  kerf status <codename> <status>  Advance to next pass
  kerf shelve <codename>           Save progress when ending a session
  kerf resume <codename>           Pick up where you left off
  kerf square <codename>           Verify the work is complete
  kerf finalize <codename> --branch <name>  Package for implementation

## Project Structure

```
cmd/adze/          — main entry point
internal/
  config/          — YAML parser, schema structs, validation, include/merge
  step/            — Step interface, ShellStep, lifecycle, executor
  secrets/         — Secret validation, masking, prompting
  dag/             — DAG resolver, graph operations
  adapter/         — Platform adapters (darwin, ubuntu, generic)
  steps/           — Built-in step library, registry
  runner/          — Execution orchestrator, resume, logging
  cli/             — CLI commands, flags, output modes
  scan/            — Machine scanning (init command)
  ui/              — Terminal UI, spinners, progress, color
  render/          — Bash script renderer
specs/             — Formal specification documents
docs/              — Design docs and brainstorming
```

## Design Docs

- `docs/design-context.md` — Full history, decisions, reasoning
- `docs/brainstorming.md` — Detailed examples, CLI mockups, config samples
- `docs/objectives.md` — Problem statement, goals, design principles, scope
- `TASKS.md` — Phased task list with status

## Development

```bash
go build ./...              # Build
go test ./...               # Test all
go test ./internal/config/  # Test specific package
go vet ./...                # Static analysis
```
