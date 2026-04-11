# adze

A declarative machine configuration tool. Define your machine's desired state in YAML, and adze shapes it into reality.

An adze shapes rough wood into a flat working surface -- takes a raw machine and shapes it into a configured, ready-to-use state.

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/gregberns/adze/main/install.sh | bash

# Scan your current machine to generate a config
adze init > my-machine.yaml

# See what would change
adze plan --config my-machine.yaml

# Apply the configuration
adze apply --config my-machine.yaml
```

## Features

- **Declarative YAML config** -- Define packages, settings, shell, identity, and more
- **Dependency-aware execution** -- Steps run in the right order via DAG resolution
- **Bidirectional sync** -- Install something manually, then `capture` it into your config
- **Stateless resume** -- Re-run after a failure and it picks up where it left off
- **Cross-platform** -- macOS (Homebrew) and Ubuntu/Debian (apt)
- **20 built-in steps** -- Packages, defaults, shell, identity, directories, and more
- **Custom steps** -- Define your own with shell commands
- **Render to bash** -- Generate standalone scripts for air-gapped machines

## Config Example

```yaml
name: "Greg's MacBook Pro"
platform: darwin

machine:
  hostname: greg-mbp

identity:
  git_name: "Greg Berns"
  git_email: "greg@example.com"
  github_user: gregberns

packages:
  brew:
    - git
    - jq
    - ripgrep
    - fzf
    - bat
    - neovim
    - name: terraform
      version: "1.7.5"
      pinned: true
  cask:
    - iterm2
    - vscodium
    - google-chrome

defaults:
  NSGlobalDomain:
    AppleShowAllExtensions: true
  com.apple.dock:
    autohide: true
    tilesize: 36

shell:
  default: zsh
  oh_my_zsh: true
  plugins:
    - zsh-syntax-highlighting

directories:
  - ~/github
  - ~/screenshots
```

## Commands

| Command | Description |
|---------|-------------|
| `adze init` | Scan machine and generate a config |
| `adze plan` | Show what changes would be made |
| `adze apply` | Apply the configuration |
| `adze status` | Show drift between config and machine |
| `adze capture` | Detect unconfigured packages |
| `adze install <pkg>` | Install a package and add to config |
| `adze remove <pkg>` | Uninstall and remove from config |
| `adze upgrade` | Upgrade non-pinned packages |
| `adze validate` | Validate config without executing |
| `adze graph` | Visualize the dependency graph |
| `adze render` | Generate a standalone bash script |
| `adze doctor` | Dump context for AI review |
| `adze step list` | List built-in steps |
| `adze version` | Print version info |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Config file path (or set `ADZE_CONFIG`) |
| `--json` | | Machine-readable JSON output |
| `--verbose` | `-v` | Show command output |
| `--quiet` | `-q` | Errors only |
| `--no-color` | | Disable color output |

## How It Works

1. **Parse** -- YAML config is parsed and validated (42 error codes, strict mode)
2. **Merge** -- Include files are resolved and deep-merged
3. **Resolve** -- Steps are ordered via DAG (Kahn's algorithm, deterministic)
4. **Check** -- Each step checks if its desired state is already present
5. **Apply** -- Steps that need changes are applied in order
6. **Verify** -- Each applied step is verified to confirm the change took effect

Failed steps propagate through the dependency graph -- downstream steps are automatically skipped. Re-running picks up where you left off (stateless resume).

## Built-in Steps

| Category | Steps |
|----------|-------|
| Core | xcode-cli-tools, homebrew, apt-essentials |
| Packages | brew-packages, brew-casks, apt-packages |
| Languages | node-fnm, python, go, rust |
| Shell | oh-my-zsh, zsh-plugins, shell-default |
| System | macos-defaults, dock-layout, machine-name, gsettings |
| Generic | directories, git-config, ssh-keys |

## Custom Steps

```yaml
custom_steps:
  my-go-tool:
    description: "Internal Go tool"
    provides: [my-go-tool]
    requires: [go]
    platform: [darwin, ubuntu]
    check: "command -v my-go-tool"
    apply:
      darwin: "go install git.internal.com/tools/my-go-tool@latest"
      ubuntu: "go install git.internal.com/tools/my-go-tool@latest"
    env: [GOPRIVATE]
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Unexpected error |
| 2 | Config error |
| 3 | Pre-flight failure |
| 4 | All operations failed |
| 5 | Partial success |
| 6 | Changes planned (plan only) |
| 7 | Drift detected (status only) |

## Building from Source

```bash
go build -o adze ./cmd/adze
```

## License

MIT
