# Machine Setup — Objectives

## The Problem

Setting up a new machine (macOS or Linux) is tedious, error-prone, and fragile. The common approaches all have significant drawbacks:

- **Shell scripts**: Hardcoded package lists, manual ordering, no drift detection, brittle across OS versions. Config is buried in imperative code. Works for bootstrapping but doesn't scale to ongoing maintenance.
- **Nix/nix-darwin**: Powerful and truly declarative, but the Nix language is a steep learning curve. One-way only (config → machine). If you install something outside Nix, it doesn't know. The ecosystem is fragmented and error messages are hostile.
- **Ansible**: YAML-based but slow, requires Python, enterprise-flavored for a personal machine. Also one-way.
- **Manual**: "I'll just remember what I installed." You won't.

The core unsolved problem: **bidirectional sync between a config file and actual machine state.** Every existing tool only goes config → machine. The moment someone runs `brew install something` or `apt install whatever` outside the config, the config drifts from reality. Nobody finds out until rebuild day, when stuff is missing.

## The Goal

A single CLI tool — written in Go, distributed as a single binary — that manages machine state declaratively from a YAML config, with:

1. **Config → machine** (`apply`): Read the config, make the machine match. Convergent and idempotent.
2. **Machine → config** (`capture`): Detect what's on the machine that isn't in the config, and offer to add it.
3. **Drift detection** (`status`): Show the diff between config and machine at any time.
4. **Atomic operations** (`install`/`remove`): Install a package AND update the config in one command, so they never diverge.
5. **Pre-flight validation** (`plan`, `validate`): Before anything executes, validate the entire config — resolve the dependency graph, check for missing dependencies, verify platform compatibility, confirm secrets/env vars are available. Like `terraform plan`.
6. **Script generation**: From the resolved config, render a plain shell script showing exactly what would run. Human-readable, auditable, runnable independently.

## Design Principles

### Dependency graph, not ordered lists
Steps declare what they **provide** and what they **require**. The tool resolves the DAG and determines execution order. No more manually ordering function calls in a script. Adding a new step that requires `node` automatically slots it after the `node` step.

### Built-in step library + custom steps
The tool ships with a library of well-known steps (git, homebrew, node, python, go, rust, oh-my-zsh, common defaults, etc.) with per-platform apply commands already defined. Users pick what they want — they don't need to author steps for common tools. Custom steps fill the gaps for anything not in the library.

### Platform-aware from the ground up
Every step declares which platforms it supports. The tool validates that a config can run on a given platform before execution. Cross-platform configs are first-class — the same step can have different apply commands for darwin vs ubuntu vs debian.

### Multi-config workspace
A single directory can contain multiple config files (e.g., `macos-dev.yaml`, `ubuntu-server.yaml`, `linux-vm.yaml`). The CLI can target any config in the workspace. Useful for someone managing both their laptop and a VM, or maintaining team configs.

### Secrets and environment variables as first-class config
Configs can declare required env vars and secrets. Pre-flight validation checks that they're all present and correctly named before any step runs. No more failing halfway through because an API key wasn't set.

### AI-native
The tool is designed to work with AI coding agents:
- **`doctor` command**: Dumps full context (config, graph, validation warnings, platform info) as a prompt an agent can consume to review and improve the config.
- **Step library as agent context**: An agent can browse available steps and compose configs from them.
- **Config authoring**: On an existing machine, an agent can run `init` to scan current state and generate a config, then iteratively refine it with the user.
- The tool does NOT require an AI agent to run. It's a standalone CLI. But it's designed so that an agent can be a first-class collaborator in authoring and maintaining configs.

### Embeddable
Other projects (e.g., a secure dev VM tool) can use this as a library or subprocess to manage package installation and machine configuration inside their own workflows.

## Target Platforms

### Primary
- macOS (Apple Silicon and Intel)
- Ubuntu / Debian

### Future (pluggable adapter system)
- Fedora / RHEL
- Arch Linux
- Alpine (containers)
- Windows (WSL)

## What This Is Not

- Not a replacement for Nix. No hermetic builds, no hash-verified store, no atomic rollback to previous generations. Those are Nix's strengths and require Nix's complexity.
- Not a configuration management fleet tool. This is for managing one machine (yours) or a small number of machines you personally care about.
- Not a containerization tool. This configures real machines and VMs, not container images (though it could be used inside a Dockerfile).
