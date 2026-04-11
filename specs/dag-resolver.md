# DAG Resolver Specification

## Overview

The DAG resolver takes a set of steps (built-in and custom, filtered to the current platform) and produces a deterministic execution order by resolving provides/requires relationships into a directed acyclic graph.

## Input

The resolver receives a list of `Step` objects from the merged configuration. Each step has:
- `Provides []string` — capabilities this step makes available
- `Requires []string` — capabilities this step needs before it can run
- `Platform []string` — platforms this step supports

Before graph construction, steps are filtered to those matching the current runtime platform.

## Graph Construction

1. For each step S, for each capability C in S.Provides: register C → S in a provides-map.
2. For each step S, for each capability R in S.Requires: look up R in the provides-map. If found, create a directed edge from the providing step to S.

## Validation

The resolver MUST perform all validation before topological sort. All errors MUST be collected and reported together (not fail-on-first).

### Duplicate Provides

If two or more steps declare the same capability in their `Provides` list, the resolver MUST report an error:

```
Error: duplicate provider for "<capability>"
  Provided by: <step-a> (<built-in|custom>)
  Provided by: <step-b> (<built-in|custom>)
  Fix: remove one of these steps or rename the capability
```

### Unresolved Requires

If a step's `Requires` list contains a capability that no step provides, the resolver MUST report an error. If a built-in step provides the missing capability, the error MUST include a suggestion:

```
Error: unresolved dependency "<capability>"
  Required by: <step-name>
  Suggestion: add the built-in step "<step>" which provides "<capability>"
```

If no built-in step provides it:

```
Error: unresolved dependency "<capability>"
  Required by: <step-name>
  No built-in step provides this capability. Define a custom step with provides: [<capability>]
```

### Platform-Filtered Breakage

If a step required by other steps is filtered out due to platform mismatch, the resolver MUST report an error:

```
Error: step "<required-step>" is required by "<dependent-step>" but does not support platform "<current-platform>"
  "<required-step>" only supports: <platforms>
```

## Topological Sort

The resolver MUST use Kahn's algorithm (BFS-based topological sort):

1. Compute in-degree for every node.
2. Initialize a priority queue with all zero-in-degree nodes, sorted alphabetically by step name.
3. While the queue is not empty:
   a. Dequeue the alphabetically-first node N.
   b. Append N to the result list.
   c. For each node M that N has an edge to: decrement M's in-degree. If M's in-degree becomes 0, enqueue M.
4. If the result list length is less than the total node count, a cycle exists.

### Cycle Detection

When unprocessed nodes remain after the algorithm completes, the resolver MUST:
1. Collect all unprocessed nodes (these are in or downstream of cycles).
2. Extract the cycle path by running DFS within the residual subgraph.
3. Report the cycle:

```
Error: dependency cycle detected
  <step-a> requires <capability-x> (provided by <step-b>)
  <step-b> requires <capability-y> (provided by <step-a>)
  Cycle: <step-a> → <step-b> → <step-a>
```

### Determinism

When multiple nodes have in-degree 0 simultaneously, they MUST be processed in alphabetical order by step name. This guarantees deterministic output across runs.

## Output

```go
type ResolvedGraph struct {
    Steps    []ResolvedStep
    Warnings []Warning
}

type ResolvedStep struct {
    Step       Step
    DependsOn  map[string]string  // capability → providing step name
    RequiredBy map[string]string  // capability → requiring step name
    Depth      int                // 0 = root (no dependencies)
}
```

The `Steps` list is in execution order (topologically sorted). `DependsOn` and `RequiredBy` are keyed by capability name to support correct skip-propagation.

## Skip Propagation

During execution, when a step fails:
1. Identify all capabilities in the failed step's `Provides`.
2. For each downstream step that transitively requires any of those capabilities: mark as `skipped`.
3. The skip message MUST name the upstream failure:

```
[skip] <step-name> — skipped because "<capability>" (provided by <failed-step>) failed
```

Steps that do not depend on the failed step MUST continue normally.

## Plan Exit Codes

When used by the `plan` command:
- Exit code 0: no changes needed (all steps already satisfied)
- Exit code 6: changes would be made (at least one step is not satisfied)
