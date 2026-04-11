# Config Schema Specification

## Overview

A machine-setup config file is a single YAML 1.2 document that declares the complete desired state for one machine. The file contains a fixed set of top-level sections — each governing one aspect of configuration (packages, system defaults, shell, identity, secrets, custom steps, and others). The parser MUST reject any unknown top-level key with a validation error. Every section is optional except `name` and `platform`. All optional sections normalize to an empty or zero value when omitted. The file is parsed, validated, and merged (if `include` entries are present) before any execution takes place.

---

## Config File Format

- Format: YAML 1.2
- Parser: `gopkg.in/yaml.v3` with strict mode enabled (`KnownFields(true)`)
- Encoding: UTF-8
- File extensions: `.yaml` or `.yml` (both accepted)
- A config file MUST be a YAML mapping at the top level. A YAML sequence, scalar, or null document MUST produce the error: `config: top-level document must be a YAML mapping`
- YAML 1.1 semantics MUST NOT be used. Specifically: `yes`, `no`, `on`, `off` are treated as strings, not booleans. Octal literals (e.g., `0777`) are treated as strings unless the field type is numeric.
- Unknown top-level keys MUST produce a validation error: `<key>: unknown field; valid top-level fields are: name, platform, tags, include, machine, identity, secrets, packages, defaults, dock, shell, directories, custom_steps`

---

## Top-Level Fields

| Field | Go Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | `string` | Required | — | Human-readable label for the config |
| `platform` | `string` | Required | — | Target platform; must be one of `darwin`, `ubuntu`, `debian`, `any` |
| `tags` | `[]string` | Optional | `[]` | Tag annotations; inert in v1 |
| `include` | `[]string` | Optional | `[]` | List of local file paths to merge before validation |
| `machine` | `MachineConfig` | Optional | zero value | Machine identity settings |
| `identity` | `IdentityConfig` | Optional | zero value | Git and GitHub identity |
| `secrets` | `[]SecretEntry` | Optional | `[]` | Required environment variable declarations |
| `packages` | `PackagesConfig` | Optional | zero value | Package installation lists |
| `defaults` | `map[string]map[string]DefaultValue` | Optional | `{}` | macOS system defaults (domain → key → value) |
| `dock` | `DockConfig` | Optional | zero value | Dock layout (macOS only) |
| `shell` | `ShellConfig` | Optional | zero value | Shell configuration |
| `directories` | `[]string` | Optional | `[]` | Directories to create |
| `custom_steps` | `map[string]CustomStep` | Optional | `{}` | User-defined steps |

---

## Section: packages

### Structure

```
PackagesConfig
  brew    []PackageEntry    optional, default []
  cask    []PackageEntry    optional, default []
  apt     []PackageEntry    optional, default []
```

Omitting the `packages` section entirely is equivalent to a `PackagesConfig` with all three lists empty.

Unknown keys within `packages` MUST produce a validation error: `packages.<key>: unknown field; valid fields are: brew, cask, apt`

### PackageEntry — Accepted Forms

Each element of `packages.brew`, `packages.cask`, and `packages.apt` MUST be one of two legal YAML forms:

**Short form** (string scalar):
```yaml
- git
```

**Object form** (mapping):
```yaml
- name: terraform
  version: "1.7.5"
  pinned: true
```

Both forms MUST be accepted by the parser and MUST normalize to the same internal representation.

### PackageEntry — Normalization Rule

Short form normalizes as: `{name: <string>, version: "", pinned: false}`

### PackageEntry — Normalized Fields

**`packages.<list>[i].name`**
- Go type: `string`
- Required: yes (after normalization)
- Constraints: non-empty; no whitespace characters
- Validation error: `packages.<list>[<i>].name: must be a non-empty string without whitespace`

**`packages.<list>[i].version`**
- Go type: `string`
- Required: no
- Default: `""` (empty string; empty string means latest)
- Constraints:
  - When present, MUST be a quoted YAML string
  - When present, MUST NOT be empty
  - An unquoted numeric-looking value (e.g., `version: 3.11`, `version: 20`) is parsed by `gopkg.in/yaml.v3` as a float64 or int. The validator MUST detect this condition by inspecting the raw YAML node tag: if the node tag is `!!float` or `!!int` for a field that maps to `version`, the validator MUST produce an error
- Validation errors:
  - `packages.<list>[<i>].version: version values must be quoted strings (e.g., version: "3.11" not version: 3.11); unquoted numeric versions lose precision`
  - `packages.<list>[<i>].version: must not be empty if present`

**`packages.<list>[i].pinned`**
- Go type: `bool`
- Required: no
- Default: `false`
- Valid values: `true`, `false`
- When `true`, the `upgrade` command MUST NOT upgrade this package. Behavioral semantics are owned by the Platform Adapters spec.

### Duplicate Detection

Within each package list (`brew`, `cask`, `apt`), duplicate `name` values (after normalization, case-sensitive) MUST produce a validation error:
- `packages.<list>: duplicate package name "<name>"`

Duplicate detection is scoped per list. The same package name MAY appear in `brew` and `cask` without error.

---

## Section: defaults

### Structure

```
DefaultsConfig = map[string]map[string]DefaultValue
```

- Outer key: **domain** (string; e.g., `NSGlobalDomain`, `com.apple.dock`)
- Inner key: **preference key** (string; e.g., `autohide`, `tilesize`)
- Value: **DefaultValue** — one of `bool`, `int`, `float64`, or `string`

Omitting `defaults` entirely is equivalent to an empty map.

### Key Constraints

**Domain key**
- Type: string
- MUST NOT be empty
- No format validation of domain string is performed at schema level
- Validation error: `defaults.<domain>: domain key must not be empty`

**Preference key**
- Type: string
- MUST NOT be empty
- Validation error: `defaults.<domain>.<key>: preference key must not be empty`

### Value Type Rules

| YAML literal | Go type | Example |
|---|---|---|
| `true` / `false` | `bool` | `autohide: true` |
| Integer literal | `int` | `tilesize: 36` |
| Float literal | `float64` | `autohide-delay: 0.5` |
| Quoted string | `string` | `location: "~/Screenshots"` |
| Unquoted string (not a recognized scalar) | `string` | `FXPreferredViewStyle: Clmv` |
| `null` | — | (error) |

- YAML `null` values MUST produce an error: `defaults.<domain>.<key>: null values are not permitted`
- Value types other than `bool`, `int`, `float64`, and `string` MUST produce an error: `defaults.<domain>.<key>: unsupported value type <type>; must be bool, int, float, or string`

---

## Section: dock

### Structure

```
DockConfig
  apps    []string    optional, default []
```

Omitting `dock` entirely is valid and equivalent to a `DockConfig` with an empty `apps` list.

The `dock` section is only meaningful when `platform: darwin`. The schema does not enforce this restriction; the Platform Adapters spec governs runtime behavior.

### Fields

**`dock.apps`**
- Go type: `[]string`
- Required: no
- Default: `[]`
- Element constraints: each entry MUST be a non-empty string; elements represent application paths (e.g., `/Applications/Finder.app`); the schema does not validate that the path exists — that is a runtime concern
- Validation error: `dock.apps[<i>]: must be a non-empty string`

---

## Section: shell

### Structure

```
ShellConfig
  default       string      optional, default ""
  oh_my_zsh     bool        optional, default false
  theme         string      optional, default ""
  plugins       []string    optional, default []
```

Omitting `shell` entirely is valid and equivalent to a `ShellConfig` with all fields at their defaults.

### Fields

**`shell.default`**
- Go type: `string`
- Required: no
- Default: `""` (empty string; empty string means leave the shell unchanged)
- Allowed values when non-empty: `zsh`, `bash`, `fish`
- Validation error: `shell.default: invalid value "<actual>"; must be one of: zsh, bash, fish (or omit to leave shell unchanged)`

**`shell.oh_my_zsh`**
- Go type: `bool`
- Required: no
- Default: `false`
- Valid values: `true`, `false`

**`shell.theme`**
- Go type: `string`
- Required: no
- Default: `""` (empty string; empty string means use the oh-my-zsh default theme)
- Constraints: MUST NOT be empty if the key is present
- Validation error: `shell.theme: must not be empty if present`

**`shell.plugins`**
- Go type: `[]string`
- Required: no
- Default: `[]`
- Element constraints: each plugin MUST be a non-empty string without whitespace characters
- Validation error: `shell.plugins[<i>]: must be a non-empty string without whitespace`

---

## Section: directories

### Structure

- Go type: `[]string`
- Required: no
- Default: `[]`

Omitting `directories` entirely is equivalent to an empty list.

### Element Constraints

- Each entry MUST be a non-empty string
- Tilde expansion is permitted (`~/github` is a valid entry)
- The schema does not validate that the path is creatable — that is a runtime concern
- Validation error: `directories[<i>]: must be a non-empty string`

### Duplicate Detection

Duplicate entries MUST produce a validation **warning** (not an error):
- `directories: duplicate entry "<path>"`

Duplicate detection is case-sensitive and does not expand tildes for comparison.

---

## Section: identity

### Structure

```
IdentityConfig
  git_name      string    optional
  git_email     string    optional
  github_user   string    optional
```

Omitting `identity` entirely is valid. All fields are individually optional. No cross-field validation is performed at the schema level — behavioral constraints belong to the git-config built-in step spec.

### Fields

**`identity.git_name`**
- Go type: `string`
- Required: no
- Constraints: MUST NOT be empty if the key is present
- Validation error: `identity.git_name: must not be empty if present`

**`identity.git_email`**
- Go type: `string`
- Required: no
- Constraints: MUST NOT be empty if the key is present; SHOULD contain the `@` character
- Validation error: `identity.git_email: must not be empty if present`
- Validation warning: `identity.git_email: value does not look like an email address`

**`identity.github_user`**
- Go type: `string`
- Required: no
- Constraints: MUST NOT be empty if the key is present; MUST NOT contain whitespace characters
- Validation error: `identity.github_user: must not be empty if present` / `identity.github_user: must not contain whitespace`

---

## Section: machine

### Structure

```
MachineConfig
  hostname    string    optional
```

Omitting `machine` entirely is valid.

### Fields

**`machine.hostname`**
- Go type: `string`
- Required: no
- Constraints: MUST NOT be empty if the key is present; MUST be a valid hostname per RFC 1123 (labels are alphanumeric and hyphens only; maximum 63 characters per label; maximum 253 characters total; labels MUST NOT start or end with a hyphen)
- Validation error: `machine.hostname: invalid hostname "<actual>"; must match RFC 1123`

---

## Section: secrets

### Structure

Omitting `secrets` entirely is equivalent to an empty list.

```
SecretEntry
  name          string    required
  description   string    optional, default ""
  required      bool      optional, default true
  sensitive     bool      optional, default false
  validate      string    optional, default ""
  prompt        bool      optional, default false
```

The `secrets` value is a YAML sequence of `SecretEntry` mappings.

### Fields

**`secrets[i].name`**
- Go type: `string`
- Required: yes
- Constraints: MUST NOT be empty; MUST match the pattern `[A-Z][A-Z0-9_]*`; MUST be unique across all entries in the `secrets` list
- Validation errors:
  - `secrets[<i>].name: required field is missing`
  - `secrets[<i>].name: must match pattern [A-Z][A-Z0-9_]* (e.g., GITHUB_TOKEN)`
  - `secrets[<i>].name: duplicate secret name "<name>"; each secret must be unique`

**`secrets[i].description`**
- Go type: `string`
- Required: no
- Default: `""` (empty string)
- Constraints: none; free text

**`secrets[i].required`**
- Go type: `bool`
- Required: no
- Default: `true`
- Valid values: `true`, `false`

**`secrets[i].sensitive`**
- Go type: `bool`
- Required: no
- Default: `false`
- Valid values: `true`, `false`
- When `true`, the secret's value MUST be replaced with `***` in all output (logs, plan, render, error messages). Behavioral semantics are owned by the Secrets System spec.

**`secrets[i].validate`**
- Go type: `string`
- Required: no
- Default: `""` (empty string; empty string means no validation command)
- Constraints: the schema does not validate shell syntax; behavioral semantics are owned by the Secrets System spec

**`secrets[i].prompt`**
- Go type: `bool`
- Required: no
- Default: `false`
- Valid values: `true`, `false`
- When `true` and the secret is missing: in interactive mode, the tool MUST prompt for the value; in non-interactive mode, the secret is treated as missing. Behavioral semantics are owned by the Secrets System spec.

---

## Section: include

### Structure

- Go type: `[]string`
- Required: no
- Default: `[]`

Omitting `include` entirely is equivalent to an empty list.

### Element Constraints

- Each entry MUST be a non-empty string
- Each entry MUST be a local file path (relative or absolute)
- Remote URLs (`http://`, `https://`, or any URL scheme) MUST be rejected with an error
- Relative paths are resolved relative to the directory containing the file in which the `include` directive appears
- Validation errors:
  - `include[<i>]: path must not be empty`
  - `include[<i>]: remote URLs are not supported; only local file paths are allowed`

### Processing

Include files are resolved and merged before the parent config is validated. The complete merge semantics are defined in the Include/Merge spec. This spec treats `include` as a field declaration only.

---

## Section: custom_steps

### Structure

```
custom_steps = map[string]CustomStep
```

The map key is the **step name** — the canonical identifier used in `provides` and `requires` references.

Omitting `custom_steps` entirely is equivalent to an empty map.

```
CustomStep
  description   string              optional, default ""
  provides      []string            optional, default []
  requires      []string            optional, default []
  platform      []string            optional, default ["any"]
  check         string              optional, default ""
  apply         map[string]string   optional, default {}
  rollback      map[string]string   optional, default {}
  env           []string            optional, default []
  tags          []string            optional, default []
```

### Step Name Key Constraints

- MUST NOT be empty
- MUST match the pattern `[a-z][a-z0-9-]*` (lowercase letters, digits, and hyphens; no underscores; no spaces; MUST begin with a lowercase letter)
- MUST be unique across all keys in `custom_steps`
- Validation error: `custom_steps.<name>: step name must match pattern [a-z][a-z0-9-]* (e.g., my-go-tool)`

### Fields

**`custom_steps.<name>.description`**
- Go type: `string`
- Required: no
- Default: `""` (empty string)
- Constraints: none; free text

**`custom_steps.<name>.provides`**
- Go type: `[]string`
- Required: no
- Default: `[]`
- Element constraints: each entry MUST be a non-empty string without whitespace; entries MUST be unique within the list
- Validation error: `custom_steps.<name>.provides[<i>]: must be a non-empty string without whitespace`

**`custom_steps.<name>.requires`**
- Go type: `[]string`
- Required: no
- Default: `[]`
- Element constraints: each entry MUST be a non-empty string without whitespace
- Soft dependencies (`wants`) do not exist in v1; only `requires` is defined
- Validation error: `custom_steps.<name>.requires[<i>]: must be a non-empty string without whitespace`

**`custom_steps.<name>.platform`**
- Go type: `[]string`
- Required: no
- Default: `["any"]`
- Allowed values per element: `darwin`, `ubuntu`, `debian`, `any`
- Validation error: `custom_steps.<name>.platform[<i>]: invalid value "<actual>"; must be one of: darwin, ubuntu, debian, any`

**`custom_steps.<name>.check`**
- Go type: `string`
- Required: no
- Default: `""` (empty string)
- A step with an empty `check` field is treated as always unsatisfied — the `apply` command MUST always run for such a step
- Constraints: none beyond being a string; shell syntax is not validated at schema level

**`custom_steps.<name>.apply`**
- Go type: `map[string]string`
- Required: no
- Default: `{}`
- Key constraints: each key MUST be one of `darwin`, `ubuntu`, `debian`, `any`
- Value constraints: each value MUST be a non-empty string
- Validation errors:
  - `custom_steps.<name>.apply.<platform>: invalid platform key "<actual>"; must be one of: darwin, ubuntu, debian, any`
  - `custom_steps.<name>.apply.<platform>: command must not be empty`

**`custom_steps.<name>.rollback`**
- Go type: `map[string]string`
- Required: no
- Default: `{}`
- Key/value constraints: same as `apply`
- **v1 status**: The `rollback` field is accepted and validated but not executed in v1. It exists so that rollback commands can be documented in configs authored now without tooling support. Rollback execution is deferred to a future release.
- Validation errors: same pattern as `apply`:
  - `custom_steps.<name>.rollback.<platform>: invalid platform key "<actual>"; must be one of: darwin, ubuntu, debian, any`
  - `custom_steps.<name>.rollback.<platform>: command must not be empty`

**`custom_steps.<name>.env`**
- Go type: `[]string`
- Required: no
- Default: `[]`
- Element constraints: each entry MUST be a non-empty string matching the pattern `[A-Z][A-Z0-9_]*`
- Validation error: `custom_steps.<name>.env[<i>]: must match pattern [A-Z][A-Z0-9_]* (e.g., GITHUB_TOKEN)`

**`custom_steps.<name>.tags`**
- Go type: `[]string`
- Required: no
- Default: `[]`
- Element constraints: each entry MUST be a non-empty string without whitespace characters
- **v1 status**: inert metadata — tags are parsed, validated, and stored, but tag-based filtering is not executed in v1. Tag filtering (e.g., `apply --tags dev`) is deferred to v2. Tags are accepted and validated to allow configs authored now to work in a future release without modification.
- Validation error: `custom_steps.<name>.tags[<i>]: must be a non-empty string without whitespace`

---

## Section: tags

**`tags`** (root-level field)
- Go type: `[]string`
- Required: no
- Default: `[]`
- Element constraints: each tag MUST be a non-empty string without whitespace characters
- **v1 status**: inert metadata — tags are parsed, validated, and stored, but tag-based filtering is not executed in v1. Tag filtering (e.g., `apply --tags dev`) is deferred to v2. Tags are accepted and validated to allow configs authored now to work in a future release without modification.
- Validation error: `tags[<i>]: tag must be a non-empty string without whitespace`

---

## Validation Rules

### Syntax

- **SYN-01**: The file MUST be valid YAML 1.2. A YAML syntax error MUST produce an error message including the line and column number where parsing failed. Syntax errors terminate validation immediately — no further validation is performed.
- **SYN-02**: The top-level document MUST be a YAML mapping. Any other YAML type MUST produce: `config: top-level document must be a YAML mapping`

### Structure

- **STR-01**: Unknown top-level keys MUST produce an error identifying the unknown key and listing all valid keys.
- **STR-02**: Unknown keys within any section mapping MUST produce an error identifying the unknown key and the section it was found in.
- **STR-03**: Fields declared as mappings MUST NOT receive a scalar or sequence value.
- **STR-04**: Fields declared as sequences MUST NOT receive a scalar or mapping value.
- **STR-05**: Fields declared as scalars MUST NOT receive a mapping or sequence value.
- **STR-06**: Required fields that are absent MUST produce a validation error.

### Types

- **TYP-01**: A field with an expected Go type of `bool` MUST reject any non-boolean YAML value. Error: `<field>: expected bool, got <type>`
- **TYP-02**: A field with an expected Go type of `string` MUST reject non-string YAML values. Error: `<field>: expected string, got <type>`
- **TYP-03**: The `version` field within any `PackageEntry` MUST have a YAML node tag of `!!str`. If the YAML node tag is `!!float` or `!!int`, the validator MUST produce: `packages.<list>[<i>].version: version values must be quoted strings (e.g., version: "3.11" not version: 3.11); unquoted numeric versions lose precision`
- **TYP-04**: `null` YAML values MUST be rejected for all fields. The error message MUST identify the field path.
- **TYP-05**: Values in `defaults.<domain>.<key>` MUST be one of `bool`, `int`, `float64`, or `string`. All other types MUST produce an error.

### Semantics

- **SEM-01**: `platform` MUST be one of `darwin`, `ubuntu`, `debian`, `any`. Any other value MUST produce: `platform: invalid value "<actual>"; must be one of: darwin, ubuntu, debian, any`
- **SEM-02**: `name` MUST NOT be empty. `name` MUST NOT exceed 255 characters.
- **SEM-03**: Each element of `tags` (root-level) MUST be a non-empty string without whitespace.
- **SEM-04**: Each element of `include` MUST be a non-empty local path. Remote URL schemes MUST be rejected.
- **SEM-05**: `machine.hostname`, if present, MUST be non-empty and MUST conform to RFC 1123.
- **SEM-06**: `identity.git_name`, if present, MUST be non-empty.
- **SEM-07**: `identity.git_email`, if present, MUST be non-empty. If present and not containing `@`, a warning MUST be emitted.
- **SEM-08**: `identity.github_user`, if present, MUST be non-empty and MUST NOT contain whitespace.
- **SEM-09**: Each `secrets[i].name` MUST be non-empty, MUST match `[A-Z][A-Z0-9_]*`, and MUST be unique within the `secrets` list.
- **SEM-10**: Package names (after normalization) MUST be non-empty and MUST NOT contain whitespace.
- **SEM-11**: Package `version` values, if present, MUST be non-empty quoted strings. Unquoted numeric versions MUST be rejected (see TYP-03).
- **SEM-12**: Package names MUST be unique within each list (`brew`, `cask`, `apt` are checked independently).
- **SEM-13**: `defaults` domain keys MUST be non-empty. Preference keys MUST be non-empty.
- **SEM-14**: `dock.apps` elements MUST be non-empty strings.
- **SEM-15**: `shell.default`, if non-empty, MUST be one of `zsh`, `bash`, `fish`.
- **SEM-16**: `shell.theme`, if present, MUST be non-empty.
- **SEM-17**: `shell.plugins` elements MUST be non-empty strings without whitespace.
- **SEM-18**: `directories` elements MUST be non-empty strings. Duplicate entries MUST produce a warning (not an error).
- **SEM-19**: Custom step name keys MUST match `[a-z][a-z0-9-]*` and MUST be unique within `custom_steps`.
- **SEM-20**: `custom_steps.<name>.platform` elements MUST each be one of `darwin`, `ubuntu`, `debian`, `any`.
- **SEM-21**: `custom_steps.<name>.apply` and `custom_steps.<name>.rollback` keys MUST each be one of `darwin`, `ubuntu`, `debian`, `any`. Values MUST be non-empty strings.
- **SEM-22**: `custom_steps.<name>.env` elements MUST match `[A-Z][A-Z0-9_]*`.
- **SEM-23**: `custom_steps.<name>.provides` elements MUST be unique within the list.
- **SEM-24**: `custom_steps.<name>.tags` elements MUST be non-empty strings without whitespace.

### Validation Execution Order

The validator MUST implement the validate-all-report-all pattern. All errors MUST be collected before reporting. Validation proceeds in the following order:

1. YAML syntax check (terminates on failure; no further validation is possible)
2. Top-level structure check: unknown top-level keys, top-level type check
3. Per-section structure checks: unknown keys within sections, missing required fields, field type checks
4. Type-level checks: including version-as-float detection (TYP-03)
5. Enum value validation: `platform`, `shell.default`, `custom_steps.<name>.platform`, `apply`/`rollback` map keys
6. Cross-field and uniqueness checks: duplicate package names, duplicate secret names, duplicate `provides` entries

After step 1, all errors from steps 2–6 MUST be collected in a single pass and reported together.

---

## Error Messages

### Format

Every validation error message MUST contain:
1. The field path (dot-separated, with zero-based array indices for sequences): e.g., `packages.brew[2].version`
2. A description of the expected constraint or type
3. The actual value that failed, quoted or formatted to make it unambiguous

Error messages MUST NOT be truncated. Long actual values SHOULD be truncated at 200 characters with `...` appended.

Warnings use the same format and field path conventions as errors but are labeled as warnings in output.

### Error Catalog

| Code | Message Template |
|---|---|
| E001 | `config: top-level document must be a YAML mapping` |
| E002 | `<key>: unknown field; valid top-level fields are: name, platform, tags, include, machine, identity, secrets, packages, defaults, dock, shell, directories, custom_steps` |
| E003 | `name: required field is missing` |
| E004 | `name: must not be empty` |
| E005 | `name: must not exceed 255 characters` |
| E006 | `platform: required field is missing` |
| E007 | `platform: invalid value "<actual>"; must be one of: darwin, ubuntu, debian, any` |
| E008 | `tags[<i>]: tag must be a non-empty string without whitespace` |
| E009 | `include[<i>]: path must not be empty` |
| E010 | `include[<i>]: remote URLs are not supported; only local file paths are allowed` |
| E011 | `machine.hostname: invalid hostname "<actual>"; must match RFC 1123` |
| E012 | `identity.git_name: must not be empty if present` |
| E013 | `identity.git_email: must not be empty if present` |
| W001 | `identity.git_email: value does not look like an email address` |
| E014 | `identity.github_user: must not be empty if present` |
| E015 | `identity.github_user: must not contain whitespace` |
| E016 | `secrets[<i>].name: required field is missing` |
| E017 | `secrets[<i>].name: must match pattern [A-Z][A-Z0-9_]* (e.g., GITHUB_TOKEN)` |
| E018 | `secrets[<i>].name: duplicate secret name "<name>"; each secret must be unique` |
| E019 | `packages.<list>[<i>].name: must be a non-empty string without whitespace` |
| E020 | `packages.<list>[<i>].version: version values must be quoted strings (e.g., version: "3.11" not version: 3.11); unquoted numeric versions lose precision` |
| E021 | `packages.<list>[<i>].version: must not be empty if present` |
| E022 | `packages.<list>: duplicate package name "<name>"` |
| E023 | `defaults.<domain>: domain key must not be empty` |
| E024 | `defaults.<domain>.<key>: preference key must not be empty` |
| E025 | `defaults.<domain>.<key>: null values are not permitted` |
| E026 | `defaults.<domain>.<key>: unsupported value type <type>; must be bool, int, float, or string` |
| E027 | `dock.apps[<i>]: must be a non-empty string` |
| E028 | `shell.default: invalid value "<actual>"; must be one of: zsh, bash, fish (or omit to leave shell unchanged)` |
| E029 | `shell.theme: must not be empty if present` |
| E030 | `shell.plugins[<i>]: must be a non-empty string without whitespace` |
| E031 | `directories[<i>]: must be a non-empty string` |
| W002 | `directories: duplicate entry "<path>"` |
| E032 | `custom_steps.<name>: step name must match pattern [a-z][a-z0-9-]* (e.g., my-go-tool)` |
| E033 | `custom_steps.<name>.provides[<i>]: must be a non-empty string without whitespace` |
| E034 | `custom_steps.<name>.requires[<i>]: must be a non-empty string without whitespace` |
| E035 | `custom_steps.<name>.platform[<i>]: invalid value "<actual>"; must be one of: darwin, ubuntu, debian, any` |
| E036 | `custom_steps.<name>.apply.<platform>: invalid platform key "<actual>"; must be one of: darwin, ubuntu, debian, any` |
| E037 | `custom_steps.<name>.apply.<platform>: command must not be empty` |
| E038 | `custom_steps.<name>.rollback.<platform>: invalid platform key "<actual>"; must be one of: darwin, ubuntu, debian, any` |
| E039 | `custom_steps.<name>.rollback.<platform>: command must not be empty` |
| E040 | `custom_steps.<name>.env[<i>]: must match pattern [A-Z][A-Z0-9_]* (e.g., GITHUB_TOKEN)` |
| E041 | `custom_steps.<name>.tags[<i>]: must be a non-empty string without whitespace` |
| E042 | `<field>: expected <type>, got <actual_type>` |

---

## Complete Example Config

```yaml
name: "Greg's MacBook Pro"
platform: darwin
tags: [dev, personal]

include:
  - shared/git.yaml
  - shared/shell.yaml

machine:
  hostname: greg-mbp

identity:
  git_name: "Greg Berns"
  git_email: "greg@example.com"
  github_user: gregberns

secrets:
  - name: GITHUB_TOKEN
    description: "GitHub personal access token for private repos"
    required: true
    sensitive: true
    validate: "gh auth status"
    prompt: false
  - name: NPM_TOKEN
    description: "npm registry auth token"
    required: false
    sensitive: true
    prompt: true
  - name: GOPRIVATE
    description: "Go private module path pattern"
    required: false
    sensitive: false

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
      pinned: true
  cask:
    - iterm2
    - vscodium
    - google-chrome
    - slack
    - flux
    - diffmerge
  apt: []

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
    location: "~/Screenshots"
    type: png

dock:
  apps:
    - /Applications/Google Chrome.app
    - /Applications/VSCodium.app
    - /Applications/iTerm.app

shell:
  default: zsh
  oh_my_zsh: true
  theme: brad-muse
  plugins:
    - zsh-syntax-highlighting

directories:
  - ~/github
  - ~/gitlab
  - ~/screenshots

custom_steps:
  my-go-tool:
    description: "Internal Go tool for project scaffolding"
    provides: [my-go-tool]
    requires: [go]
    platform: [darwin, ubuntu]
    check: "command -v my-go-tool"
    apply:
      darwin: "go install git.internal.com/tools/my-go-tool@latest"
      ubuntu: "go install git.internal.com/tools/my-go-tool@latest"
    rollback:
      darwin: "rm -f $(go env GOPATH)/bin/my-go-tool"
      ubuntu: "rm -f $(go env GOPATH)/bin/my-go-tool"
    env: [GOPRIVATE]
    tags: [dev]
  configure-ssh-agent:
    description: "Configure SSH agent to load keys at login"
    provides: [ssh-agent-configured]
    requires: []
    platform: [darwin]
    check: "grep -q 'AddKeysToAgent' ~/.ssh/config 2>/dev/null"
    apply:
      darwin: "mkdir -p ~/.ssh && printf 'Host *\n  AddKeysToAgent yes\n  UseKeychain yes\n' >> ~/.ssh/config"
    env: []
    tags: []
```
