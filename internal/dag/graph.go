package dag

// StepInput represents a step for DAG resolution.
type StepInput struct {
	Name      string
	Provides  []string
	Requires  []string
	Platforms []string
	BuiltIn   bool
}

// KnownBuiltIn maps capabilities to their providing built-in step names.
// Used for suggestion messages on unresolved requires.
type KnownBuiltIn map[string]string

// ResolvedGraph holds the topologically sorted execution plan.
type ResolvedGraph struct {
	Steps    []ResolvedStep
	Warnings []string
}

// ResolvedStep is a single step in the resolved execution order.
type ResolvedStep struct {
	Name       string
	Config     StepInput         // the step's config
	BuiltIn    bool              // true for built-in steps, false for custom
	DependsOn  map[string]string // capability → providing step name
	RequiredBy map[string]string // capability → requiring step name
	Depth      int               // 0 = root (no dependencies)
}
