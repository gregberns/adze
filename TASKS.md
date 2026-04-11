# Tasks & Status

## Current Phase: Design → Spec

The project has completed initial brainstorming and design decisions. The next phase is to formalize these into a spec and begin implementation.

---

## Completed

- [x] Evaluate alternatives (Nix, Ansible, Chezmoi, Homebrew Bundle, etc.)
- [x] Define core problem (bidirectional sync between config and machine state)
- [x] Choose implementation language (Go — single binary, no runtime deps)
- [x] Choose config format (YAML)
- [x] Design the step primitive (provides/requires → DAG)
- [x] Resolve key design decisions (batched packages, install vs upgrade, version pinning, composable includes, `brew leaves` for capture, `--config` when ambiguous)
- [x] Design the CLI command surface
- [x] Design the resume/recovery model
- [x] Design secrets/env var handling
- [x] Design AI-native features (doctor, init, step library)
- [x] Design bootstrap model (pre-built binary + config URL)
- [x] Document all decisions with reasoning (docs/design-context.md)
- [x] Capture brainstorming details (docs/brainstorming.md)
- [x] Define objectives and scope (docs/objectives.md)

## Up Next

### Phase 1: Spec

- [ ] **Brainstorm project name** — needs a real name. Woodworking theme preferred (the sibling project is called "kerf"). Short, memorable, typeable. Rename the project folder/docs once decided.
- [ ] **Write formal spec** — structured, implementable specification covering:
  - [ ] Config schema (exact YAML structure with all fields, types, defaults, validation rules)
  - [ ] Step primitive (Go struct, interface, lifecycle)
  - [ ] Dependency graph resolver (algorithm, error cases, output format)
  - [ ] Platform adapter interface (Go interface, what each adapter must implement)
  - [ ] Built-in step library (full list of v1 steps with per-platform commands)
  - [ ] CLI commands (exact flags, arguments, output formats, exit codes)
  - [ ] Include/merge system (exact merge semantics, conflict resolution, cycle detection)
  - [ ] Secrets system (declaration, validation, sourcing, masking)
  - [ ] Resume/recovery (per-step and per-item failure handling, reporting format)
  - [ ] `render` output format (generated script structure)
  - [ ] `doctor` output format (AI prompt template)
  - [ ] Bootstrap script (exact content)
  - [ ] Error messages and exit codes
- [ ] **Define v1 scope** — draw the line between "must have for v1" and "future." Candidates for v1 cut: remote apply, community registry, parallel execution, container testing, notifications.

### Phase 2: Implementation

- [ ] Initialize Go module and project structure
- [ ] Implement config parser (YAML → Go structs, include resolution, merge)
- [ ] Implement dependency graph resolver (build, validate, topological sort)
- [ ] Implement platform detection and adapter interface
- [ ] Implement darwin adapter (brew, cask, defaults, dockutil, scutil)
- [ ] Implement ubuntu/debian adapter (apt, gsettings, systemctl)
- [ ] Implement generic adapter (directories, git config, SSH, shell setup)
- [ ] Implement built-in step library
- [ ] Implement core commands: validate, plan, apply
- [ ] Implement drift detection: status
- [ ] Implement reverse sync: capture
- [ ] Implement atomic operations: install, remove
- [ ] Implement upgrade (with pinning respect)
- [ ] Implement render (script generation)
- [ ] Implement doctor (AI context dump)
- [ ] Implement graph (dependency visualization)
- [ ] Implement init (machine scan → config generation)
- [ ] Implement step subcommands (list, info, add)
- [ ] Build terminal UI (progress, status, spinners)
- [ ] Write bootstrap script
- [ ] Set up GitHub releases with cross-compiled binaries
- [ ] Write README

### Phase 3: Polish & Extend

- [ ] Shell hook for drift detection
- [ ] `.env` file support for secrets
- [ ] Interactive `capture` mode
- [ ] Tags/profiles for conditional step execution
- [ ] Pre/post hooks on steps
- [ ] Remote apply via SSH
- [ ] Community step registry
- [ ] Parallel execution for independent steps

---

## Reference

| Doc | Purpose |
|-----|---------|
| `docs/design-context.md` | Complete history, decisions, and reasoning — the "brief a new collaborator" doc |
| `docs/brainstorming.md` | Detailed brainstorming with examples, CLI mockups, config samples, open questions |
| `docs/objectives.md` | High-level problem statement, goals, design principles, scope boundaries |
