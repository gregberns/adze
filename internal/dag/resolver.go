package dag

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"
)

// Resolve builds the dependency graph and returns a topologically sorted execution order.
// platform is the current runtime platform (e.g., "darwin", "ubuntu").
func Resolve(steps []StepInput, platform string, known KnownBuiltIn) (*ResolvedGraph, []error) {
	if known == nil {
		known = KnownBuiltIn{}
	}

	// Phase 1: Platform filtering — separate active vs filtered-out steps.
	active, filtered := filterByPlatform(steps, platform)

	// Build a lookup for filtered-out steps by name.
	filteredByName := make(map[string]*StepInput, len(filtered))
	for i := range filtered {
		filteredByName[filtered[i].Name] = &filtered[i]
	}

	// Phase 2: Build provides-map and collect validation errors.
	var errs []error

	// provides-map: capability → step name
	providesMap := make(map[string]string)
	// Track all providers for duplicate detection (capability → list of step names).
	allProviders := make(map[string][]providerInfo)

	for i := range active {
		s := &active[i]
		for _, cap := range s.Provides {
			allProviders[cap] = append(allProviders[cap], providerInfo{name: s.Name, builtIn: s.BuiltIn})
		}
	}
	// Also consider filtered-out steps for platform-filtered breakage detection.
	for i := range filtered {
		s := &filtered[i]
		for _, cap := range s.Provides {
			// Only add to allProviders if not already there from active steps.
			// We need these for platform-filtered breakage but not for duplicate detection
			// among active steps.
			_ = s
			_ = cap
		}
	}

	// Check duplicate provides among active steps.
	for cap, providers := range allProviders {
		if len(providers) > 1 {
			var lines []string
			lines = append(lines, fmt.Sprintf("Error: duplicate provider for %q", cap))
			for _, p := range providers {
				kind := "custom"
				if p.builtIn {
					kind = "built-in"
				}
				lines = append(lines, fmt.Sprintf("  Provided by: %s (%s)", p.name, kind))
			}
			lines = append(lines, "  Fix: remove one of these steps or rename the capability")
			errs = append(errs, fmt.Errorf("%s", strings.Join(lines, "\n")))
		} else {
			providesMap[cap] = providers[0].name
		}
	}

	// Build filtered provides-map: capability → step name for filtered-out steps.
	filteredProvidesMap := make(map[string]string)
	for i := range filtered {
		s := &filtered[i]
		for _, cap := range s.Provides {
			filteredProvidesMap[cap] = s.Name
		}
	}

	// Phase 3: Build edges and check unresolved requires / platform-filtered breakage.
	// edges: from providing step name → list of dependent step names
	edges := make(map[string][]string)
	// reverse: from dependent step name → list of providing step names
	// Also track capability-level dependency info.
	type edgeInfo struct {
		capability   string
		providerName string
	}

	stepEdges := make(map[string][]edgeInfo)    // step name → edges coming into it
	revEdges := make(map[string][]edgeInfo)      // step name → edges going out of it (what it's required by)
	capabilityEdge := make(map[[2]string]string) // [from, to] → capability

	for i := range active {
		s := &active[i]
		for _, req := range s.Requires {
			provider, ok := providesMap[req]
			if ok {
				edges[provider] = append(edges[provider], s.Name)
				capabilityEdge[[2]string{provider, s.Name}] = req
				stepEdges[s.Name] = append(stepEdges[s.Name], edgeInfo{capability: req, providerName: provider})
				revEdges[provider] = append(revEdges[provider], edgeInfo{capability: req, providerName: s.Name})
				continue
			}

			// Check if a filtered-out step provides it (platform-filtered breakage).
			if filteredProvider, filteredOk := filteredProvidesMap[req]; filteredOk {
				fs := filteredByName[filteredProvider]
				errs = append(errs, fmt.Errorf(
					"Error: step %q is required by %q but does not support platform %q\n  %q only supports: %s",
					filteredProvider, s.Name, platform,
					filteredProvider, strings.Join(fs.Platforms, ", "),
				))
				continue
			}

			// Unresolved dependency.
			if builtInStep, exists := known[req]; exists {
				errs = append(errs, fmt.Errorf(
					"Error: unresolved dependency %q\n  Required by: %s\n  Suggestion: add the built-in step %q which provides %q",
					req, s.Name, builtInStep, req,
				))
			} else {
				errs = append(errs, fmt.Errorf(
					"Error: unresolved dependency %q\n  Required by: %s\n  No built-in step provides this capability. Define a custom step with provides: [%s]",
					req, s.Name, req,
				))
			}
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}

	// Phase 4: Kahn's algorithm — topological sort with alphabetical tie-breaking.
	stepByName := make(map[string]*StepInput, len(active))
	for i := range active {
		stepByName[active[i].Name] = &active[i]
	}

	inDegree := make(map[string]int, len(active))
	for i := range active {
		name := active[i].Name
		if _, exists := inDegree[name]; !exists {
			inDegree[name] = 0
		}
	}
	for _, targets := range edges {
		for _, t := range targets {
			inDegree[t]++
		}
	}

	// Priority queue (min-heap by name for alphabetical ordering).
	pq := &nameHeap{}
	heap.Init(pq)
	for name, deg := range inDegree {
		if deg == 0 {
			heap.Push(pq, name)
		}
	}

	var sorted []string
	for pq.Len() > 0 {
		n := heap.Pop(pq).(string)
		sorted = append(sorted, n)

		// Sort edges for determinism.
		targets := edges[n]
		sort.Strings(targets)
		for _, m := range targets {
			inDegree[m]--
			if inDegree[m] == 0 {
				heap.Push(pq, m)
			}
		}
	}

	// Phase 5: Cycle detection.
	if len(sorted) < len(active) {
		// Find cycle via DFS in residual subgraph.
		residual := make(map[string]bool, len(active)-len(sorted))
		sortedSet := make(map[string]bool, len(sorted))
		for _, name := range sorted {
			sortedSet[name] = true
		}
		for i := range active {
			if !sortedSet[active[i].Name] {
				residual[active[i].Name] = true
			}
		}

		cycle := findCycle(residual, edges, capabilityEdge)
		if cycle != nil {
			var lines []string
			lines = append(lines, "Error: dependency cycle detected")
			for i := 0; i < len(cycle)-1; i++ {
				from := cycle[i]
				to := cycle[i+1]
				cap := capabilityEdge[[2]string{from, to}]
				lines = append(lines, fmt.Sprintf("  %s requires %s (provided by %s)", to, cap, from))
			}
			// Build cycle path string.
			cyclePath := strings.Join(cycle, " \u2192 ")
			lines = append(lines, fmt.Sprintf("  Cycle: %s", cyclePath))
			errs = append(errs, fmt.Errorf("%s", strings.Join(lines, "\n")))
		}
		return nil, errs
	}

	// Phase 6: Compute depth and build ResolvedStep entries.
	depthMap := make(map[string]int, len(sorted))
	for _, name := range sorted {
		d := 0
		for _, ei := range stepEdges[name] {
			providerDepth := depthMap[ei.providerName]
			if providerDepth+1 > d {
				d = providerDepth + 1
			}
		}
		depthMap[name] = d
	}

	result := &ResolvedGraph{
		Steps: make([]ResolvedStep, 0, len(sorted)),
	}

	for _, name := range sorted {
		s := stepByName[name]

		dependsOn := make(map[string]string)
		for _, ei := range stepEdges[name] {
			dependsOn[ei.capability] = ei.providerName
		}

		requiredBy := make(map[string]string)
		for _, ei := range revEdges[name] {
			requiredBy[ei.capability] = ei.providerName
		}

		result.Steps = append(result.Steps, ResolvedStep{
			Name:       name,
			Config:     *s,
			BuiltIn:    s.BuiltIn,
			DependsOn:  dependsOn,
			RequiredBy: requiredBy,
			Depth:      depthMap[name],
		})
	}

	return result, nil
}

// filterByPlatform splits steps into active (matching platform) and filtered-out.
// A step with an empty Platforms list matches all platforms.
func filterByPlatform(steps []StepInput, platform string) (active, filtered []StepInput) {
	for _, s := range steps {
		if len(s.Platforms) == 0 || containsString(s.Platforms, platform) || containsString(s.Platforms, "any") {
			active = append(active, s)
		} else {
			filtered = append(filtered, s)
		}
	}
	return
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

type providerInfo struct {
	name    string
	builtIn bool
}

// findCycle uses DFS in the residual subgraph to extract a cycle path.
func findCycle(residual map[string]bool, edges map[string][]string, capEdge map[[2]string]string) []string {
	// Sort residual node names for deterministic cycle reporting.
	var nodes []string
	for n := range residual {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	visited := make(map[string]bool)
	onStack := make(map[string]bool)
	parent := make(map[string]string)

	var cyclePath []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		onStack[node] = true

		targets := edges[node]
		sortedTargets := make([]string, len(targets))
		copy(sortedTargets, targets)
		sort.Strings(sortedTargets)

		for _, next := range sortedTargets {
			if !residual[next] {
				continue
			}
			if !visited[next] {
				parent[next] = node
				if dfs(next) {
					return true
				}
			} else if onStack[next] {
				// Found cycle — reconstruct path.
				cyclePath = []string{next}
				cur := node
				for cur != next {
					cyclePath = append([]string{cur}, cyclePath...)
					cur = parent[cur]
				}
				cyclePath = append([]string{next}, cyclePath...)
				// Reverse so it reads naturally: next → ... → node → next
				// Actually cyclePath is already: next, ..., node, next
				// But we built it backwards. Let me rebuild properly.
				cyclePath = nil
				path := []string{node}
				cur = node
				for cur != next {
					cur = parent[cur]
					path = append(path, cur)
				}
				// Reverse path so it goes from next → ... → node
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				// Append next again to close the cycle.
				path = append(path, next)
				cyclePath = path
				return true
			}
		}
		onStack[node] = false
		return false
	}

	for _, node := range nodes {
		if !visited[node] {
			if dfs(node) {
				return cyclePath
			}
		}
	}
	return nil
}

// nameHeap is a min-heap of strings for alphabetical priority queue.
type nameHeap []string

func (h nameHeap) Len() int           { return len(h) }
func (h nameHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h nameHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *nameHeap) Push(x interface{}) {
	*h = append(*h, x.(string))
}

func (h *nameHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
