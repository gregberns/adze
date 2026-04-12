package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gregberns/adze/internal/steps"
	"github.com/spf13/cobra"
)

// newStepCmd creates the "step" parent command with subcommands.
func newStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "step",
		Short: "Manage built-in and custom steps",
		Long:  "List, inspect, and scaffold steps.",
	}

	cmd.AddCommand(newStepListCmd())
	cmd.AddCommand(newStepInfoCmd())
	cmd.AddCommand(newStepAddCmd())

	return cmd
}

// newStepListCmd creates the "step list" subcommand.
func newStepListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all built-in steps",
		Long:  "List all built-in steps, optionally filtered by platform.",
		RunE:  runStepList,
	}
	cmd.Flags().String("platform", "", "filter by platform: darwin, ubuntu, or any")
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}

// newStepInfoCmd creates the "step info" subcommand.
func newStepInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show details for a built-in step",
		Long:  "Show full details for a built-in step including its provides, requires, platform, and commands.",
		Args:  cobra.ExactArgs(1),
		RunE:  runStepInfo,
	}
	cmd.Flags().Bool("json", false, "output as JSON")
	return cmd
}

// newStepAddCmd creates the "step add" subcommand.
func newStepAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Scaffold a custom step in the config",
		Long:  "Scaffold a custom step definition in the config's custom_steps section.",
		Args:  cobra.ExactArgs(1),
		RunE:  runStepAdd,
	}
	cmd.Flags().String("config", "", "config file to modify (default: auto-detect)")
	return cmd
}

// runStepList implements the step list command.
func runStepList(cmd *cobra.Command, args []string) error {
	registry := steps.NewRegistry()
	out := cmd.OutOrStdout()

	platform, _ := cmd.Flags().GetString("platform")
	jsonFlag, _ := cmd.Flags().GetBool("json")

	var defs []steps.StepDefinition
	if platform != "" {
		defs = registry.ForPlatform(platform)
	} else {
		defs = registry.All()
	}

	if jsonFlag {
		return stepListJSON(cmd, defs)
	}

	// Group by category
	fmt.Fprintf(out, "Built-in steps (%d available):\n\n", len(defs))

	categories := groupByCategory(defs)
	categoryNames := []string{"core", "packages", "languages", "shell", "system", "generic"}

	for _, cat := range categoryNames {
		catDefs := categories[cat]
		if len(catDefs) == 0 {
			continue
		}

		fmt.Fprintf(out, "%s:\n", capitalize(cat))
		for _, def := range catDefs {
			platforms := formatPlatforms(def.Platforms)
			fmt.Fprintf(out, "  %-25s %-50s %s\n", def.Name, def.Description, platforms)
		}
		fmt.Fprintln(out)
	}

	return nil
}

// stepListJSON outputs the step list as JSON.
func stepListJSON(cmd *cobra.Command, defs []steps.StepDefinition) error {
	type jsonStep struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Category      string   `json:"category"`
		Type          string   `json:"type"`
		Platforms     []string `json:"platforms"`
		Provides      []string `json:"provides"`
		Requires      []string `json:"requires"`
		ConfigSection string   `json:"config_section,omitempty"`
	}

	var items []jsonStep
	for _, def := range defs {
		items = append(items, jsonStep{
			Name:          def.Name,
			Description:   def.Description,
			Category:      def.Category,
			Type:          def.Type,
			Platforms:     def.Platforms,
			Provides:      def.Provides,
			Requires:      def.Requires,
			ConfigSection: def.ConfigSection,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

// runStepInfo implements the step info command.
func runStepInfo(cmd *cobra.Command, args []string) error {
	registry := steps.NewRegistry()
	out := cmd.OutOrStdout()
	name := args[0]

	def, ok := registry.Get(name)
	if !ok {
		return fmt.Errorf("step %q not found; run 'adze step list' to see available steps", name)
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return stepInfoJSON(cmd, def)
	}

	fmt.Fprintf(out, "Step:          %s\n", def.Name)
	fmt.Fprintf(out, "Description:   %s\n", def.Description)
	fmt.Fprintf(out, "Type:          %s\n", def.Type)
	fmt.Fprintf(out, "Platforms:     %s\n", strings.Join(def.Platforms, ", "))

	if len(def.Provides) > 0 {
		fmt.Fprintf(out, "Provides:      %s\n", strings.Join(def.Provides, ", "))
	} else {
		fmt.Fprintf(out, "Provides:      (none)\n")
	}

	if len(def.Requires) > 0 {
		fmt.Fprintf(out, "Requires:      %s\n", strings.Join(def.Requires, ", "))
	} else if len(def.PlatformRequires) > 0 {
		var parts []string
		for plat, reqs := range def.PlatformRequires {
			parts = append(parts, fmt.Sprintf("%s (%s)", strings.Join(reqs, ", "), plat))
		}
		sort.Strings(parts)
		fmt.Fprintf(out, "Requires:      %s\n", strings.Join(parts, "; "))
	} else {
		fmt.Fprintf(out, "Requires:      (none)\n")
	}

	if def.ConfigSection != "" {
		fmt.Fprintf(out, "Config section: %s\n", def.ConfigSection)
	} else {
		fmt.Fprintf(out, "Config section: (none)\n")
	}

	return nil
}

// stepInfoJSON outputs step info as JSON.
func stepInfoJSON(cmd *cobra.Command, def steps.StepDefinition) error {
	type jsonInfo struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Type          string   `json:"type"`
		Category      string   `json:"category"`
		Platforms     []string `json:"platforms"`
		Provides      []string `json:"provides"`
		Requires      []string `json:"requires"`
		ConfigSection string   `json:"config_section,omitempty"`
	}

	info := jsonInfo{
		Name:          def.Name,
		Description:   def.Description,
		Type:          def.Type,
		Category:      def.Category,
		Platforms:     def.Platforms,
		Provides:      def.Provides,
		Requires:      def.Requires,
		ConfigSection: def.ConfigSection,
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(info)
}

// runStepAdd implements the step add command.
func runStepAdd(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	name := args[0]

	// Validate step name format
	if !isValidStepName(name) {
		return fmt.Errorf("invalid step name %q; must match pattern [a-z][a-z0-9-]* (e.g., my-custom-step)", name)
	}

	// Check if it conflicts with a built-in step
	registry := steps.NewRegistry()
	if _, exists := registry.Get(name); exists {
		return fmt.Errorf("step %q conflicts with a built-in step; choose a different name", name)
	}

	// Generate scaffold YAML
	scaffold := generateStepScaffold(name)

	// Try to find and update config file
	configPath, _ := cmd.Flags().GetString("config")
	if configPath == "" {
		// Try global --config flag
		gf := ResolveGlobalFlags(cmd)
		configPath = gf.Config
	}

	if configPath != "" {
		err := appendStepToConfig(configPath, name, scaffold)
		if err != nil {
			return fmt.Errorf("updating config: %w", err)
		}
		fmt.Fprintf(out, "Added custom step %q to %s\n", name, configPath)
		fmt.Fprintln(out, "Edit the step's check and apply commands to match your needs.")
		return nil
	}

	// No config file — just print the scaffold
	fmt.Fprintf(out, "# Add the following to your config's custom_steps section:\n\n")
	fmt.Fprintf(out, "custom_steps:\n")
	fmt.Fprint(out, scaffold)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "# Tip: specify --config <file> to add this directly to your config.")

	return nil
}

// isValidStepName checks if a step name matches the required pattern.
func isValidStepName(name string) bool {
	if len(name) == 0 {
		return false
	}
	if name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, c := range name[1:] {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

// generateStepScaffold generates YAML for a custom step scaffold.
func generateStepScaffold(name string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %s:\n", name))
	b.WriteString(fmt.Sprintf("    description: \"TODO: describe %s\"\n", name))
	b.WriteString("    provides:\n")
	b.WriteString(fmt.Sprintf("      - %s\n", name))
	b.WriteString("    requires: []\n")
	b.WriteString("    platform:\n")
	b.WriteString("      - any\n")
	b.WriteString("    check: \"false  # TODO: command that exits 0 when step is satisfied\"\n")
	b.WriteString("    apply:\n")
	b.WriteString("      any: \"echo TODO: implement apply for " + name + "\"\n")
	return b.String()
}

// appendStepToConfig appends a custom step scaffold to an existing config file.
func appendStepToConfig(path, name, scaffold string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	content := string(data)

	// Check if the step name already exists
	if strings.Contains(content, fmt.Sprintf("  %s:", name)) {
		return fmt.Errorf("step %q already exists in config", name)
	}

	// Check if custom_steps section exists
	if strings.Contains(content, "custom_steps:") {
		// Append to existing custom_steps section
		content = content + scaffold
	} else {
		// Add new custom_steps section
		content = content + "\ncustom_steps:\n" + scaffold
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// groupByCategory groups step definitions by their category.
func groupByCategory(defs []steps.StepDefinition) map[string][]steps.StepDefinition {
	groups := make(map[string][]steps.StepDefinition)
	for _, def := range defs {
		groups[def.Category] = append(groups[def.Category], def)
	}
	return groups
}

// formatPlatforms formats a platform list for display.
func formatPlatforms(platforms []string) string {
	if len(platforms) == 0 {
		return "[any]"
	}
	return "[" + strings.Join(platforms, ", ") + "]"
}

// capitalize returns the string with its first letter capitalized.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

