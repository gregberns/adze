# Secrets System Specification

## Overview

The secrets system manages declaration, validation, sourcing, and masking of environment variables and secrets required by steps. Validation runs during pre-flight, before any step executes.

## Secret Declaration

Secrets are declared in the `secrets` section of the config. The field schema is defined in the Config Schema spec. This spec defines the behavioral semantics.

## Validation Flow

Pre-flight secret validation runs after config parsing and before step execution. For each entry in `config.secrets`:

1. Check if the environment variable `name` is set (`os.Getenv`).
2. If set and a `validate` command is specified: execute the validate command with a 30-second timeout.
   - Exit 0: secret is valid.
   - Non-zero: secret is set but validation failed.
   - Timeout: secret validation timed out (treated as invalid).
3. If set and no `validate` command: secret is valid.
4. If not set:
   - If `prompt` is true and the session is interactive: prompt the user (see Interactive Prompting).
   - If `required` is true: secret is missing (steps needing it will be skipped).
   - If `required` is false: secret is missing-optional (warning only).

## Cross-Reference Validation

For each step in the resolved configuration, for each environment variable in the step's `env` field:
- If the variable has no corresponding entry in `config.secrets`: the tool MUST emit a validation warning (not error):

```
Warning: step "<step-name>" references env var "<var>" not declared in secrets section
```

## Missing Secret Behavior

When a required secret is missing:
- All steps that list the secret's name in their `env` field MUST be marked as `skipped`.
- The skip message MUST identify the missing secret:

```
[skip] <step-name> — requires <SECRET_NAME> (not set)
```

- Other steps MUST continue normally.

When an optional secret (`required: false`) is missing:
- A warning MUST be emitted.
- No steps are skipped on this basis alone.

## Sourcing Precedence

In v1, the only source for secret values is environment variables (`os.Getenv`). `.env` file support, system keychain, and password manager integration are deferred.

## Interactive Prompting

When a secret has `prompt: true` and is not set in the environment:

**Interactive mode** (TTY detected, `--yes` not specified):
1. Print to stderr: the secret's `description` and a prompt for input.
2. If `sensitive` is true: disable terminal echo during input.
3. Read the value from stdin.
4. Set the value in the process environment (`os.Setenv`) for the duration of the run.
5. The value is NOT persisted to disk.

**Non-interactive mode** (`--yes` flag, or stdin is not a TTY):
- Prompting is skipped.
- The secret is treated as missing (normal missing-secret handling applies).

## Masking

For each secret with `sensitive: true`:
1. After the secret's value is resolved (from env or prompt), register the value in a masking filter.
2. All output channels (stdout, stderr, log files) MUST pass through the masking filter.
3. Any occurrence of the literal secret value in output MUST be replaced with `***`.
4. Masking applies to: plan output, apply progress, error messages, render output, doctor output, and log files.
5. In rendered scripts (from the `render` command): sensitive secrets MUST be referenced as `${VAR_NAME}` (never as literal values) and annotated with a comment `# (sensitive)`.

## Validation Output Format

The pre-flight secrets section of `plan` and `validate` output MUST follow this format:

```
Pre-flight: Secrets
  ✓ GITHUB_TOKEN       set (validated via: gh auth status)
  ⚠ OPENAI_API_KEY     not set (optional)
  ✗ SSH_PASSPHRASE     not set (required — will prompt during apply)
  ⚠ GOPRIVATE          referenced by step "my-go-tool" but not declared in secrets
```

Symbols: `✓` for valid, `✗` for missing-required, `⚠` for warnings (missing-optional or undeclared reference).
