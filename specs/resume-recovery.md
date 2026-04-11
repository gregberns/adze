# Resume/Recovery Specification

## Overview

The tool uses a stateless recovery model. The machine itself is the source of truth. Every step's check runs on every invocation, and already-satisfied steps are skipped. No state file is maintained.

## Core Principle

Re-running after a failure automatically resumes from where the previous run left off. This is achieved through idempotent steps with honest check commands:
- If a step was successfully applied on a previous run, its check returns "satisfied" and the step is skipped.
- If a step failed or was not attempted, its check returns "unsatisfied" and the step is applied.

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
