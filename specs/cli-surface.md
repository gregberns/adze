# CLI Surface Specification

## Overview

The CLI is the user-facing interface to the tool. All commands follow consistent patterns for flags, output, and exit codes.

## Commands

### init

```
adze init [--config output.yaml]
```

Scan the current machine and generate a config YAML. See Init & Doctor spec for detection sources and output format.

- Default output: stdout
- `--config`: write to specified file
- Exit 0: success
- Exit 1: unexpected error

### plan

```
adze plan [--config file.yaml] [--json]
```

Resolve the dependency graph, check current state against config, and show what would change. No mutations.

Output (human):
```
Platform: <os> (<version>)
Config: <file>

Pre-flight:
  ✓ Platform compatible
  ✓ Dependency graph valid (<N> steps, 0 cycles)
  ✗ Secret GITHUB_TOKEN not set
  ✓ Secret NPM_TOKEN not set (optional, skipping)

Plan:
  1. [skip]    xcode-cli-tools     already satisfied
  2. [skip]    homebrew             already satisfied
  3. [install] bat                  brew install bat
  ...

Summary: <N> to apply, <M> already satisfied, <K> blocked by missing secrets
```

- Exit 0: no changes needed
- Exit 2: config error
- Exit 3: pre-flight failure (graph errors, required secrets missing)
- Exit 6: changes would be made

### apply

```
adze apply [--config file.yaml] [--yes] [--json]
```

Execute the plan. Pre-flight validation runs first.

- `--yes`: non-interactive mode (skip secret prompts, assume yes to confirmations)
- `--config <url>`: accepts a URL (see Bootstrap spec)
- Progress UI: see Output section
- Exit codes: 0 (success), 1 (unexpected), 2 (config error), 3 (pre-flight fail), 4 (all failed), 5 (partial success)

### status

```
adze status [--config file.yaml] [--json]
```

Compare config against current machine state. Show drift.

Output (human):
```
Comparing <file> against current machine state...

Packages:
  + lazygit      installed, not in config
  + htop         installed, not in config
  - hugo         in config, not installed

Defaults:
  ~ com.apple.dock.tilesize    config: 36    actual: 48

Everything else: ✓ in sync
```

- `+`: on machine, not in config
- `-`: in config, not on machine
- `~`: value differs
- Exit 0: in sync
- Exit 7: drift detected

### capture

```
adze capture [--all] [--config file.yaml] [--json]
```

Detect packages on the machine that are not in the config.

- No flags: list drift to stdout (no mutations)
- `--all`: write all detected packages to the config file
- v1 captures packages only (brew/cask on macOS, apt on Ubuntu)
- Exit 0: success (or no drift found)

### install

```
adze install <pkg> [--cask] [--config file.yaml]
```

Atomic operation: install the package on the machine AND add it to the config file.

- `--cask`: install as a Homebrew cask (macOS only)
- On Ubuntu: `--cask` is not valid; the apt adapter is used automatically
- Exit 0: success
- Exit 4: install failed

### remove

```
adze remove <pkg> [--config file.yaml]
```

Atomic operation: uninstall the package AND remove it from the config file.

- Exit 0: success
- Exit 4: removal failed

### upgrade

```
adze upgrade [--config file.yaml] [--all] [--json]
```

Upgrade non-pinned packages to latest versions.

- Packages with `pinned: true` are never upgraded.
- Packages with `version: "X"` are upgraded only within the version constraint.
- `--all`: include casks with auto-updates (equivalent to `brew upgrade --greedy` for casks).
- Non-package steps are not affected.
- Exit codes: 0 (all upgraded), 4 (all failed), 5 (partial)

### validate

```
adze validate [--config file.yaml] [--json]
```

Deep validation of the config without execution. Checks:
- YAML syntax
- Schema compliance (all fields, types, constraints)
- Include resolution
- Dependency graph (cycles, unresolved requires)
- Secret declarations (cross-reference with step env fields)
- Platform compatibility

All errors are collected and reported together.

- Exit 0: valid
- Exit 2: config errors found
- Exit 3: graph or pre-flight errors found

### doctor

```
adze doctor [--config file.yaml]
```

Dump full context for AI agent review. See Init & Doctor spec for output format.

- Output: always to stdout
- Exit 0: success

### render

```
adze render [--config file.yaml] [--output file.sh]
```

Generate a standalone bash script from the resolved config. See Render Engine spec for output format.

- Default: stdout
- `--output`: write to file, set executable bit
- Exit 0: success
- Exit 2: config error

### graph

```
adze graph [--config file.yaml] [--format dot|text]
```

Visualize the dependency graph.

Text format (default):
```
xcode-cli-tools
└── homebrew
    ├── brew-packages
    ├── brew-casks
    ├── node-fnm
    │   └── ...
    └── go
```

Dot format (`--format dot`):
```
digraph { "homebrew" -> "brew-packages"; ... }
```

- Exit 0: success

### step list

```
adze step list [--platform darwin|ubuntu|any] [--json]
```

List all built-in steps. See Step Library spec for output format.

### step info

```
adze step info <name> [--json]
```

Show full details for a built-in step.

### step add

```
adze step add <name> [--config file.yaml]
```

Scaffold a custom step definition in the config's `custom_steps` section.

### version

```
adze version
```

Print version, commit hash, and build date.

## Global Flags

| Flag | Short | Effect | Env Var |
|------|-------|--------|---------|
| `--config` | `-c` | Explicit config file | `ADZE_CONFIG` |
| `--json` | | Machine-readable JSON output | — |
| `--verbose` | `-v` | Show command output | `DADO_LOG_LEVEL=debug` |
| `--quiet` | `-q` | Errors only | `DADO_LOG_LEVEL=error` |
| `--no-color` | | Disable color | `NO_COLOR` |

Flag precedence: explicit flag > environment variable > default.

## Config Auto-Detection

When `--config` is not specified and `ADZE_CONFIG` is not set:

1. Scan the current directory for `*.yaml` and `*.yml` files.
2. For each file: attempt to parse as a config (check for `platform` field).
3. Filter to files whose `platform` matches the current runtime platform (or `any`).
4. If exactly one match: use it.
5. If zero matches: exit with error and guidance to run `init` or specify `--config`.
6. If multiple matches: exit with error listing candidates.

Parent directories are NOT searched.

## Exit Code Taxonomy

| Code | Name | Meaning |
|------|------|---------|
| 0 | Success | Operation completed successfully |
| 1 | Unexpected | Internal error, panic, or bug |
| 2 | ConfigError | Config parse failure or schema validation failure |
| 3 | PreFlightFail | Missing required secrets, dependency graph errors |
| 4 | ExecFailure | All attempted operations failed (no progress made) |
| 5 | PartialSuccess | Some operations succeeded, some failed |
| 6 | ChangesPlanned | `plan` only: changes would be made |
| 7 | DriftDetected | `status` only: machine state differs from config |

## Output Modes

### Human (interactive TTY)

- Color: green (✓ success), red (✗ failure), yellow (⚠ warning, - skip), bold (headers)
- Spinner on current step; replaced with status symbol on completion
- Step counter: `[N/M]`
- Elapsed time shown for steps > 2 seconds

### Human (non-interactive / piped)

- No color, no spinner, no cursor movement
- One line per step on completion
- Same information content

### JSON

Query commands (`plan`, `status`, `validate`, `capture`, `step list`, `graph`): single JSON object at end.

Long-running commands (`apply`, `upgrade`): NDJSON (one event object per line). Each event has a `type` field. The stream ends with a `summary` event.

Pretty-printed when stdout is a TTY; compact when piped.

### Color Detection

Color is enabled when ALL of the following are true:
1. `--no-color` flag is not set
2. `NO_COLOR` environment variable is not set
3. `TERM` is not `dumb`
4. stdout is a TTY
5. `CI` environment variable is not set

`FORCE_COLOR` environment variable overrides all checks and enables color.
