# Tasks & Status

## Project Name: adze

An adze shapes rough wood into a flat working surface — takes a raw machine and shapes it into a configured, ready-to-use state. Sibling project to kerf.

---

## Completed

### Design Phase
- [x] Evaluate alternatives (Nix, Ansible, Chezmoi, Homebrew Bundle, etc.)
- [x] Define core problem (bidirectional sync between config and machine state)
- [x] Choose implementation language (Go — single binary, no runtime deps)
- [x] Choose config format (YAML)
- [x] Design the step primitive (provides/requires → DAG)
- [x] Resolve key design decisions (15 decisions documented with reasoning)
- [x] Design the CLI command surface
- [x] Design the resume/recovery model
- [x] Design secrets/env var handling
- [x] Design AI-native features (doctor, init, step library)
- [x] Design bootstrap model (pre-built binary + config URL)
- [x] Document all decisions with reasoning (docs/design-context.md)
- [x] Capture brainstorming details (docs/brainstorming.md)
- [x] Define objectives and scope (docs/objectives.md)

### Spec Phase (kerf work: machine-spec — SQUARED)
- [x] Brainstorm and decide project name → **adze**
- [x] Write problem-space doc (01-problem-space.md)
- [x] Decompose into 12 spec components (02-components.md)
- [x] Research pass — 12 component research docs (03-research/)
- [x] Design pass — 12 component design docs (04-design/)
- [x] Write formal spec drafts — 12 specs (05-spec-drafts/)
- [x] Integration review — cross-spec consistency (06-integration.md)
- [x] Generate implementation task breakdown (07-tasks.md)
- [x] Kerf square passed (43/43 artifacts present)

---

## Up Next

### Pre-Implementation Housekeeping
- [ ] **Replace `<tool>` placeholders in spec drafts** with `adze` — all 12 spec files in `~/.kerf/projects/machine-setup/machine-spec/05-spec-drafts/`
- [ ] **Finalize kerf work** — `kerf finalize machine-spec --branch spec/v1`
- [ ] **Rename repo** — `machine-setup` → `adze` (folder, GitHub remote, go module name)
- [ ] **Copy finalized specs into repo** — `specs/` directory with all 12 spec files
- [ ] **Update CLAUDE.md** — reflect adze name, spec location, implementation workflow

### Phase 1: Foundation (Implementation)
- [ ] **T1. Initialize Go module** — `go mod init`, `cmd/adze/main.go`, package directories
- [ ] **T2. Config parser** — YAML parsing, Go structs, validation (42 error codes), both package syntaxes
- [ ] **T3. Include/merge system** — relative path resolution, deep merge, list dedup, circular detection
- [ ] **T4. Step primitive** — Step interface, ShellStep adapter, lifecycle (check→apply→verify), batch semantics
- [ ] **T5. Secrets system** — pre-flight validation, masking (`***`), interactive prompting

### Phase 2: Core Engine
- [ ] **T6. DAG resolver** — Kahn's algorithm, cycle detection, skip propagation, deterministic sort
- [ ] **T7. Platform adapters** — Adapter interface, darwin (brew/defaults/scutil), ubuntu (apt/gsettings/systemctl), generic
- [ ] **T8. Built-in step library** — 22 steps, config section bindings, step registry
- [ ] **T9. Resume/recovery** — Runner orchestrator, stateless resume, failure propagation, run summary, log files

### Phase 3: CLI Commands
- [ ] **T10. CLI framework** — cobra setup, global flags, config auto-detection, output modes (human/JSON)
- [ ] **T11. plan + apply** — pre-flight → execution → summary, progress UI, URL configs
- [ ] **T12. status + capture** — drift detection, reverse sync (packages only v1)
- [ ] **T13. install + remove + upgrade** — atomic operations, version pin respect
- [ ] **T14. validate + graph** — deep validation, tree/dot visualization
- [ ] **T15. render** — bash script generation, pre-flight checks, per-step blocks
- [ ] **T16. init + doctor** — machine scanning, AI context dump
- [ ] **T17. step list/info/add** — step library browsing and scaffolding

### Phase 4: Distribution
- [ ] **T18. Terminal UI** — spinners, progress, color detection (NO_COLOR, FORCE_COLOR)
- [ ] **T19. Bootstrap + releases** — install.sh, goreleaser, GitHub releases (4 architectures)
- [ ] **T20. README**

### Future (post-v1)
- [ ] Shell hook for drift detection (brew wrapper)
- [ ] `.env` file support for secrets
- [ ] Interactive capture mode (checkbox UI)
- [ ] Tags/profiles filtering (`apply --tags dev`)
- [ ] Pre/post hooks on steps
- [ ] Remote apply via SSH
- [ ] Community step registry
- [ ] Parallel execution for independent steps
- [ ] Fedora/RHEL/Arch/Alpine adapters
- [ ] Password manager integration (1Password, Bitwarden)

---

## Parallelization Plan

| Wave | Tasks | Notes |
|------|-------|-------|
| 1 | T1 | Foundation — must be first |
| 2 | T2, T19 | Config parser + bootstrap (independent) |
| 3 | T3, T4, T5 | Include, step primitive, secrets (parallel, all need T2) |
| 4 | T6, T7, T10 | DAG, adapters, CLI framework (parallel, need T4) |
| 5 | T8, T9, T18 | Step library, runner, UI (parallel, need T6/T7) |
| 6 | T11–T17 | All CLI commands (parallel, need T10 + various) |
| 7 | T20 | README (last) |

---

## Reference

| Doc | Purpose |
|-----|---------|
| `docs/design-context.md` | Complete history, decisions, and reasoning |
| `docs/brainstorming.md` | Detailed brainstorming with examples, CLI mockups, config samples |
| `docs/objectives.md` | High-level problem statement, goals, design principles, scope |
| `~/.kerf/projects/machine-setup/machine-spec/` | Full kerf spec work (43 artifacts) |
| `~/.kerf/.../05-spec-drafts/*.md` | The 12 formal spec documents |
| `~/.kerf/.../07-tasks.md` | Detailed implementation tasks with acceptance criteria |
