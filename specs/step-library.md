# Built-in Step Library Specification

## Overview

The tool ships with a library of well-known steps covering common tools, languages, shell configuration, and system settings. Built-in steps are compiled into the binary and implement the Step interface in Go.

## Step Inventory

### Core Infrastructure

#### xcode-cli-tools

| Field | Value |
|-------|-------|
| Provides | `xcode-cli-tools` |
| Requires | — |
| Platforms | darwin |
| Type | atomic |
| Check | `xcode-select -p` (exit 0 = installed) |
| Apply | `xcode-select --install` |

#### homebrew

| Field | Value |
|-------|-------|
| Provides | `homebrew` |
| Requires | `xcode-cli-tools` |
| Platforms | darwin |
| Type | atomic |
| Check | `command -v brew` (exit 0 = installed) |
| Apply | Official Homebrew install script: `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"` |

#### apt-essentials

| Field | Value |
|-------|-------|
| Provides | `apt-essentials` |
| Requires | — |
| Platforms | ubuntu, debian |
| Type | atomic |
| Check | `dpkg-query -W build-essential` (exit 0 = installed) |
| Apply | `sudo apt-get update && sudo apt-get install -y build-essential curl wget git` |

### Package Management

#### brew-packages

| Field | Value |
|-------|-------|
| Provides | `brew-packages` |
| Requires | `homebrew` |
| Platforms | darwin |
| Type | batch |
| Config section | `packages.brew` |
| Per-item check | `brew list <pkg>` |
| Per-item apply | `brew install <pkg>` (or `brew install <pkg>@<version>` for versioned) |

#### brew-casks

| Field | Value |
|-------|-------|
| Provides | `brew-casks` |
| Requires | `homebrew` |
| Platforms | darwin |
| Type | batch |
| Config section | `packages.cask` |
| Per-item check | `brew list --cask <pkg>` |
| Per-item apply | `brew install --cask <pkg>` |

#### apt-packages

| Field | Value |
|-------|-------|
| Provides | `apt-packages` |
| Requires | `apt-essentials` |
| Platforms | ubuntu, debian |
| Type | batch |
| Config section | `packages.apt` |
| Per-item check | `dpkg-query -W <pkg>` |
| Per-item apply | `sudo apt-get install -y <pkg>` (or `sudo apt-get install -y <pkg>=<version>` for versioned) |

### Languages

#### node-fnm

| Field | Value |
|-------|-------|
| Provides | `node`, `fnm` |
| Requires | `homebrew` (darwin), `apt-essentials` (ubuntu) |
| Platforms | darwin, ubuntu |
| Type | atomic |
| Check | `command -v fnm` |
| Apply (darwin) | `brew install fnm` |
| Apply (ubuntu) | `curl -fsSL https://fnm.vercel.app/install \| bash` |

#### python

| Field | Value |
|-------|-------|
| Provides | `python` |
| Requires | `homebrew` (darwin), `apt-essentials` (ubuntu) |
| Platforms | darwin, ubuntu |
| Type | atomic |
| Check | `command -v python3` |
| Apply (darwin) | `brew install python` |
| Apply (ubuntu) | `sudo apt-get install -y python3 python3-pip python3-venv` |

#### go

| Field | Value |
|-------|-------|
| Provides | `go` |
| Requires | `homebrew` (darwin), `apt-essentials` (ubuntu) |
| Platforms | darwin, ubuntu |
| Type | atomic |
| Check | `command -v go` |
| Apply (darwin) | `brew install go` |
| Apply (ubuntu) | `sudo apt-get install -y golang` |

#### rust

| Field | Value |
|-------|-------|
| Provides | `rust`, `cargo` |
| Requires | — |
| Platforms | any |
| Type | atomic |
| Check | `command -v rustc` |
| Apply | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh -s -- -y` |

### Shell

#### oh-my-zsh

| Field | Value |
|-------|-------|
| Provides | `oh-my-zsh` |
| Requires | — |
| Platforms | any |
| Type | atomic |
| Check | `[ -d "$HOME/.oh-my-zsh" ]` |
| Apply | `sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended` |

#### zsh-plugins

| Field | Value |
|-------|-------|
| Provides | `zsh-plugins` |
| Requires | `oh-my-zsh` |
| Platforms | any |
| Type | batch |
| Config section | `shell.plugins` |
| Per-item check | `[ -d "$HOME/.oh-my-zsh/custom/plugins/<name>" ]` |
| Per-item apply | `git clone <repo-url> "$HOME/.oh-my-zsh/custom/plugins/<name>"` |

The plugin repo URL is derived from the plugin name using well-known repositories (e.g., `zsh-syntax-highlighting` → `https://github.com/zsh-users/zsh-syntax-highlighting.git`). Unknown plugins produce a warning.

#### shell-default

| Field | Value |
|-------|-------|
| Provides | `shell-default` |
| Requires | — |
| Platforms | any |
| Type | atomic |
| Config section | `shell.default` |
| Check | `[ "$SHELL" = "<configured-shell-path>" ]` |
| Apply | `chsh -s <configured-shell-path>` (may require sudo on Linux) |

### System (macOS)

#### macos-defaults

| Field | Value |
|-------|-------|
| Provides | `macos-defaults` |
| Requires | — |
| Platforms | darwin |
| Type | batch |
| Config section | `defaults` |
| Per-item check | `defaults read <domain> <key>` compared to expected value |
| Per-item apply | `defaults write <domain> <key> -<type> <value>` |

After all defaults are written, affected processes MUST be restarted: `killall Dock`, `killall Finder`, `killall SystemUIServer` as appropriate.

#### dock-layout

| Field | Value |
|-------|-------|
| Provides | `dock-layout` |
| Requires | `homebrew` |
| Platforms | darwin |
| Type | batch |
| Config section | `dock` |
| Runtime prerequisite | `dockutil` binary must be available in PATH |
| Per-item check | `dockutil --find "<app-name>"` |
| Per-item apply | `dockutil --add "<app-path>"` |

Before executing, this step MUST check for the `dockutil` binary (`command -v dockutil`). If not found, the step MUST fail with the message: "dockutil is required for dock configuration. Add 'dockutil' to packages.brew in your config." This is a runtime check, not a DAG dependency — `dockutil` is a brew package, not a step capability.

#### machine-name

| Field | Value |
|-------|-------|
| Provides | `machine-name` |
| Requires | — |
| Platforms | darwin |
| Type | atomic |
| Config section | `machine.hostname` |
| Check | `scutil --get ComputerName` compared to configured value |
| Apply | `sudo scutil --set ComputerName "<value>" && sudo scutil --set LocalHostName "<value>" && sudo scutil --set HostName "<value>"` |

### System (Linux)

#### gsettings

| Field | Value |
|-------|-------|
| Provides | `gsettings` |
| Requires | — |
| Platforms | ubuntu |
| Type | batch |
| Config section | `defaults` (ubuntu variant) |
| Per-item check | `gsettings get <schema> <key>` compared to expected value |
| Per-item apply | `gsettings set <schema> <key> <value>` |

### Generic

#### directories

| Field | Value |
|-------|-------|
| Provides | `directories` |
| Requires | — |
| Platforms | any |
| Type | batch |
| Config section | `directories` |
| Per-item check | `[ -d "<path>" ]` |
| Per-item apply | `mkdir -p "<path>"` |

#### git-config

| Field | Value |
|-------|-------|
| Provides | `git-config` |
| Requires | — |
| Platforms | any |
| Type | atomic |
| Config section | `identity` |
| Check | `git config --global user.name` and `git config --global user.email` match configured values |
| Apply | `git config --global user.name "<value>"` and `git config --global user.email "<value>"` (plus github.user if configured) |

#### ssh-keys

| Field | Value |
|-------|-------|
| Provides | `ssh-keys` |
| Requires | — |
| Platforms | any |
| Type | atomic |
| Check | `[ -f "$HOME/.ssh/id_ed25519" ]` (or configured key type) |
| Apply | `ssh-keygen -t ed25519 -C "<email>" -f "$HOME/.ssh/id_ed25519" -N ""` |

The SSH key generation uses an empty passphrase by default. If the `SSH_PASSPHRASE` secret is configured and available, it is used as the passphrase.

## Config Section Bindings

| Step | Reads From |
|------|-----------|
| brew-packages | `config.packages.brew[]` |
| brew-casks | `config.packages.cask[]` |
| apt-packages | `config.packages.apt[]` |
| macos-defaults | `config.defaults{}` |
| gsettings | `config.defaults{}` |
| dock-layout | `config.dock{}` |
| git-config | `config.identity{}` |
| directories | `config.directories[]` |
| shell-default | `config.shell.default` |
| oh-my-zsh | `config.shell.oh_my_zsh` |
| zsh-plugins | `config.shell.plugins[]` |
| machine-name | `config.machine.hostname` |

Steps not listed above (xcode-cli-tools, homebrew, apt-essentials, language steps) are included automatically when another step requires their capabilities.

## Discoverability

### step list

The `step list` command MUST output all built-in steps grouped by category:

```
Built-in steps (<N> available):

Core:
  xcode-cli-tools    Install Xcode Command Line Tools              [darwin]
  homebrew            Install Homebrew package manager               [darwin]
  apt-essentials      Install build-essential and common deps        [ubuntu, debian]

Packages:
  brew-packages       Install Homebrew formulas from config          [darwin]
  ...
```

The `--platform` flag filters to steps supporting the specified platform.

### step info

The `step info <name>` command MUST output the full step definition:

```
Step: <name>
Description: <description>
Type: <atomic|batch>
Platforms: <platforms>
Provides: <capabilities>
Requires: <capabilities>
Config section: <section> (or "none")
Check: <command>
Apply: <command(s)>
```
