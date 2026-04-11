# Render Engine Specification

## Overview

The `render` command generates a standalone bash script from a resolved configuration. The script is runnable without the tool installed and produces equivalent results to `apply` for the same starting machine state.

## Output Format

The rendered script is a bash script with the following structure:

1. Shebang line: `#!/bin/bash`
2. Header comment: source config path, platform, generation timestamp, step count
3. Shell options: `set -euo pipefail`
4. Counter variables for tracking results
5. Pre-flight section: required environment variable checks
6. Step blocks: one per step in DAG-resolved order
7. Summary footer: result counts and exit code

## Pre-flight Section

The script MUST check all required secrets before any step runs:

```bash
# Pre-flight: Required Environment Variables
missing=0
[ -z "${VAR_NAME:-}" ] && echo "ERROR: Missing required secret: VAR_NAME (description)" >&2 && missing=1
if [ "$missing" -ne 0 ]; then
  echo "Set missing variables and re-run." >&2
  exit 1
fi
```

Optional secrets (`required: false`) MUST NOT be checked in pre-flight. Steps that need them will simply fail if the variable is unset.

## Atomic Step Blocks

Each atomic step renders as:

```bash
echo "[N/M] <step-name>"
if <check-command> &>/dev/null; then
  echo "  ✓ already satisfied"
  SKIPPED=$((SKIPPED + 1))
else
  <apply-command>
  if [ $? -eq 0 ]; then
    echo "  ✓ applied"
    SUCCEEDED=$((SUCCEEDED + 1))
  else
    echo "  ✗ FAILED" >&2
    FAILED=$((FAILED + 1))
  fi
fi
```

## Batch Step Blocks

Batch steps (packages, defaults, directories) render as individual commands per item:

```bash
echo "[N/M] <step-name> (<count> items)"
step_ok=0; step_fail=0; step_skip=0

# <item-1>
if <check-command-for-item-1> &>/dev/null; then
  step_skip=$((step_skip + 1))
else
  <apply-command-for-item-1>
  if [ $? -eq 0 ]; then step_ok=$((step_ok + 1)); else echo "  ✗ <item-1> failed" >&2; step_fail=$((step_fail + 1)); fi
fi

# ... one block per item

echo "  <step-name>: $step_ok applied, $step_skip present, $step_fail failed"
```

Items MUST be rendered as individual commands (not loops) for debuggability.

## Secret Handling

- Sensitive secrets MUST be referenced as `${VAR_NAME}` — literal values MUST NOT appear in the script.
- Non-sensitive secrets MAY also use `${VAR_NAME}` references.
- Secrets with `sensitive: true` MUST include a comment: `# (sensitive — value not embedded)`

## Fidelity

The rendered script produces equivalent results to `apply` for the same starting machine state. Acknowledged differences:
- Built-in Go steps may have richer checking logic than the shell equivalents.
- Terminal UI (spinners, color, progress) is not reproduced.
- Error handling is simpler (no structured JSON output, no log files).

The spec defines this as "best-effort equivalent."

## Output Options

- Default: write to stdout.
- `--output <file>`: write to the specified file and set the executable bit (`chmod +x`).

## Summary Footer

```bash
echo ""
echo "=== Summary ==="
echo "  Succeeded: $SUCCEEDED"
echo "  Skipped: $SKIPPED"
echo "  Failed: $FAILED"
[ "$FAILED" -gt 0 ] && exit 1 || exit 0
```
