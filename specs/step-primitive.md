# Step Primitive Specification

## Overview

The Step Primitive is the atomic unit of machine configuration. Every operation the system performs — installing a package, writing a config file, running a script — is expressed as a Step. All Steps, whether implemented in Go or defined in YAML, satisfy the same `Step` interface. The executor, DAG runner, and reporting engine operate exclusively on this interface.

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

---

## Step Interface

All Steps MUST implement the following Go interface:

```go
// Step is the atomic unit of machine configuration.
// Built-in steps implement this directly in Go.
// Custom (YAML-defined) steps are wrapped by ShellStep, which also implements this.
type Step interface {
    Name() string
    Check(ctx context.Context, cfg StepConfig) (StepResult, error)
    Apply(ctx context.Context, cfg StepConfig) (StepResult, error)
}
```

`Name()` MUST return the canonical name of the step as declared in config. It MUST be stable across calls.

`Check` and `Apply` MUST each return a `(StepResult, error)` pair. The `error` return value is RESERVED for unexpected infrastructure failures: a process that could not be launched, a context that was cancelled before execution began, or an OS-level failure. The `error` return MUST NOT be used to signal that the desired state is absent or that a command exited non-zero. Those outcomes are signalled via `StepResult.Status`.

An implementation that returns a non-nil `error` alongside a non-zero `StepResult.Status` produces undefined behavior; callers MAY ignore the `StepResult` entirely when `error` is non-nil.

---

## Step Configuration

`StepConfig` is the input to every `Check` and `Apply` call. It carries all runtime inputs derived from the merged YAML config, resolved before the call is made.

```go
// StepConfig carries all runtime inputs to a step's Check and Apply methods.
type StepConfig struct {
    // Metadata
    Name        string
    Description string
    Tags        []string // inert in v1; never used for filtering or execution

    // Dependency declarations
    Provides []string // capabilities this step advertises after successful execution
    Requires []string // capabilities that must be satisfied before this step runs

    // Platform targeting
    // Empty slice means "all platforms".
    // When non-empty, this step runs only on the listed platforms.
    Platforms []string

    // Execution commands (custom steps only; nil for built-in steps)
    Check    *ShellCommand // nil means "always needs apply"
    Apply    *ShellCommand
    Rollback *ShellCommand // persisted but never executed in v1

    // Environment
    // Env lists the names of env vars this step requires.
    // Values are sourced from the process environment via the Secrets System.
    // Validation occurs before the step runs (see Pre-Flight phase).
    Env []string

    // Timeouts
    CheckTimeout time.Duration // default: 5 minutes
    ApplyTimeout time.Duration // default: 15 minutes

    // Batch items (nil for atomic steps)
    // When non-nil, the step iterates Items and produces per-item results.
    Items []StepItem

    // Platform-specific apply overrides for ShellStep.
    // Key is a platform identifier ("darwin", "ubuntu", "debian").
    // Resolution order: PlatformApply[detectedPlatform] → Apply → StatusSkipped.
    PlatformApply map[string]*ShellCommand
}
```

### ShellCommand

```go
// ShellCommand represents a command to execute via os/exec.
// Args are passed directly to exec, not via a shell.
// If shell interpretation is required, Args MUST be ["sh", "-c", "<command>"].
type ShellCommand struct {
    Args []string          // e.g. ["brew", "list", "--formula", "git"]
    Env  map[string]string // additional env vars merged for this command only
}
```

`ShellCommand.Args` MUST contain at least one element. `Args[0]` is the executable. `Args[1:]` are the arguments. The executor MUST NOT pass these through a shell interpreter. The `Env` map MUST be merged on top of the process environment for the duration of that command only; it MUST NOT persist to subsequent commands.

### StepItem

```go
// StepItem is a single element within a batch step (e.g., one package).
type StepItem struct {
    Name    string // e.g. "git", "terraform"
    Version string // empty string means "any version acceptable"
    Pinned  bool   // if true, the item MUST NOT be upgraded by an upgrade operation
}
```

### Field Constraints

| Field | Type | Constraint |
|---|---|---|
| `Name` | `string` | MUST be non-empty |
| `Tags` | `[]string` | MAY be nil or empty; values are preserved but ignored in v1 |
| `Provides` | `[]string` | MAY be nil; no two steps in the same config MAY declare the same capability |
| `Requires` | `[]string` | MAY be nil; every listed capability MUST be declared as `Provides` by exactly one other step |
| `Platforms` | `[]string` | MAY be nil (means all platforms); valid values are `"darwin"`, `"ubuntu"`, `"debian"`, `"any"` |
| `Check` | `*ShellCommand` | MAY be nil for built-in steps; nil for `ShellStep` means "always unsatisfied" |
| `Apply` | `*ShellCommand` | MAY be nil if `PlatformApply` covers all required platforms |
| `Rollback` | `*ShellCommand` | MAY be nil; MUST NOT be executed in v1 regardless of value |
| `Env` | `[]string` | MAY be nil; values are env var names, not values |
| `CheckTimeout` | `time.Duration` | MUST be > 0; default is 5 minutes if not specified by config |
| `ApplyTimeout` | `time.Duration` | MUST be > 0; default is 15 minutes if not specified by config |
| `Items` | `[]StepItem` | NIL for atomic steps; empty slice is valid and produces `StatusSatisfied` |
| `PlatformApply` | `map[string]*ShellCommand` | MAY be nil; keys MUST be valid platform identifiers |

---

## Step Lifecycle

For every step, the executor runs the following sequence exactly once per execution. Phases are run in order; a terminal outcome in any phase stops execution and produces the final `StepResult`.

```
┌──────────────────────────────────────────────────────────────────┐
│  PRE-FLIGHT                                                      │
│  Validate env vars against the Secrets System.                   │
│  Any required env var missing → StatusSkipped; STOP.             │
└───────────────────────┬──────────────────────────────────────────┘
                        │
                        ▼
┌──────────────────────────────────────────────────────────────────┐
│  CHECK  (timeout: CheckTimeout, default 5 min)                   │
│  Run cfg.Check command or built-in Check logic.                  │
│  exit 0      → StatusSatisfied; STOP.                            │
│  non-zero    → unsatisfied; continue to APPLY.                   │
│  timeout     → treated as unsatisfied; log warning; continue.    │
│  infra error → return error; STOP.                               │
└───────────────────────┬──────────────────────────────────────────┘
                        │ (unsatisfied)
                        ▼
┌──────────────────────────────────────────────────────────────────┐
│  APPLY  (timeout: ApplyTimeout, default 15 min)                  │
│  Run platform-selected apply command or built-in Apply logic.    │
│  exit 0            → continue to VERIFY.                         │
│  non-zero          → StatusFailed; STOP.                         │
│  timeout           → StatusFailed, reason "timed out"; STOP.     │
│  no apply command  → StatusSkipped, reason "no apply command     │
│                       for platform <name>"; STOP.                │
│  infra error       → return error; STOP.                         │
└───────────────────────┬──────────────────────────────────────────┘
                        │ (apply exit 0)
                        ▼
┌──────────────────────────────────────────────────────────────────┐
│  VERIFY  (re-run Check logic; timeout: CheckTimeout)             │
│  exit 0    → StatusApplied; STOP.                                │
│  non-zero  → StatusVerifyFailed; STOP.                           │
│  timeout   → StatusVerifyFailed, reason "verify timed out"; STOP.│
│  infra err → return error; STOP.                                 │
└──────────────────────────────────────────────────────────────────┘
```

### Pre-Flight Phase

The executor MUST evaluate every name in `cfg.Env` against the Secrets System before calling `Check`. If any required env var is absent, the executor MUST NOT call `Check` or `Apply`. The step result MUST be `StatusSkipped` with `Reason` set to `"missing required env var: <NAME>"`, where `<NAME>` is the first missing variable name encountered.

If an env var is declared optional (`required: false` in the `secrets` config section) and is absent, the executor MUST emit a warning to the run log and MUST proceed with execution.

### Check Phase

The executor MUST apply `cfg.CheckTimeout` to the Check phase. The default `CheckTimeout` is 5 minutes. If the check command exceeds `CheckTimeout`:

1. The executor MUST send `SIGTERM` to the process.
2. The executor MUST wait up to 5 seconds for the process to exit.
3. If the process has not exited after 5 seconds, the executor MUST send `SIGKILL`.
4. The result MUST be treated as unsatisfied (not as a failure): execution continues to the Apply phase.
5. The executor MUST log a warning including the step name and the duration elapsed.

If `cfg.Check` is nil (for a `ShellStep`), the step MUST be treated as always unsatisfied. Execution proceeds directly to the Apply phase without error.

### Apply Phase

The executor MUST apply `cfg.ApplyTimeout` to the Apply phase. The default `ApplyTimeout` is 15 minutes. If the apply command exceeds `ApplyTimeout`:

1. The executor MUST send `SIGTERM` to the process.
2. The executor MUST wait up to 5 seconds for the process to exit.
3. If the process has not exited after 5 seconds, the executor MUST send `SIGKILL`.
4. The result MUST be `StatusFailed` with `Reason` set to `"timed out after <ApplyTimeout>"`.

If the apply command exits non-zero, the executor MUST capture the full stderr output and write it to the step's log file.

### Verify Phase

The Verify phase re-executes the same Check logic used in the Check phase. The same `CheckTimeout` applies. If the step has no `Check` command (i.e., `cfg.Check` is nil for a `ShellStep`), the executor MUST treat Verify as failed and return `StatusVerifyFailed`.

`StatusVerifyFailed` MUST NOT trigger a retry. It is a terminal outcome.

### Upstream Skip Propagation

The executor MUST skip a step when any capability listed in `cfg.Requires` was provided by a step whose final status was `StatusFailed`, `StatusVerifyFailed`, or `StatusSkipped`. The skip reason MUST be set to `"skipped because <step-name> failed"`, where `<step-name>` is the name of the failing upstream step.

Skip propagation MUST be transitive: if step A fails and step B requires A's provides, and step C requires B's provides, then both B and C are skipped.

`StatusPartial` MUST NOT cause downstream skips. A step with `StatusPartial` is considered to have partially satisfied its provides. All downstream steps that require those provides MUST run.

The step itself is not responsible for upstream skip propagation. This is the executor's responsibility.

---

## Step Result

```go
// StepStatus enumerates the possible outcomes of a step execution.
type StepStatus string

const (
    // StatusSatisfied: Check passed; no Apply was needed.
    StatusSatisfied StepStatus = "satisfied"

    // StatusApplied: Apply ran and exited 0; Verify passed.
    StatusApplied StepStatus = "applied"

    // StatusFailed: Apply ran and exited non-zero, or Apply timed out.
    StatusFailed StepStatus = "failed"

    // StatusPartial: batch step where some items succeeded and some failed.
    // Does not cause downstream skips.
    StatusPartial StepStatus = "partial"

    // StatusSkipped: step was not attempted.
    // Causes: (a) a required upstream step failed or was skipped,
    // or (b) a required env var is missing.
    // The Reason field MUST be populated.
    StatusSkipped StepStatus = "skipped"

    // StatusVerifyFailed: Apply exited 0 but the post-apply Check still fails.
    // Treated as a failure for downstream skip propagation.
    StatusVerifyFailed StepStatus = "verify_failed"
)

// StepResult is the outcome of a single Check or Apply call.
type StepResult struct {
    Status   StepStatus
    Reason   string // MUST be set when Status is StatusSkipped, StatusFailed, or StatusVerifyFailed

    // ItemResults is populated only for batch steps.
    // Each entry corresponds to a StepItem at the same index in cfg.Items.
    ItemResults []ItemResult

    // Duration of the operation that produced this result.
    Duration time.Duration
}

// ItemResult is the outcome for a single item within a batch step.
type ItemResult struct {
    Item   StepItem
    Status StepStatus // MUST be one of: StatusSatisfied, StatusApplied, StatusFailed
    Reason string     // MUST be populated when Status is StatusFailed
}
```

### Status Semantics

| Status | Meaning | Downstream skips triggered? |
|---|---|---|
| `satisfied` | Desired state was already present; no change was made | No |
| `applied` | Desired state was absent; Apply succeeded; Verify confirmed | No |
| `failed` | Apply exited non-zero or timed out | Yes |
| `partial` | Batch step: at least one item succeeded and at least one failed | No |
| `skipped` | Step was not attempted (missing env var or upstream failure) | Yes |
| `verify_failed` | Apply exited 0 but post-apply Check did not confirm desired state | Yes |

`Reason` MUST be a non-empty string when `Status` is `StatusSkipped`, `StatusFailed`, or `StatusVerifyFailed`. `Reason` SHOULD be a non-empty string for `StatusPartial` to summarize item-level failures. `Reason` MAY be empty when `Status` is `StatusSatisfied` or `StatusApplied`.

`ItemResults` MUST be nil for atomic (non-batch) steps. For batch steps, `ItemResults` MUST have the same length as `cfg.Items`, and each entry MUST correspond to the `StepItem` at the same index.

---

## Batch Steps

A batch step is a single DAG node that iterates an ordered list of items internally. It produces one `ItemResult` per item and one aggregate `StepResult` for the node.

### Execution Per Item

For each item in `cfg.Items`, the step MUST execute the following sequence:

```
1. Run Check for this item.
   exit 0   → ItemResult{Status: StatusSatisfied}; continue to next item.
   non-zero → continue to Apply.

2. Run Apply for this item.
   exit 0   → continue to Verify.
   non-zero → ItemResult{Status: StatusFailed, Reason: "<stderr summary>"}; continue to next item.

3. Run Verify for this item.
   exit 0   → ItemResult{Status: StatusApplied}.
   non-zero → ItemResult{Status: StatusFailed, Reason: "verify failed"}.
```

A failed item MUST NOT halt iteration. The step MUST continue processing remaining items.

### Aggregate Result

The final `StepResult.Status` for a batch step is determined as follows:

| Items outcome | `StepResult.Status` |
|---|---|
| `cfg.Items` is empty | `StatusSatisfied` |
| All items: `StatusSatisfied` | `StatusSatisfied` |
| All items: `StatusApplied`, or mix of `StatusSatisfied` and `StatusApplied`, none `StatusFailed` | `StatusApplied` |
| At least one `StatusFailed` and at least one `StatusSatisfied` or `StatusApplied` | `StatusPartial` |
| All items: `StatusFailed` | `StatusFailed` |

### Resume Behavior

On re-run, the per-item Check determines whether each item requires action. Items that are already in the desired state produce `StatusSatisfied` and are not re-applied. No state file is read or written; the machine state is the authoritative record.

### Built-in vs Custom Batch Steps

Built-in batch steps (e.g., `brew-packages`) implement per-item Check and Apply in Go and produce structured `ItemResult` values. Custom steps defined via YAML and adapted by `ShellStep` MUST NOT use batch semantics in v1. `ShellStep` is always atomic. `cfg.Items` MUST be nil or empty when passed to a `ShellStep`.

---

## Platform Dispatch

Platform dispatch determines which apply command runs for a given step on the current platform.

### Valid Platform Identifiers

The valid runtime platform identifiers are `"darwin"`, `"ubuntu"`, and `"debian"`. These are detected at runtime from `GOOS` and `/etc/os-release`. The string `"any"` is a config-level keyword meaning "all platforms" — it is valid in the `Platforms` field but MUST NOT appear as a key in `PlatformApply`. When filtering steps by platform, a step with `"any"` in its `Platforms` list matches all runtime platforms. The `"ubuntu"` and `"debian"` platforms share the same adapter (ubuntu/debian adapter).

### Resolution Order for ShellStep

Given a detected platform identifier `P`, the executor MUST resolve the apply command as follows:

1. If `cfg.PlatformApply[P]` is present and non-nil, use it.
2. Else if `cfg.Apply` is non-nil, use it.
3. Else: the step result MUST be `StatusSkipped` with `Reason` set to `"no apply command for platform <P>"`.

### Resolution for Built-in Steps

Built-in steps implement platform dispatch internally in Go. The lifecycle runner does not inspect `PlatformApply` for built-in steps. The runner treats the result of the built-in's `Apply` method as authoritative.

### Platform Filtering vs. Platform Dispatch

Platform filtering (excluding a step from the DAG entirely when `cfg.Platforms` does not include the detected platform) is the DAG Resolver's responsibility. It occurs during graph construction, before any step is executed. Platform dispatch (selecting the correct apply command within an already-included step) is the executor's responsibility and occurs at Apply time. These are two distinct mechanisms.

A step's own platform dispatch logic fires only for steps that have already passed platform filtering. A step MUST NOT re-check `cfg.Platforms` inside its `Apply` implementation.

---

## Command Execution

### Check Command Contract

- Exit 0 signals that the desired state is already present. No Apply is needed.
- Any non-zero exit code signals that the desired state is absent or incorrect. Apply MUST run.
- The check command MUST be purely read-only. It MUST NOT modify system state.
- Timeout handling: see the Check Phase section of the Step Lifecycle.

### Apply Command Contract

- Exit 0 signals that the operation completed. Verify MUST run to confirm.
- Any non-zero exit code signals that the operation failed. The full stderr output MUST be captured and written to the step's log file.
- Apply commands MAY have side effects. The system assumes all apply commands are idempotent: running Apply on a system already in the desired state MUST be safe.
- Timeout handling: see the Apply Phase section of the Step Lifecycle.

### No Shell Interpolation

The executor MUST pass `ShellCommand.Args[0]` as the executable and `ShellCommand.Args[1:]` as arguments directly to `os/exec`. The executor MUST NOT pass command arguments through a shell interpreter.

If a step author requires shell features (pipes, redirects, variable expansion, glob expansion), the command MUST be expressed as `["sh", "-c", "<shell expression>"]`. This is an explicit, author-visible opt-in. It is not provided automatically.

This constraint applies to all commands: Check, Apply, and any item-level commands within batch steps.

### Environment Variable Merging

When executing a command, the executor MUST merge the following into the process environment, in this order (later entries win on conflict):

1. The inherited process environment.
2. The resolved values of all names listed in `cfg.Env` (sourced from the Secrets System).
3. The `ShellCommand.Env` map for the specific command being executed.

Merged environment MUST NOT persist beyond the lifetime of the single command invocation.

### Process Lifecycle on Timeout

When a timeout fires:

1. The executor MUST send `SIGTERM` to the process group of the launched command.
2. The executor MUST wait up to 5 seconds for the process to exit.
3. If the process has not exited within 5 seconds, the executor MUST send `SIGKILL` to the process group.
4. The executor MUST log the step name, the phase (Check or Apply), and the total elapsed duration.

---

## Custom Steps (ShellStep)

`ShellStep` adapts a YAML-defined custom step to the `Step` interface. All built-in steps implement `Step` directly in Go and do not use `ShellStep`.

```go
// ShellStep adapts a YAML custom step to the Step interface.
// It executes ShellCommand values from StepConfig via os/exec (not via a shell).
type ShellStep struct {
    config StepConfig
}

func (s *ShellStep) Name() string { return s.config.Name }

func (s *ShellStep) Check(ctx context.Context, cfg StepConfig) (StepResult, error) {
    if cfg.Check == nil {
        // No check command: step is always considered unsatisfied.
        return StepResult{Status: StatusFailed, Reason: "no check command defined"}, nil
    }
    ctx, cancel := context.WithTimeout(ctx, cfg.CheckTimeout)
    defer cancel()
    exitCode, err := execCommand(ctx, cfg.Check, cfg.Env)
    if err != nil {
        return StepResult{}, err // infrastructure failure
    }
    if exitCode == 0 {
        return StepResult{Status: StatusSatisfied}, nil
    }
    return StepResult{Status: StatusFailed}, nil
}

func (s *ShellStep) Apply(ctx context.Context, cfg StepConfig) (StepResult, error) {
    cmd := platformApplyCommand(cfg) // selects PlatformApply[detectedPlatform] or falls back to cfg.Apply
    if cmd == nil {
        return StepResult{
            Status: StatusSkipped,
            Reason: "no apply command for current platform",
        }, nil
    }
    ctx, cancel := context.WithTimeout(ctx, cfg.ApplyTimeout)
    defer cancel()
    exitCode, err := execCommand(ctx, cmd, cfg.Env)
    if err != nil {
        return StepResult{}, err
    }
    if exitCode != 0 {
        return StepResult{Status: StatusFailed, Reason: "apply exited non-zero"}, nil
    }
    return StepResult{Status: StatusApplied}, nil
}
```

`execCommand` MUST use `os/exec` with `Args[0]` as the executable and `Args[1:]` as arguments. It MUST NOT invoke a shell.

`ShellStep` MUST NOT be used for batch steps in v1. If `cfg.Items` is non-nil and non-empty, the executor MUST return an error before calling `ShellStep.Check` or `ShellStep.Apply`.

---

## Environment Variable Validation

Env var validation occurs in the Pre-Flight phase, before `Check` is called. The Step Primitive does not call the Secrets System directly; this is the executor's responsibility.

### Validation Sequence

1. The executor reads `cfg.Env` (the ordered list of required env var names for this step).
2. For each name, the executor queries the Secrets System whether the var is present and non-empty.
3. If any required var is missing: the executor MUST NOT call `Check` or `Apply`. The step result MUST be `StatusSkipped` with `Reason` set to `"missing required env var: <NAME>"`.
4. If an env var is declared optional (`required: false` in the `secrets` config section) and is absent, the executor MUST emit a warning to the run log and MUST proceed with step execution.
5. Validate commands associated with env vars (declared in the `secrets` section) are run by the Secrets System before the run begins, not per-step. The Secrets System caches validation results. The Step Primitive consumes the cached result; it does not re-run validation.

### Boundary Definition

`cfg.Env` is a declaration of names. It does not contain values. The executor resolves values from the Secrets System and passes a `map[string]string` of resolved values to the command execution layer. The Step Primitive never handles secret values directly.

---

## Deferred Features

### Rollback Field

The `Rollback` field of `StepConfig` is accepted in YAML config and preserved in the `StepConfig` struct. It MUST NOT be executed in v1.

```go
// Rollback is persisted from config but is never executed in v1.
// It exists so the field is preserved in config round-trips and
// so future versions can execute it without a schema change.
Rollback *ShellCommand
```

Implementors MUST NOT wire `Rollback` into any execution path in v1. The call site where rollback execution would otherwise occur MUST contain the following comment verbatim:

```go
// Rollback is not executed in v1. See step-primitive-design.md.
```

The rationale for deferral is that rollback semantics require ordering guarantees (reverse DAG order, partial rollback decisions) that are out of scope for v1. Defining the field now ensures configs written today are forward-compatible.

### Tags Field

The `Tags` field of `StepConfig` is accepted in YAML config and preserved in the `StepConfig` struct. It MUST NOT be used for step filtering, conditional execution, or any runtime behavior in v1.

```go
Tags []string // inert in v1; never used for filtering
```

No component of the system MAY read `Tags` for any purpose other than preservation through config serialization and deserialization round-trips. Implementors MUST NOT write code that branches on the contents of `Tags` in v1.

### Per-Step Timeout Configuration

The config schema accepts `timeout_check` and `timeout_apply` fields per step. These fields MUST NOT be read or enforced in v1. Implementations MUST apply the default timeouts unconditionally (5 minutes for Check, 15 minutes for Apply). These fields are reserved for v2.

---

## Timeout Defaults

| Phase | Default | Notes |
|---|---|---|
| Check | 5 minutes | Applied unconditionally in v1; per-step config is reserved for v2 |
| Apply | 15 minutes | Applied unconditionally in v1; per-step config is reserved for v2 |
| Verify | Same as `CheckTimeout` | Verify re-runs Check logic with the same timeout |
| Post-SIGTERM grace period | 5 seconds | Applied before SIGKILL is sent, in both Check and Apply phases |
