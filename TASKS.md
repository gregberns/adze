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

### Pre-Implementation Housekeeping
- [x] Replace `<tool>` placeholders in spec drafts with `adze`
- [x] Finalize kerf work — `kerf finalize machine-spec --branch spec/v1`
- [x] Copy finalized specs into repo — `specs/` directory with all 12 spec files
- [x] Update CLAUDE.md — reflect adze name, spec location, implementation workflow

### Phase 1: Foundation
- [x] **T1. Initialize Go module** — go.mod, cmd/adze/main.go, package directories
- [x] **T2. Config parser** — YAML parsing, Go structs, 42 error codes, both package syntaxes (82 tests)
- [x] **T3. Include/merge system** — deep merge, list dedup, circular detection, depth limit (37 tests)
- [x] **T4. Step primitive** — Step interface, ShellStep, lifecycle, timeout, batch (35+ tests)
- [x] **T5. Secrets system** — pre-flight validation, masking, prompting (22 tests)

### Phase 2: Core Engine
- [x] **T6. DAG resolver** — Kahn's algorithm, cycle detection, deterministic sort (19 tests)
- [x] **T7. Platform adapters** — darwin, ubuntu, generic with full operations (82 tests)
- [x] **T8. Built-in step library** — 20 steps, registry, config bindings (52 tests)
- [x] **T9. Resume/recovery** — runner orchestrator, skip propagation, logging (24 tests)

### Phase 3: CLI Commands
- [x] **T10. CLI framework** — cobra, global flags, config auto-detection (40+ tests)
- [x] **T11. plan + apply** — pre-flight → execution → summary, progress UI (28 tests)
- [x] **T12. status + capture** — drift detection, reverse sync (22 tests)
- [x] **T13. install + remove + upgrade** — atomic operations, version pins (22 tests)
- [x] **T14. validate + graph** — deep validation, tree/dot visualization (16 tests)
- [x] **T15. render** — bash script generation with pre-flight checks (18 tests)
- [x] **T16. init + doctor** — machine scanning, AI context dump (29 tests)
- [x] **T17. step list/info/add** — step library browsing and scaffolding (18 tests)

### Phase 4: Distribution
- [x] **T18. Terminal UI** — spinners, progress, color detection (32 tests)
- [x] **T19. Bootstrap + releases** — install.sh, goreleaser, 4 architectures (20 tests)
- [x] **T20. README** — quick start, config example, command reference

---

## Implementation Stats

- **547 passing tests** across 12 packages
- **20 built-in steps** in the step library
- **42 validation error codes** + 2 warnings
- **16 CLI commands** (14 + 3 step subcommands)
- **3 platform adapters** (darwin, ubuntu, generic)
- **7 exit codes** with distinct semantics

---

## Known Issues (from spec-code convergence review)

- SEM-23: custom_steps provides uniqueness not enforced (duplicates within a single step's provides list)
- STR-02: Unknown keys within section mappings not strictly validated (parser uses manual traversal)
- Pre-flight skip reason format differs from spec wording (functionally equivalent)
- Darwin adapter PackageRemove pin check relies on caller, not runtime brew query

---

## Future (post-v1)
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

## Reference

| Doc | Purpose |
|-----|---------|
| `README.md` | Quick start and command reference |
| `specs/` | 12 formal specification documents |
| `docs/design-context.md` | Complete history, decisions, and reasoning |
| `docs/brainstorming.md` | Detailed brainstorming with examples |
| `docs/objectives.md` | High-level problem statement and goals |
