# Machine Setup — Brainstorming

## Core Concepts

### The Step Primitive

Everything is a step. A step is the atomic unit of machine configuration.

```yaml
step:
  name: string              # unique identifier
  description: string       # human-readable, shown in plan output
  provides: [string]        # capabilities this step makes available
  requires: [string]        # capabilities this step needs before it can run
  platform: [string]        # which platforms this step applies to (darwin, ubuntu, debian, any)
  check: string | map       # command(s) to determine if step is already satisfied
  apply: map                # per-platform commands to execute
  rollback: map             # per-platform undo commands (optional)
  env: [string]             # env vars / secrets this step needs at runtime
  tags: [string]            # for filtering (e.g., "server", "dev", "minimal")
```

Steps can be:
- **Built-in**: ship with the binary, cover common tools (git, brew, node, etc.)
- **Custom**: defined in the user's config YAML for anything not in the library
- **Community**: could eventually support a shared registry of step definitions

### The Dependency Graph

Steps form a directed acyclic graph (DAG) via provides/requires.

```
xcode-cli-tools ──→ homebrew ──→ git
                         │
                         ├──→ node ──→ fnm ──→ pnpm
                         ├──→ python ──→ pipx
                         ├──→ go ──→ my-go-tool
                         └──→ ripgrep, bat, fd, jq, ...
```

Resolution algorithm:
1. Collect all steps (built-in + custom) relevant to current platform
2. Build graph from provides/requires edges
3. Validate: no cycles, no unresolved requirements
4. Topological sort → execution order
5. For each step: run check → if unsatisfied, run apply

Open questions:
- **Soft vs hard dependencies**: should there be a `wants` (nice to have) vs `requires` (must have)? Or keep it simple with just `requires`?
- **Conditional steps**: steps that only run if a tag is active (e.g., `server` profile enables different steps than `dev`). Tags + filtering might be enough.
- **Parallel execution**: steps at the same depth in the DAG could theoretically run in parallel. Worth it? Adds complexity. Maybe a future optimization.

### Platform Adapters

The pluggable layer that knows how to do things on each OS.

```
Adapter interface:
  - PackageInstall(name string) → platform-specific install command
  - PackageCheck(name string) → is this package installed?
  - PackageList() → what's currently installed?
  - DefaultsRead(domain, key) → read a system preference
  - DefaultsWrite(domain, key, value, type) → set a system preference
  - ServiceEnable/Disable(name) → manage services
  - ShellDefault(shell) → set default shell
```

Adapters:
- **darwin**: brew, brew --cask, defaults, dockutil, scutil, launchctl, pmset
- **ubuntu/debian**: apt, snap, gsettings/dconf, systemctl
- **generic**: directories, git config, dotfiles (symlinks/copies), shell setup, SSH keys

The adapter handles the translation so steps can be defined at a higher level:

```yaml
# Instead of writing platform-specific apply commands for common packages,
# just declare the package and the adapter figures it out:
packages:
  - git      # adapter: brew install git | apt install git
  - ripgrep  # adapter: brew install ripgrep | apt install ripgrep
```

Custom steps still have explicit per-platform apply commands for anything the adapter doesn't handle.

---

## Config File Design

### Multi-config workspace

```
~/machine-setup/
├── macos-dev.yaml        # daily driver MacBook
├── macos-server.yaml     # Mac Mini server
├── ubuntu-vm.yaml        # dev VM
└── shared/
    ├── git.yaml          # shared git config (included by all)
    └── shell.yaml        # shared shell preferences
```

CLI targets a specific config:
```bash
machine-setup plan --config macos-dev.yaml
machine-setup apply --config ubuntu-vm.yaml
machine-setup status  # uses default config or auto-detects by platform
```

### Config structure (strawman)

```yaml
# macos-dev.yaml
name: "Greg's MacBook Pro"
platform: darwin
tags: [dev, personal]

# Import shared config fragments
include:
  - shared/git.yaml
  - shared/shell.yaml

# Machine identity
machine:
  hostname: greg-mbp

# Identity (used by built-in git and ssh steps)
identity:
  git_name: "Greg Berns"
  git_email: "greg@example.com"
  github_user: gregberns

# Required environment variables / secrets
# Validated before any step runs
secrets:
  - name: GITHUB_TOKEN
    description: "GitHub personal access token for private repos"
    check: "gh auth status"  # alternative to checking env var directly
  - name: NPM_TOKEN
    description: "npm registry auth token"
    required: false  # won't block execution if missing, just warns

# Packages — handled by platform adapter
# Short form: latest version. Object form: pinned version.
packages:
  brew:
    - git
    - jq
    - ripgrep
    - fzf
    - bat
    - fd
    - neovim
    - tmux
    - fnm
    - go
    - name: python
      version: "3.11"
    - name: node
      version: "20"
    - name: terraform
      version: "1.7.5"
      pinned: true           # machine-setup upgrade won't touch this
  cask:
    - iterm2
    - vscodium
    - google-chrome
    - slack
    - flux
    - diffmerge

# macOS system defaults
defaults:
  NSGlobalDomain:
    AppleShowAllExtensions: true
    NSDocumentSaveNewDocumentsToCloud: false
    NSAutomaticQuoteSubstitutionEnabled: false
    AppleKeyboardUIMode: 3
  com.apple.dock:
    autohide: true
    autohide-delay: 0
    tilesize: 36
    mru-spaces: false
  com.apple.finder:
    FXPreferredViewStyle: Clmv
  com.apple.screencapture:
    location: ~/Screenshots
    type: png

# Dock layout (macOS only)
dock:
  apps:
    - /Applications/Google Chrome.app
    - /Applications/VSCodium.app
    - /Applications/iTerm.app

# Shell configuration
shell:
  default: zsh
  oh_my_zsh: true
  theme: brad-muse
  plugins:
    - zsh-syntax-highlighting

# Directories to create
directories:
  - ~/github
  - ~/gitlab
  - ~/screenshots

# Custom steps — for anything not covered by built-ins
custom_steps:
  my-go-tool:
    description: "Internal Go tool for project scaffolding"
    provides: [my-go-tool]
    requires: [go]
    check: "command -v my-go-tool"
    apply:
      darwin: "go install git.internal.com/tools/my-go-tool@latest"
      ubuntu: "go install git.internal.com/tools/my-go-tool@latest"
    env: [GOPRIVATE]
```

### Shared config fragments

```yaml
# shared/git.yaml
# Included by multiple machine configs
identity:
  git_name: "Greg Berns"
  git_email: "greg@example.com"
  github_user: gregberns
```

Question: How do includes merge? Deep merge? Override? Explicit merge strategy? Probably deep merge with the main config winning on conflicts.

---

## CLI Commands — Detailed

### `machine-setup init`
Scan current machine and generate a config YAML from actual state.

Sources:
- `brew list` / `brew list --cask` → packages section
- `defaults read` for known domains → defaults section
- `dockutil --list` → dock section
- `git config --global --list` → identity section
- Check for oh-my-zsh, plugins, themes → shell section
- Scan common directories → directories section
- Detect platform → platform field

This is the "start from where you are" command. Run it on your current machine before a rebuild to capture what you have.

### `machine-setup plan [--config file.yaml]`
Resolve the dependency graph, check current state, show what would change. No mutations.

Output:
```
Platform: darwin (macOS 15.4)
Config: macos-dev.yaml

Pre-flight checks:
  ✓ Platform compatible
  ✓ Dependency graph valid (23 steps, 0 cycles)
  ✗ Secret GITHUB_TOKEN not set
  ✓ Secret NPM_TOKEN not set (optional, skipping)

Execution plan:
  1. [skip]    xcode-cli-tools     already satisfied
  2. [skip]    homebrew             already satisfied
  3. [install] bat                  brew install bat
  4. [install] fd                   brew install fd
  5. [skip]    git                  already satisfied
  6. [install] vscodium             brew install --cask vscodium
  7. [apply]   macos-defaults       12 defaults to set, 2 already correct
  8. [apply]   dock-layout          3 apps to configure
  9. [skip]    oh-my-zsh            already satisfied
  10. [apply]  directories          2 to create, 1 exists

Summary: 6 to apply, 4 already satisfied
         1 secret missing (GITHUB_TOKEN) — will block steps requiring it
```

### `machine-setup apply [--config file.yaml]`
Execute the plan. With progress UI.

Pre-flight validation runs first. If secrets are missing or graph is invalid, abort before any step executes.

Progress output (rich terminal UI):
```
Applying macos-dev.yaml...

[1/10] xcode-cli-tools          ✓ already satisfied
[2/10] homebrew                  ✓ already satisfied  
[3/10] bat                       ⠋ installing...
[3/10] bat                       ✓ installed (2.1s)
[4/10] fd                        ⠋ installing...
...
```

### `machine-setup status [--config file.yaml]`
Drift detection.

```
Comparing macos-dev.yaml against current machine state...

Packages:
  + lazygit      installed, not in config
  + htop         installed, not in config
  - hugo         in config, not installed

Casks:
  + raycast      installed, not in config

Defaults:
  ~ com.apple.dock.tilesize    config: 36    actual: 48
  ~ com.apple.dock.autohide    config: true  actual: false

Everything else: ✓ in sync
```

### `machine-setup capture [--all] [--config file.yaml]`
Interactive (or auto) reverse sync.

### `machine-setup install <pkg> [--cask] [--config file.yaml]`
Atomic: install + add to config.

### `machine-setup remove <pkg> [--config file.yaml]`
Atomic: uninstall + remove from config.

### `machine-setup validate [--config file.yaml]`
Deep validation without execution. Checks:
- YAML syntax
- All requires have matching provides
- No dependency cycles
- All referenced platforms have apply commands
- All env vars in `secrets` section are documented
- All env vars referenced in step `env` fields are in `secrets`
- Custom step check commands are syntactically valid
- Include files exist and parse correctly

### `machine-setup doctor [--config file.yaml]`
Dump everything for AI agent review. Outputs a self-contained prompt with:
- Full config
- Resolved dependency graph
- Validation warnings
- Current platform info
- Step library listing (available built-in steps)
- Suggestions for the agent: "analyze dependencies, suggest missing steps, identify platform gaps"

### `machine-setup render [--config file.yaml]`
Generate a standalone shell script from the resolved config. The script is runnable without machine-setup installed. Useful for:
- Auditing: "show me exactly what commands will run"
- Fallback: if you can't install the Go binary, the rendered script works
- Sharing: give someone a script without requiring them to install the tool

```bash
#!/bin/bash
# Generated by machine-setup from macos-dev.yaml
# Platform: darwin
# Generated: 2026-04-10T15:30:00Z

set -euo pipefail

echo "Step 1/10: xcode-cli-tools"
if xcode-select --print-path &>/dev/null; then
  echo "  ✓ already satisfied"
else
  echo "  installing..."
  xcode-select --install
fi

echo "Step 2/10: homebrew"
if command -v brew &>/dev/null; then
  echo "  ✓ already satisfied"
else
  echo "  installing..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# ... etc
```

### `machine-setup graph [--config file.yaml] [--format dot|text]`
Visualize the dependency graph.

Text mode for terminal:
```
xcode-cli-tools
└── homebrew
    ├── git
    ├── node
    │   └── pnpm
    ├── go
    │   └── my-go-tool
    ├── python
    ├── ripgrep
    ├── bat
    └── fd
```

Dot mode for Graphviz rendering:
```
digraph { "homebrew" -> "git"; "homebrew" -> "node"; "node" -> "pnpm"; ... }
```

### `machine-setup step list [--platform darwin|ubuntu|any]`
Browse the built-in step library.

```
Built-in steps:

Core:
  xcode-cli-tools    Install Xcode Command Line Tools           [darwin]
  homebrew            Install Homebrew package manager            [darwin]
  apt-essentials      Install build-essential and common deps     [ubuntu, debian]
  
Languages:
  node                Install Node.js via fnm                     [darwin, ubuntu]
  python              Install Python 3                            [darwin, ubuntu]
  go                  Install Go                                  [darwin, ubuntu]
  rust                Install Rust via rustup                     [any]
  
Shell:
  oh-my-zsh           Install Oh My Zsh framework                 [any]
  zsh-plugins         Install common Zsh plugins                  [any]
  starship            Install Starship prompt                     [any]

System (macOS):
  macos-defaults      Apply macOS system preferences              [darwin]
  dock-layout         Configure Dock apps and settings            [darwin]
  machine-name        Set hostname via scutil                     [darwin]
  
System (Linux):
  gsettings           Apply GNOME/desktop preferences             [ubuntu]
```

### `machine-setup step info <name>`
Show full details for a built-in step.

---

## Script Generation — Deeper Thoughts

The `render` command is interesting beyond just auditing. Consider:

- **Bootstrap scenario**: You can't install the Go binary yet (fresh machine, no Homebrew, no Go). But you CAN run a shell script. So the workflow could be:
  1. On your current machine: `machine-setup render --config macos-dev.yaml > setup.sh`
  2. Host that script (gist, S3, your repo)
  3. On new machine: `curl -fsSL https://example.com/setup.sh | bash`
  
  This is the NTM-style bootstrap, but the script is generated from your config, not hand-written.

- **CI/CD**: render a script for a CI job that sets up a runner.

- **Diffing configs**: render two configs to scripts, diff the scripts. Clear view of what's different between your macOS and Linux setups.

---

## Secrets / Env Vars — Deeper Thoughts

### Declaration in config
```yaml
secrets:
  - name: GITHUB_TOKEN
    description: "For cloning private repos"
    validate: "gh auth status"  # optional: richer validation than just "is it set"
    required: true
    
  - name: OPENAI_API_KEY
    description: "For AI-powered tools"
    required: false  # warn but don't block
    
  - name: SSH_PASSPHRASE
    description: "Used during SSH key generation"
    sensitive: true  # never print value, even in debug mode
    prompt: true     # if missing, prompt user interactively instead of failing
```

### Pre-flight validation
```
$ machine-setup validate --config macos-dev.yaml

Secrets:
  ✓ GITHUB_TOKEN     set (validated via: gh auth status)
  ⚠ OPENAI_API_KEY   not set (optional — some steps may skip)
  ✗ SSH_PASSPHRASE   not set (required — will prompt during apply)
```

### Where secrets come from
Options (not mutually exclusive):
1. Environment variables (simplest)
2. `.env` file in workspace (loaded by tool, gitignored)
3. System keychain (macOS Keychain, Linux secret-service)
4. 1Password CLI / Bitwarden CLI
5. Interactive prompt during apply

Start with 1 + 2 + 5. Add password manager integration later.

---

## Agent Integration — Deeper Thoughts

### The `doctor` prompt template

```
You are helping me configure machine-setup, a declarative machine configuration tool.

## My config
[full YAML]

## Resolved dependency graph
[topological order with provides/requires]

## Validation results  
[any warnings or errors]

## Available built-in steps not currently used
[list of steps in the library that aren't in this config]

## Current machine state (if available)
[output of machine-setup status]

## What I need help with
1. Review my custom steps — are dependencies declared correctly?
2. Are there built-in steps I should be using instead of custom ones?
3. For darwin-only steps, can you suggest ubuntu equivalents?
4. Are there common tools/configs I might be missing for a [dev/server] setup?
5. Review my defaults settings — any that are outdated or problematic on current macOS?
```

### Agent-authored configs

An agent on your current machine could:
1. Run `machine-setup init` to capture current state
2. Run `machine-setup step list` to see what's available
3. Draft a config combining captured state + step library
4. Run `machine-setup validate` to check it
5. Run `machine-setup plan` to preview
6. Iterate with you until it's right

The tool provides all the feedback loops the agent needs without the agent needing to understand Homebrew internals or `defaults write` syntax.

---

## Resolved Design Decisions

### 1. Package steps are batched, not individual DAG nodes

Packages (brew, cask, apt) are a single step in the DAG: "install brew packages" depends on "homebrew", not each package individually. This keeps the graph manageable (not 30 nodes for 30 packages).

However, package installation must handle **partial failure gracefully**:
- Attempt to install all packages in the list
- If some fail, report exactly which ones failed and why
- Continue with the rest of the setup (don't abort everything)
- On re-run, the check phase detects which packages are already installed and skips them, effectively **resuming where you left off**

This means the "check" for the brew-packages step isn't a single boolean. It's per-package:
```
brew-packages step:
  check each: brew list <pkg>
  apply each: brew install <pkg>
  result: { succeeded: [...], failed: [...], skipped: [...] }
```

The step is marked "partial" if some succeeded and some failed, and the failed list is reported clearly.

### 2. Install and upgrade are separate operations

`apply` only installs missing things. It does NOT upgrade existing packages.
A separate `machine-setup upgrade` command handles upgrades.

This prevents `apply` from being destructive — you don't want to update every package when you just needed to add one new one.

### 3. Version pinning is first-class

Packages can specify versions. Critical for dev machines where library versions must match code expectations.

```yaml
packages:
  brew:
    - git                    # latest
    - node@20                # pinned major version (homebrew syntax)
    - python@3.11            # pinned minor version
    - name: postgresql
      version: "16"          # explicit version object form
    - name: terraform
      version: "1.7.5"       # exact version
      pinned: true           # prevent upgrades from touching this
  cask:
    - vscodium
    - name: docker
      version: "4.28.0"     # specific cask version
```

Short form (`- git`) for latest, object form (`- name: ... version: ...`) for pinned.

The `pinned: true` flag means even `machine-setup upgrade` won't touch it. Useful for "I need exactly this version and don't want it to change."

Platform adapter implications:
- **darwin/brew**: `brew install node@20`, `brew pin terraform`
- **ubuntu/apt**: `apt install nodejs=20.x`, `apt-mark hold terraform`
- The adapter translates the abstract version pin into the platform-specific mechanism

### 4. `--config` required when ambiguous

If the workspace has multiple config files and the command can't auto-detect (e.g., two darwin configs), `--config` is required. If there's only one config matching the current platform, it can be auto-selected.

### 5. `capture` uses `brew leaves` (not `brew list`)

When scanning machine state to reverse-sync into config, use `brew leaves` (explicitly installed packages, typically ~30) rather than `brew list` (everything including transitive dependencies, typically 200+). This produces a clean, intentional config rather than a dump of every dependency.

### 6. Configs are composable via includes

Configs support an `include` directive for shared fragments. Deep merge semantics: included files provide defaults, the main config wins on conflicts.

```yaml
# macos-dev.yaml
include:
  - shared/identity.yaml    # git name, email, github user
  - shared/shell.yaml       # zsh, oh-my-zsh, plugins
  - shared/dev-packages.yaml # common dev tools

# Anything here overrides the included values
packages:
  brew:
    - swift  # additional package on top of shared dev-packages
```

Merge rules:
- Scalars: main config wins
- Lists: concatenated (main config entries appended to included entries), deduplicated
- Maps: deep merged recursively, main config wins on key conflicts
- Include order matters: later includes override earlier ones, main config overrides all

---

## Resume / Recovery Model

A key design goal: if a run fails partway through, re-running picks up where it left off. This falls out naturally from idempotent steps with per-item checks, but needs to be explicit for batch steps (packages).

### How it works

Every step's check runs at the start, even on re-run:
```
Run 1 (fresh machine):
  [1/8] xcode-cli-tools    ✓ installed (45s)
  [2/8] homebrew            ✓ installed (30s)
  [3/8] brew-packages       ⚠ partial (18/20 succeeded, 2 failed)
         ✗ libpq — formula not found (typo? tap needed?)
         ✗ watchman — build failed (missing dependency?)
  [4/8] brew-casks          ✓ installed (60s)
  [5/8] git-config          ✓ applied
  [6/8] oh-my-zsh           ✓ installed
  [7/8] macos-defaults      ✓ applied
  [8/8] directories         ✓ created

Result: 7/8 steps complete, 1 partial. 2 packages need attention.
```

User fixes the issue (adds a tap, fixes the name), re-runs:
```
Run 2 (re-run after fix):
  [1/8] xcode-cli-tools    ✓ already satisfied (skip)
  [2/8] homebrew            ✓ already satisfied (skip)
  [3/8] brew-packages       ✓ 18 already installed, installing 2 remaining...
         ✓ libpq — installed
         ✓ watchman — installed
  [4/8] brew-casks          ✓ already satisfied (skip)
  [5/8] git-config          ✓ already satisfied (skip)
  [6/8] oh-my-zsh           ✓ already satisfied (skip)
  [7/8] macos-defaults      ✓ already satisfied (skip)
  [8/8] directories         ✓ already satisfied (skip)

Result: 8/8 steps complete. All packages installed.
```

No state file needed for this. The machine itself is the state. Each check inspects actual state.

### Failure modes

| Failure | Behavior |
|---------|----------|
| Package not found | Report, skip, continue with remaining packages |
| Package build fails | Report with error output, skip, continue |
| Step check command errors | Report, treat as "not satisfied", attempt apply |
| Step apply command fails | Report, mark step as failed, continue with independent steps |
| Dependency of failed step | Skip (can't run if dependency failed), report why |
| Network failure mid-run | User re-runs when network is back, resumes automatically |
| Secret missing at runtime | Skip steps that need it, report which steps were skipped and why |

### The report

After every run, a summary shows exactly what needs attention:

```
═══ Run Summary ═══

Succeeded: 7 steps, 28 packages, 12 defaults, 3 directories
Failed:    2 packages
Skipped:   1 step (my-go-tool — requires GOPRIVATE, not set)

Failed packages:
  libpq    — formula not found
             Fix: brew tap homebrew/core, or check spelling
  watchman — build error (exit code 1)
             Log: ~/.machine-setup/logs/watchman.log

Next steps:
  1. Fix the issues above
  2. Re-run: machine-setup apply --config macos-dev.yaml
     (completed steps will be skipped automatically)
```

---

## Open Questions (Remaining)

1. **Step versioning**: Built-in steps will change as Homebrew changes, as macOS changes, as install methods change. Should steps have versions? Or just track the tool version?

2. **State file**: Should the tool maintain a local state file (like Terraform's `.tfstate`) tracking what it's applied? Or always derive state from the machine? State file adds complexity but enables better drift detection. Current lean: no state file, derive from machine.

3. **Remote apply**: Could you `machine-setup apply --config ubuntu-vm.yaml --host user@vm-ip` to configure a remote machine via SSH? Useful for the VM use case. Adds significant complexity.

4. **Hooks**: Pre/post hooks on steps (like git hooks). "Before applying `node`, run this." "After applying `oh-my-zsh`, source `.zshrc`." The step definition could include `pre_apply` and `post_apply` commands.

5. **Profiles / tags**: The config has `tags: [dev, personal]`. Steps could also have tags. `machine-setup apply --tags dev` only applies dev-tagged steps. Useful for "server" vs "dev" on the same base config.

6. **Notifications**: After a long apply, ping a webhook or send a notification. Low priority but nice for unattended runs.

7. **Testing**: Could you `machine-setup apply --config macos-dev.yaml --dry-run --in-docker` to test a Linux config in a container before applying to a real VM? Would need a Docker adapter.

8. **Community step registry**: A central place to share step definitions. `machine-setup step install community/docker` pulls a well-tested Docker step. Ambitious but interesting long-term.
