# Resume/Recovery Specification

## Overview

The tool uses a stateless recovery model. The machine itself is the source of truth. Every step's check runs on every invocation, and already-satisfied steps are skipped. No state file is maintained.

## Core Principle

Re-running after a failure automatically resumes from where the previous run left off. This is achieved through idempotent steps with honest check commands:
- If a step was successfully applied on a previous run, its check returns "satisfied" and the step is skipped.
- If a step failed or was not attempted, its check returns "unsatisfied" and the step is applied.

## Runner Input Contract

### StepConfig Requirement

Before calling `Run()`, the caller MUST provide a fully-populated `StepConfig` for every step in the execution graph. A StepConfig is "fully-populated" when it contains all fields resolved from the config file and the built-in step registry: Name, Check, Apply, PlatformApply, Items, Env, Timeout, and any step-type-specific parameters.

### Injection Mechanism

The Runner receives StepConfigs as a map keyed by step name. The Runner MUST NOT fall back to constructing its own StepConfig from partial data (e.g., building a minimal config with only Name/Provides/Requires). If a step appears in the execution graph but has no corresponding entry in the StepConfig map, the Runner MUST treat this as an internal error and fail the step with a clear error message identifying the missing config.

### No Implicit Defaults

The Runner MUST NOT synthesize Check or Apply commands. Command resolution — including platform-specific dispatch and built-in step registry lookups — is the responsibility of the `BuildStepConfigs` phase, not the Runner. The Runner executes whatever commands it receives.

## StepConfig Lifecycle

The following four-phase pipeline is the only supported path from config to execution. There is no valid use of the Runner that bypasses `BuildStepConfigs`.

1. **Config parsing.** YAML files are parsed and validated per the config-schema spec. This produces raw config structs with user-specified check/apply commands, items, and metadata.

2. **BuildStepConfigs.** Raw config is combined with the built-in step registry to produce fully-resolved StepConfig objects. For built-in steps (e.g., `homebrew`, `brew-packages`), the registry supplies Check, Apply, and PlatformApply commands. For user-defined steps, commands come directly from the config. The result is a complete set of StepConfigs.

3. **DAG resolution.** StepConfigs (or their dependency metadata) are fed into the DAG resolver, which determines execution order per the dag-resolver spec. The DAG operates on step names and their Provides/Requires relationships.

4. **Runner injection.** The resolved StepConfigs are mapped by name and injected into the Runner. The Runner now has both the execution order (from the DAG) and the full command payloads (from the StepConfig map). Execution MAY proceed.

## Step Classification

### Atomic Steps

Atomic steps have a single check/apply cycle. The step is either fully satisfied or not.

Execution:
1. Run check. If satisfied: result is `satisfied`, skip to next step.
2. Run apply.
3. Run check again (verify).
4. If satisfied: result is `applied`.
5. If not satisfied: result is `verify_failed`.
6. If apply errored: result is `failed`.

### Batch Steps

Batch steps iterate over a list of items, each with its own check/apply cycle.

Execution per item:
1. Run check(item). If satisfied: item result is `satisfied`.
2. Run apply(item). If errored: item result is `failed`.
3. Run check(item) again (verify). If satisfied: item result is `applied`. If not satisfied: item result is `verify_failed`.

Aggregate step result:
- All items satisfied → step result: `satisfied`
- All items applied or satisfied (none failed) → step result: `applied`
- Some items failed → step result: `partial`
- All items failed → step result: `failed`

### Nil Apply Handling

When the Runner encounters a step whose StepConfig has:
- `Apply` is nil, AND
- `PlatformApply` has no entry for the current platform (or `PlatformApply` is nil)

The Runner MUST:
1. NOT attempt to execute the step.
2. Record the step result as `skipped`.
3. Set the skip reason to: `"no apply command for platform <P>"` where `<P>` is the current platform identifier (e.g., `darwin`, `ubuntu`).
4. NOT propagate the skip as a failure — the step's `Provides` capabilities MUST remain available to downstream steps.

This behavior is distinct from failure-induced skips, which DO block downstream steps.

### Classification Table

| Type | Steps |
|------|-------|
| Atomic | xcode-cli-tools, homebrew, apt-essentials, node-fnm, python, go, rust, oh-my-zsh, shell-default, machine-name, git-config, ssh-keys |
| Batch | brew-packages, brew-casks, apt-packages, macos-defaults, gsettings, dock-layout, directories, zsh-plugins |

## Failure Propagation

When a step fails (result is `failed` or all items in a batch failed):
1. Identify all capabilities in the failed step's `Provides` list.
2. For each downstream step that transitively requires any of those capabilities: mark as `skipped`.
3. The skip reason MUST reference the upstream failure.

When a batch step has `partial` result (some items succeeded, some failed):
- The step's provided capabilities are considered available.
- Downstream steps are NOT skipped.

## Runner Callback Interface

The Runner MUST expose a callback interface for live progress reporting during execution.

### Callback Definitions

- `OnStepStart(stepName string, index int, total int)` — The Runner MUST invoke this immediately before a step's Check command runs. `index` is the 1-based position in execution order; `total` is the count of steps in the graph.

- `OnStepComplete(stepName string, index int, total int, result StepResult)` — The Runner MUST invoke this immediately after a step's full lifecycle completes (whether the outcome is satisfied, applied, failed, skipped, partial, or verify_failed). `result` carries the status, skip reason (if any), duration, and per-item results for batch steps.

### Contract Rules

1. Callbacks are optional. If no callback is registered, the Runner MUST execute identically — the Runner MUST NOT require callbacks to function.
2. Callbacks MUST be invoked synchronously on the same goroutine as step execution, before the next step begins. The Runner MUST NOT proceed to the next step until the callback returns.
3. The Runner MUST invoke `OnStepStart` exactly once per step, and `OnStepComplete` exactly once per step, in execution order. Skipped steps (both nil-apply skips and failure-propagation skips) MUST receive both callbacks.
4. The Runner MUST NOT buffer or reorder callback invocations. Each pair MUST be emitted as the step is processed.
5. Callbacks MUST NOT modify Runner state. They are read-only observers.

### Ownership

The callback interface is part of the Runner contract and is specified here. The CLI Surface spec (`specs/cli-surface.md`) defines how the UI layer consumes these callbacks to render progress (spinners, step counters, NDJSON events). That spec depends on this interface definition.

## Run Summary

After every run, the tool MUST display a summary:

```
=== Run Summary ===

Steps: <total> total
  ✓ <N> succeeded (<applied> applied, <satisfied> already satisfied)
  ✗ <N> failed
  - <N> skipped

Failed:
  <step-name> (partial: <succeeded>/<total> items)
    ✗ <item> — <error message>
      Log: <log-path>

Skipped:
  <step-name> — <skip reason>

Re-run: adze apply (completed steps skip automatically)
```

The "Re-run" line MUST only appear when there are failures.

## Exit Codes

This section defines exit codes for execution outcomes only. The full exit code taxonomy (including config errors, pre-flight failures, and unexpected errors) is defined in the CLI Surface spec.

| Code | Condition |
|------|-----------|
| 0 | All steps succeeded or were already satisfied |
| 1 | Unexpected error (panic, internal bug) |
| 2 | Config error (parse or validation failure — before execution begins) |
| 3 | Pre-flight failure (missing required secrets, graph errors — before execution begins) |
| 4 | All attempted operations failed; no new steps were successfully applied |
| 5 | Some steps were applied successfully and some failed (partial progress) |

Code 4 indicates "nothing changed." Code 5 indicates "progress was made but the run is incomplete."

Warnings (optional secrets missing, steps skipped due to missing optional secrets) do not affect the exit code.

## Log Files

- Location: `~/.config/adze/logs/<step-name>.log` (or `<step-name>-<item>.log` for batch items)
- Content: full stdout and stderr from the step's apply command
- Lifecycle: overwritten on each run (not appended)
- Creation: log files are created only for steps/items that fail
- The run summary MUST reference log paths for failed steps
