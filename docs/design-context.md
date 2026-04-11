# Design Context — Full History and Reasoning

This document captures the complete context behind the machine-setup project: why it exists, what alternatives were evaluated, what decisions were made and why, and where the design stands. Written so that a new collaborator (human or AI) can understand not just *what* we're building but *why* every choice was made.

---

## Origin

The project owner (Greg) is about to rebuild a macOS machine. Over the years he's built shell scripts to automate this — one for dev machines, one for servers. The scripts work but have fundamental limitations:

- Package lists are hardcoded in bash
- Execution order is manually arranged via function call ordering
- No drift detection — if you install something outside the script, the script doesn't know
- No validation before execution — you find out about problems mid-run
- Config is buried in imperative code, not a separate data file
- No cross-platform support — separate scripts for different machines

**Source material**: Two existing scripts informed the design requirements:
1. **Dev setup script** (gist: `gregberns/d38fafa628fe461112f729c20a21529c`) — ~500 lines covering: Xcode CLI tools, Homebrew, brew packages, casks, git config, SSH keys, Oh My Zsh + theme + plugins, dotfiles, macOS system defaults (~40 `defaults write` commands), Dock configuration, directory creation. Has idempotency checks, colored output, logging, and confirmation prompts.
2. **Server optimizer script** (gist: `gregberns/070e100251d07efd1e8af508d4e3903d`) — Disables unnecessary macOS services for server use: Spotlight indexing, UI animations, launch agents/daemons (Siri, Game Center, Photos, iCloud, etc.), crash reporting, auto-updates. Requires SIP disabled for some operations. Includes a generated restore script.

These scripts represent the real-world scope of what the tool needs to handle.

---

## Alternatives Evaluated

### Nix / nix-darwin / home-manager

We did a deep evaluation of Nix. Here's what it actually is and why it was rejected:

**How Nix works**: Nix is a package manager where every package is installed to a unique path based on a hash of all its inputs (`/nix/store/abc123-git-2.44.0/`). Your "profile" is a set of symlinks into the store. Switching profiles (installing/removing packages) is atomic — swap one symlink. Rollback is instant.

**nix-darwin** extends this to macOS: it manages packages, Homebrew casks (via a declarative bridge), macOS `defaults write` settings (via `system.defaults.*`), launchd services, shell config, and more. Combined with **home-manager**, it manages dotfiles, git config, program-specific configs — everything in one set of `.nix` files.

**What's genuinely great about it**:
- Single source of truth. One `flake.nix`, one `darwin-rebuild switch`, whole machine converges.
- Atomic rollback. `darwin-rebuild switch --rollback` undoes the last change instantly.
- The `system.defaults` module covers most `defaults write` commands with typed, documented options.
- The Homebrew integration manages Homebrew declaratively, including removing unlisted packages on rebuild.

**Why it was rejected**:
- **The language is a real barrier.** Nix is a lazy, pure, functional language unlike anything most developers know. Simple things are simple, but conditionals, module options, and overlays require deep knowledge. Error messages are cryptic walls of text about evaluation failures.
- **One-way only.** Nix only does config → machine. If you `brew install something` outside the config, Nix doesn't know. Worse, with `cleanup = "zap"` enabled, `darwin-rebuild switch` will *remove* what you just installed because it's not in the config. You can't casually install now and codify later.
- **macOS requires an APFS volume.** The root filesystem `/` is read-only on macOS (since Catalina). Nix needs `/nix/store` at root, so the installer creates a dedicated APFS volume mounted at `/nix`. This is a permanent system-level disk layout change.
- **Flakes are still "experimental"** after 4+ years. Everyone uses them, but docs are inconsistent about whether to assume them.
- **The ecosystem knowledge is scattered** across blog posts, NixOS wiki, Discourse, and GitHub issues. No canonical reference.

**The fundamental philosophical gap**: Nix is designed for people who want to manage their system through config files exclusively. The moment you do anything outside the config, you're fighting the system. This project is designed for people who sometimes just `brew install` something and want to codify it later. Bidirectional sync vs one-way enforcement.

### Ansible (geerlingguy/mac-dev-playbook)

**What it is**: YAML playbooks with roles for Homebrew, Mac App Store, dotfiles, macOS defaults, Dock config. Well-maintained by Jeff Geerling.

**Why rejected**:
- Requires Python. macOS no longer ships Python 3, so you need to install Python before you can use the tool that installs things. The bootstrap is awkward.
- Slow. A full run takes 10-15 minutes even for small changes.
- Not truly declarative — won't remove packages you delete from config unless you write explicit removal tasks.
- Enterprise-flavored. "Writing enterprise YAML to install VS Code on your laptop."
- Also one-way (config → machine).

### Homebrew Bundle (Brewfile)

**What it is**: A `Brewfile` that lists taps, brew packages, casks, Mac App Store apps, and VS Code extensions. `brew bundle install` converges to the Brewfile. `brew bundle cleanup --force` removes unlisted packages.

**Assessment**: Excellent for package management, trivial learning curve. But only manages packages — no dotfiles, no macOS defaults, no system config. We'll likely use Brewfile mechanics *under the hood* (or model our package step similarly), but the user-facing config needs to cover much more.

### Chezmoi (dotfile manager)

**What it is**: Manages dotfiles as copies (not symlinks) with Go template support for multi-machine configs. Excellent one-command bootstrap. 19K GitHub stars.

**Assessment**: Best-in-class for dotfiles. Could complement this project. But doesn't handle packages, macOS defaults, or system configuration. Cross-reference if we need dotfile management beyond what our tool handles directly.

### dsully/macos-defaults

**What it is**: A Rust tool that manages macOS `defaults write` commands declaratively via YAML files.

**Assessment**: Solves a real pain point. Our tool's `defaults` section serves the same purpose. Worth studying their YAML format and domain coverage, but we'll implement this as a built-in step rather than depending on an external tool.

### Other tools evaluated

- **Mackup** (app settings backup): Symlink mode broken on macOS Sonoma+. Increasingly obsolete.
- **Dotbot**: Simple YAML-based dotfile linker. Less capable than chezmoi.
- **GNU Stow**: Symlink farm manager. Minimal, no config, no templating.
- **Zero.sh**: macOS-focused setup framework. Small community (326 stars), written in Swift.
- **Strap** (Mike McQuaid): Shell-based macOS bootstrap. Minimal, opinionated. Good model for our bootstrap script.
- **Salt/Chef/Puppet**: Enterprise fleet management. Total overkill for personal machines.
- **Devbox** (Jetify): Nix under the hood with JSON config, but focused on per-project dev environments, not whole-machine setup.

### The gap in the landscape

No existing tool does bidirectional sync between a config file and machine state. Every tool is config → machine only. The `status` (drift detection) and `capture` (machine → config) features are genuinely novel in this space.

---

## Key Design Decisions (with reasoning)

### 1. Go, not Python or shell

**Decision**: Single Go binary.

**Why Go**: Compiles to a single binary with no runtime dependencies. On a fresh machine, you download one file and it works. No version conflicts, no venv, no "which python."

**Why not Python**: macOS no longer ships Python 3. The first thing the user does is fight Python version management before the setup tool even runs. This was specifically rejected.

**Why not shell**: Shell is fine for the bootstrap script (3-5 lines to download the binary). But a 2000+ line shell script that parses YAML, resolves dependency graphs, does rich terminal output, and handles cross-platform logic is unmaintainable. Shell is the bootstrap layer, Go is the tool.

### 2. YAML config, not a DSL

**Decision**: User-facing config is YAML.

**Why**: Familiar to developers, easy for AI agents to read and write, no new language to learn. The #1 complaint about Nix is the language. We avoid that entirely.

**Why not TOML**: TOML is fine for flat config but gets awkward for nested structures (package lists, defaults domains). YAML handles the hierarchical nature of machine config naturally.

**Why not a Brewfile-style DSL**: A Brewfile is Ruby. It's simple but not pure data — it's executable code. YAML is data that can be parsed, validated, merged, and generated without executing anything.

### 3. Bidirectional sync (the core innovation)

**Decision**: The tool syncs in both directions — config → machine AND machine → config.

**Why**: This is the unsolved problem. Everyone builds config → machine. But real humans `brew install something` at the terminal and forget to update their config. Six months later they rebuild and wonder where half their tools went.

**The four operations**:
- `apply`: config → machine (what Nix, Ansible, and setup scripts do)
- `status`: diff machine vs config (like `git status` — shows drift)
- `capture`: machine → config (reverse sync — add discovered packages to config)
- `install`/`remove`: both directions atomically (install + update config in one command)

### 4. Dependency graph via provides/requires

**Decision**: Steps declare `provides` (capabilities they make available) and `requires` (capabilities they need). The tool resolves the DAG via topological sort.

**Why not manual ordering**: The existing setup scripts order everything by manually calling functions in sequence. Adding a new step means figuring out where it goes. With provides/requires, you declare dependencies and the tool figures out ordering. Adding a step that requires `node` automatically slots it after the `node` step.

**Cycle detection**: Required. If A requires B and B requires A, the tool errors at validation time (before any execution).

**Unresolved dependency detection**: If a step requires `cargo` but nothing provides `cargo`, the tool errors at validation time with a clear message. This is where AI agent review (via `doctor`) adds value — suggesting missing steps.

### 5. Batched package steps with per-item failure handling

**Decision**: All brew packages are one step in the DAG ("brew-packages" depends on "homebrew"), not 30 individual nodes. But within that step, each package is checked and installed individually.

**Why batched**: 30 packages = 30 DAG nodes is noisy and adds no value. You rarely need to express "tool X depends on ripgrep specifically." The dependency is on the package manager, not individual packages.

**Why per-item failure handling**: If package 15 of 30 fails, we don't want to abort. Report the failure, continue with the remaining packages, and on re-run, skip the 29 that succeeded and retry the 1 that failed. The machine is the state — each re-run checks actual state per package.

### 6. Install and upgrade are separate operations

**Decision**: `apply` only installs missing things. It does NOT upgrade existing packages. A separate `upgrade` command handles that.

**Why**: If you run `apply` to add one new package, you don't want it silently upgrading 30 existing packages. That's destructive and surprising. Upgrades should be intentional.

### 7. Version pinning is first-class

**Decision**: Packages can specify versions with two syntaxes:
```yaml
- git                    # latest
- name: terraform
  version: "1.7.5"
  pinned: true           # even upgrade won't touch this
```

**Why**: Dev machines need specific versions that match what the codebase expects. `node@20`, `python@3.11`, etc. The `pinned: true` flag goes further — it tells even `machine-setup upgrade` to leave it alone.

**Platform adapter responsibility**: The adapter translates abstract version pins to platform-specific mechanisms (`brew install node@20` / `brew pin terraform` on macOS, `apt install nodejs=20.x` / `apt-mark hold terraform` on Ubuntu).

### 8. Composable configs via includes

**Decision**: Configs support `include:` directives with deep merge semantics.

**Why**: If your macOS dev machine and Linux VM share 70% of their config (git identity, shell preferences, common dev tools), you don't want to maintain it in two places.

**Merge rules**:
- Scalars: main config wins on conflict
- Lists: concatenated, deduplicated
- Maps: deep merged recursively, main config wins on key conflicts
- Include order: later includes override earlier, main config overrides all

### 9. Platform adapters are pluggable

**Decision**: The tool has an adapter interface that abstracts platform-specific operations. Ship with darwin and ubuntu/debian adapters. Others can be added.

**Why**: The same config section (`packages`, `defaults`, `shell`) needs different commands on different OSes. The adapter translates. This also enables the `validate` command to check: "can this config run on this platform?"

**Primary targets**: macOS (Apple Silicon + Intel), Ubuntu/Debian.
**Future**: Fedora/RHEL, Arch, Alpine, Windows (WSL).

### 10. AI-native but AI-optional

**Decision**: The tool is designed to work with AI agents but does not require one.

**Why**: AI adds massive value in two places:
1. **Config authoring** — on the current machine, an agent runs `init`, browses the step library, drafts a config, runs `validate` + `plan` to iterate. The tool provides all the feedback loops.
2. **Config review** — `doctor` dumps full context (config, graph, validation warnings, platform info, available steps) as a prompt for an agent to analyze.

But AI is NOT needed during execution on a fresh machine. The tool runs standalone. This avoids the bootstrap chicken-and-egg problem (you'd need a browser + auth to get an API key, which you don't have on a fresh machine).

### 11. No state file — the machine is the state

**Decision**: The tool derives state from the machine (checking what's installed, what defaults are set) rather than maintaining a state file like Terraform's `.tfstate`.

**Why**: Simpler. No state file to get corrupted, lost, or out of sync. Every command checks actual machine state. The tradeoff is slightly slower checks (running `brew list` instead of reading a file), but for a personal machine this is negligible.

**Revisit if**: Drift detection proves too slow or inaccurate without a state file.

### 12. Bootstrap via pre-built binary

**Decision**: Fresh machine bootstrap is:
```bash
curl -fsSL https://github.com/<user>/<tool>/releases/latest/download/<tool>-darwin-arm64 -o /usr/local/bin/<tool>
chmod +x /usr/local/bin/<tool>
<tool> apply --config https://raw.githubusercontent.com/<user>/configs/main/macos-dev.yaml
```

**Why**: Three lines, gets the real binary with full features (progress UI, drift detection, proper error handling). The config can be a URL (gist, raw GitHub file) — no need to clone a repo first.

**The `render` command** generates a standalone shell script from the config. This serves as an audit artifact ("show me exactly what will run") and a fallback, but is NOT the primary bootstrap path. The binary is.

### 13. `capture` uses `brew leaves`, not `brew list`

**Decision**: When scanning machine state, use `brew leaves` (explicitly installed, ~30 packages) not `brew list` (everything including transitive deps, ~200+).

**Why**: The config should represent intentional choices, not every transitive dependency. `leaves` produces a clean, meaningful package list.

### 14. `--config` required when ambiguous

**Decision**: If the workspace has multiple configs matching the current platform, `--config` is required. If there's only one match, auto-select it.

### 15. Shell hook for drift detection

**Decision**: The tool can inject a shell hook that wraps `brew`:
```bash
brew() {
  command brew "$@"
  if [[ "$1" == "install" || "$1" == "uninstall" || "$1" == "remove" ]]; then
    <tool> notify-drift
  fi
}
```

**Why**: When someone runs `brew install` directly (bypassing the tool), this sets a flag. Next terminal open or tool invocation shows a warning: "Machine state may have drifted. Run `status` to check."

Non-blocking, non-intrusive. Just a nudge to keep config in sync.

---

## The CLI

```
<tool>
├── init                      # scan current machine → generate config YAML
├── plan [--config]           # dry run: resolve graph, check state, show what would change
├── apply [--config]          # config → machine (with progress UI, pre-flight validation)
├── status [--config]         # drift detection: machine vs config
├── capture [--all] [--config]# machine → config (interactive or auto)
│
├── install <pkg> [--cask] [--config]  # atomic: install + add to config
├── remove <pkg> [--config]            # atomic: uninstall + remove from config
├── upgrade [--config]                 # upgrade non-pinned packages
│
├── validate [--config]       # deep validation, no execution
├── doctor [--config]         # dump context for AI agent review
├── render [--config]         # generate standalone shell script from config
├── graph [--config] [--format dot|text]  # visualize dependency graph
│
└── step
    ├── list [--platform]     # browse built-in step library
    ├── info <name>           # show step details
    └── add <name>            # scaffold a custom step in config
```

---

## Resume / Recovery Model

If a run fails partway through, re-running picks up where it left off. No state file — the machine is the state.

**Mechanism**: Every step's check runs on every invocation. Already-satisfied steps are skipped instantly. Batch steps (packages) check each item individually — if 18/20 packages installed on the first run, the re-run installs only the remaining 2.

**Failure behavior**: Partial failures don't abort the run. Failed items are reported. Independent steps continue. Steps that depend on a failed step are skipped with an explanation. The run summary at the end lists exactly what failed, why, and what to do.

---

## Related Project: Secure Dev VM

Greg has another project (`~/github/secure-dev`) that sets up secure development VMs. It faces the same "keep packages in sync with config" challenge. This tool is designed to be embeddable — the VM project could use it as a library (Go package) or subprocess to manage package installation and configuration inside VMs.

---

## Inspiration: NTM Bootstrap Pattern

The NTM project (github.com/Dicklesworthstone/ntm) uses a `curl | bash` bootstrap pattern with cache-busting and rich terminal progress output. This inspired our bootstrap approach (curl to download binary, then the binary handles everything with proper progress UI) and the emphasis on good status reporting during execution.

---

## Naming

The project currently uses the working name "machine-setup" but needs a real name before going public. Greg's other project uses a woodworking theme ("kerf"). Something in that vein could work — short, memorable, related to building/crafting/shaping.

This is explicitly deferred to the spec phase.

---

## Open Questions (deferred to spec)

These are acknowledged design questions that don't block v1 but should be addressed eventually:

1. **Step versioning**: Built-in steps will change as package managers and OSes evolve. Track via tool version, or version steps independently?
2. **Remote apply**: `apply --host user@vm-ip` to configure a remote machine via SSH. Useful for VM project. Significant complexity.
3. **Hooks**: Pre/post hooks on steps. "After applying oh-my-zsh, source .zshrc."
4. **Profiles / tags**: Tag-based step filtering. `apply --tags dev` only applies dev-tagged steps.
5. **Notifications**: Webhook/notification after long unattended runs.
6. **Testing in containers**: `apply --dry-run --in-docker` to test a Linux config in a container.
7. **Community step registry**: Shared step definitions. `step install community/docker`.
8. **Soft dependencies**: `wants` (nice to have) vs `requires` (must have). Current lean: keep it simple with just `requires`.
9. **Parallel execution**: Steps at the same DAG depth could run in parallel. Future optimization.
