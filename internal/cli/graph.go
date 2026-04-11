package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/steps"
	"github.com/spf13/cobra"
)

func newGraphCmdImpl() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Visualize the dependency graph",
		Long:  "Display the dependency graph in text tree format (default) or DOT format for Graphviz.",
		RunE:  runGraph,
	}
	cmd.Flags().String("format", "text", "output format: text or dot")
	return cmd
}

func runGraph(cmd *cobra.Command, args []string) error {
	gf := ResolveGlobalFlags(cmd)
	format, _ := cmd.Flags().GetString("format")

	if format != "text" && format != "dot" {
		return fmt.Errorf("invalid format %q; must be text or dot", format)
	}

	configPath, err := ResolveConfig(gf.Config)
	if err != nil {
		return err
	}

	cfg, valErrs, _, loadErr := config.LoadConfig(configPath, false)
	if loadErr != nil {
		return fmt.Errorf("loading config: %w", loadErr)
	}
	if len(valErrs) > 0 || cfg == nil {
		return fmt.Errorf("config has validation errors; run 'adze validate' for details")
	}

	platform := resolvePlatform(cfg.Platform)
	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, platform, reg)

	var dagInputs []dag.StepInput
	for _, sc := range stepConfigs {
		dagInputs = append(dagInputs, dag.StepInput{
			Name:      sc.Name,
			Provides:  sc.Provides,
			Requires:  sc.Requires,
			Platforms: sc.Platforms,
			BuiltIn:   !isCustomStep(sc.Name, cfg),
		})
	}

	knownBuiltIns := buildKnownBuiltIns(reg)
	graph, dagErrs := dag.Resolve(dagInputs, platform, knownBuiltIns)
	if len(dagErrs) > 0 {
		return fmt.Errorf("dependency resolution failed; run 'adze validate' for details")
	}

	var output string
	switch format {
	case "dot":
		output = renderDot(graph)
	default:
		output = renderTextTree(graph)
	}

	fmt.Fprint(cmd.OutOrStdout(), output)
	return nil
}

// renderDot produces a Graphviz DOT representation of the dependency graph.
func renderDot(graph *dag.ResolvedGraph) string {
	var sb strings.Builder
	sb.WriteString("digraph {\n")

	// Collect edges and sort for deterministic output
	type edge struct {
		from, to string
	}
	var edges []edge

	for _, step := range graph.Steps {
		for _, provider := range step.DependsOn {
			edges = append(edges, edge{from: provider, to: step.Name})
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})

	for _, e := range edges {
		sb.WriteString(fmt.Sprintf("  %q -> %q;\n", e.from, e.to))
	}

	// Also include nodes with no edges (roots with no dependents)
	hasEdge := make(map[string]bool)
	for _, e := range edges {
		hasEdge[e.from] = true
		hasEdge[e.to] = true
	}
	var isolated []string
	for _, step := range graph.Steps {
		if !hasEdge[step.Name] {
			isolated = append(isolated, step.Name)
		}
	}
	sort.Strings(isolated)
	for _, name := range isolated {
		sb.WriteString(fmt.Sprintf("  %q;\n", name))
	}

	sb.WriteString("}\n")
	return sb.String()
}

// renderTextTree produces a text tree representation of the dependency graph.
func renderTextTree(graph *dag.ResolvedGraph) string {
	if len(graph.Steps) == 0 {
		return "(no steps)\n"
	}

	// Build adjacency list: parent -> children (dependency order means parent is the provider)
	children := make(map[string][]string)
	hasParent := make(map[string]bool)

	for _, step := range graph.Steps {
		for _, provider := range step.DependsOn {
			children[provider] = append(children[provider], step.Name)
			hasParent[step.Name] = true
		}
	}

	// Sort children for determinism
	for k := range children {
		sort.Strings(children[k])
	}

	// Find roots (steps with no parents)
	var roots []string
	for _, step := range graph.Steps {
		if !hasParent[step.Name] {
			roots = append(roots, step.Name)
		}
	}
	sort.Strings(roots)

	var sb strings.Builder
	for i, root := range roots {
		writeTree(&sb, root, "", i == len(roots)-1, children)
	}

	return sb.String()
}

// writeTree recursively writes a tree node with box-drawing characters.
func writeTree(sb *strings.Builder, name string, prefix string, isLast bool, children map[string][]string) {
	// Write the current node
	if prefix == "" {
		// Root node - no connector
		sb.WriteString(name + "\n")
	} else {
		connector := "\u251c\u2500\u2500 " // "├── "
		if isLast {
			connector = "\u2514\u2500\u2500 " // "└── "
		}
		sb.WriteString(prefix + connector + name + "\n")
	}

	// Write children
	kids := children[name]
	for i, child := range kids {
		childPrefix := prefix
		if prefix == "" {
			// Direct children of root
			childPrefix = ""
		} else if isLast {
			childPrefix = prefix + "    "
		} else {
			childPrefix = prefix + "\u2502   " // "│   "
		}
		writeTree(sb, child, childPrefix, i == len(kids)-1, children)
	}
}
