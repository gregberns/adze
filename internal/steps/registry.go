// Package steps provides the built-in step library, a registry for lookup,
// and config-to-StepConfig bindings for the adze machine configuration tool.
package steps

import (
	"sort"

	"github.com/gregberns/adze/internal/step"
)

// StepDefinition describes a built-in step's metadata and constructor.
type StepDefinition struct {
	Name          string
	Description   string
	Category      string   // "core", "packages", "languages", "shell", "system", "generic"
	Type          string   // "atomic" or "batch"
	Platforms     []string // e.g., ["darwin"], ["ubuntu", "debian"], ["any"]
	Provides      []string
	Requires      []string
	ConfigSection string // e.g., "packages.brew", "defaults", "shell.plugins", or ""
	Constructor   func() step.Step
}

// Registry maps step names to their definitions.
type Registry struct {
	steps map[string]StepDefinition
	order []string // insertion order for deterministic iteration
}

// NewRegistry creates a new Registry pre-populated with all built-in steps.
func NewRegistry() *Registry {
	r := &Registry{
		steps: make(map[string]StepDefinition),
	}
	registerAll(r)
	return r
}

// Register adds a step definition to the registry.
func (r *Registry) Register(def StepDefinition) {
	if _, exists := r.steps[def.Name]; !exists {
		r.order = append(r.order, def.Name)
	}
	r.steps[def.Name] = def
}

// Get returns the definition for the named step, or false if not found.
func (r *Registry) Get(name string) (StepDefinition, bool) {
	def, ok := r.steps[name]
	return def, ok
}

// All returns all step definitions in a stable order (sorted by category then name).
func (r *Registry) All() []StepDefinition {
	defs := make([]StepDefinition, 0, len(r.steps))
	for _, name := range r.order {
		defs = append(defs, r.steps[name])
	}
	sort.SliceStable(defs, func(i, j int) bool {
		ci := categoryOrder(defs[i].Category)
		cj := categoryOrder(defs[j].Category)
		if ci != cj {
			return ci < cj
		}
		return defs[i].Name < defs[j].Name
	})
	return defs
}

// ForPlatform returns all step definitions that support the given platform.
// A step matches if its Platforms list contains the platform or "any".
func (r *Registry) ForPlatform(platform string) []StepDefinition {
	all := r.All()
	var result []StepDefinition
	for _, def := range all {
		if platformMatches(def.Platforms, platform) {
			result = append(result, def)
		}
	}
	return result
}

// Count returns the total number of registered steps.
func (r *Registry) Count() int {
	return len(r.steps)
}

// platformMatches returns true if the given platform matches any in the platforms list.
func platformMatches(platforms []string, platform string) bool {
	for _, p := range platforms {
		if p == "any" || p == platform {
			return true
		}
	}
	return false
}

// categoryOrder returns a sort key for step categories.
func categoryOrder(category string) int {
	switch category {
	case "core":
		return 0
	case "packages":
		return 1
	case "languages":
		return 2
	case "shell":
		return 3
	case "system":
		return 4
	case "generic":
		return 5
	default:
		return 6
	}
}
