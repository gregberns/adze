# Include/Merge System Specification

## Overview

Configs support an `include` directive that composes multiple YAML files into a single merged configuration. Includes are processed before any other config resolution.

## Include Directive

The `include` field is an optional list of relative file paths at the config root:

```yaml
include:
  - shared/identity.yaml
  - shared/shell.yaml
```

Each path is resolved relative to the directory containing the including file.

## Resolution Order

1. For the main config file, process includes in listed order (first to last).
2. Each included file's own `include` directives are resolved recursively (depth-first).
3. The main config is applied last, overriding all includes.

Within includes, later entries override earlier entries on conflict.

## Merge Rules

The merge function operates recursively on two YAML nodes: `base` (from include) and `override` (from later include or main config).

### Scalars

Override wins. The resulting value is the override value.

### Maps

Deep merge recursively. For each key:
- If the key exists only in base: keep base value.
- If the key exists only in override: use override value.
- If the key exists in both: recursively merge.

### Lists

Concatenate base and override, then deduplicate:
- **String items**: deduplicate by exact string match. First occurrence is kept.
- **Object items with a `name` field**: deduplicate by `name` value. Later occurrence (override) replaces earlier (base).
- **Mixed string and object items**: strings are normalized to objects (`"git"` → `{name: "git"}`) before deduplication. Object form wins over string form.
- **Items without a `name` field**: concatenate without deduplication.

### Null Override

If the override value is `null`, the key is removed from the merged result. This allows an including config to explicitly delete a key defined by an include.

### Type Mismatch

If base and override define the same key with different YAML types (e.g., base defines it as a list, override as a scalar):
- The override value wins.
- A warning MUST be emitted:

```
Warning: type mismatch at "<field-path>"
  <include-file> defines it as <type>
  <override-file> defines it as <type>
  Using <override-file>'s value
```

## Circular Include Detection

The resolver MUST maintain a set of absolute file paths visited during resolution. Before processing an include:
1. Resolve the path to an absolute path.
2. If the path is in the visited set: report an error.
3. Add the path to the visited set.
4. Process the include.

The visited set is passed through recursion (not cleared between siblings).

Error format:

```
Error: circular include detected
  <file-a> includes <file-b>
  <file-b> includes <file-c>
  <file-c> includes <file-a>  ← cycle
```

## Depth Limit

Include nesting MUST NOT exceed 10 levels. If the limit is reached:

```
Error: include depth limit (10) exceeded
  Chain: <file-1> → <file-2> → ... → <file-11>
  This usually indicates a circular include
```

## Missing Include File

If a referenced include file does not exist:

```
Error: include file not found
  Referenced in: <including-file> (line <N>)
  Path: <relative-path>
  Resolved to: <absolute-path>
```

## Remote Includes

URLs (paths starting with `http://` or `https://`) are NOT supported in v1. If a URL is detected:

```
Error: remote includes are not supported
  Path: <url>
  Use a local file path instead
```

## Include in URL-sourced Configs

When a config is loaded from a URL (`--config https://...`), includes are not supported because includes require local files. If a URL-sourced config contains an `include` directive, the tool MUST report an error during validation (before execution):

```
Error: URL-sourced configs cannot use includes
  Config loaded from: <url>
  Includes require local file paths
  Download the config and its includes locally, then use --config with a local path
```

This validation MUST occur during the config validation phase (alongside schema validation), using the same error code as other config errors (exit code 2).
