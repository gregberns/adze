package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	cfgpkg "github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/steps"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newDoctorCmd creates the "doctor" subcommand.
func newDoctorCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Dump full context for AI agent review",
		Long:  "Dump full context about the machine and config for AI agent review. Output is always to stdout.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd, args, version)
		},
	}
	return cmd
}

// runDoctor implements the doctor command.
// The output follows the spec's 6 sections in order:
//  1. Config — full resolved config as YAML
//  2. Dependency Graph — resolved step execution order with provides/requires
//  3. Validation Results — all errors and warnings
//  4. Platform Information — OS, arch, shell, tool version
//  5. Available Steps Not In Use — built-in steps not referenced by config
//  6. Review Questions — fixed set of review prompts
func runDoctor(cmd *cobra.Command, args []string, version string) error {
	out := cmd.OutOrStdout()
	gf := ResolveGlobalFlags(cmd)
	registry := steps.NewRegistry()

	// Load config (needed for multiple sections).
	configPath, configErr := ResolveConfig(gf.Config)
	var cfg *cfgpkg.Config
	var validationErrs []cfgpkg.ValidationError
	var validationWarns []cfgpkg.ValidationWarning
	var parseErr error
	var readErr error

	if configErr == nil {
		var data []byte
		data, readErr = os.ReadFile(configPath)
		if readErr == nil {
			cfg, validationErrs, validationWarns, parseErr = cfgpkg.Parse(data)
		}
	}

	platform := currentPlatform()
	if cfg != nil && cfg.Platform != "" && cfg.Platform != "any" {
		platform = cfg.Platform
	}

	// Section 1: Config
	fmt.Fprintln(out, "=== Config ===")
	if configErr != nil {
		fmt.Fprintf(out, "Config: not found (%s)\n", configErr)
	} else if readErr != nil {
		fmt.Fprintf(out, "Config: not found (%s)\n", readErr)
	} else if parseErr != nil {
		fmt.Fprintf(out, "Config: YAML syntax error (%s)\n", parseErr)
	} else if cfg != nil {
		yamlBytes, marshalErr := yaml.Marshal(cfg)
		if marshalErr != nil {
			fmt.Fprintf(out, "Config: marshal error (%s)\n", marshalErr)
		} else {
			fmt.Fprintf(out, "%s", string(yamlBytes))
		}
	}
	fmt.Fprintln(out)

	// Section 2: Dependency Graph
	fmt.Fprintln(out, "=== Dependency Graph ===")
	if cfg != nil {
		stepInputs := buildStepInputs(registry, cfg, platform)
		resolved, dagErrs := dag.Resolve(stepInputs, platform, buildKnownBuiltIn(registry))
		if len(dagErrs) > 0 {
			fmt.Fprintf(out, "Errors (%d):\n", len(dagErrs))
			for _, e := range dagErrs {
				fmt.Fprintf(out, "  - %s\n", e)
			}
		} else {
			for i, s := range resolved.Steps {
				provides := "—"
				if len(s.Config.Provides) > 0 {
					provides = strings.Join(s.Config.Provides, ", ")
				}
				requires := "—"
				if len(s.Config.Requires) > 0 {
					requires = strings.Join(s.Config.Requires, ", ")
				}
				fmt.Fprintf(out, "%d. %s [provides: %s] [requires: %s]\n",
					i+1, s.Name, provides, requires)
			}
		}
	} else {
		fmt.Fprintln(out, "No config loaded.")
	}
	fmt.Fprintln(out)

	// Section 3: Validation Results
	fmt.Fprintln(out, "=== Validation Results ===")
	if len(validationErrs) == 0 && len(validationWarns) == 0 {
		fmt.Fprintln(out, "No errors or warnings.")
	} else {
		for _, e := range validationErrs {
			fmt.Fprintf(out, "ERROR [%s] %s\n", string(e.Code), e.Message)
		}
		for _, w := range validationWarns {
			fmt.Fprintf(out, "WARN  [%s] %s\n", string(w.Code), w.Message)
		}
	}
	fmt.Fprintln(out)

	// Section 4: Platform Information
	fmt.Fprintln(out, "=== Platform Information ===")
	fmt.Fprintf(out, "OS: %s (%s)\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(out, "Shell: %s\n", os.Getenv("SHELL"))
	fmt.Fprintf(out, "Tool version: %s\n", version)
	fmt.Fprintln(out)

	// Section 5: Available Steps Not In Use
	fmt.Fprintln(out, "=== Available Steps Not In Use ===")
	if cfg != nil {
		unused := findUnusedSteps(registry, cfg, platform)
		if len(unused) == 0 {
			fmt.Fprintln(out, "None.")
		} else {
			for _, entry := range unused {
				fmt.Fprintf(out, "  %s\n", entry)
			}
		}
	} else {
		fmt.Fprintln(out, "No config loaded.")
	}
	fmt.Fprintln(out)

	// Section 6: Review Questions
	fmt.Fprintln(out, "=== Review Questions ===")
	questions := fixedReviewQuestions()
	for i, q := range questions {
		fmt.Fprintf(out, "%d. %s\n", i+1, q)
	}

	return nil
}

// buildStepInputs converts registry steps and config custom_steps into DAG inputs.
func buildStepInputs(registry *steps.Registry, cfg *cfgpkg.Config, platform string) []dag.StepInput {
	var inputs []dag.StepInput

	// Add built-in steps that match the platform
	for _, def := range registry.ForPlatform(platform) {
		inputs = append(inputs, dag.StepInput{
			Name:      def.Name,
			Provides:  def.Provides,
			Requires:  def.RequiresForPlatform(platform),
			Platforms: def.Platforms,
			BuiltIn:   true,
		})
	}

	// Add custom steps from config
	if cfg != nil {
		for name, cs := range cfg.CustomSteps {
			inputs = append(inputs, dag.StepInput{
				Name:      name,
				Provides:  cs.Provides,
				Requires:  cs.Requires,
				Platforms: cs.Platform,
				BuiltIn:   false,
			})
		}
	}

	return inputs
}

// buildKnownBuiltIn creates a KnownBuiltIn map from the registry.
func buildKnownBuiltIn(registry *steps.Registry) dag.KnownBuiltIn {
	known := dag.KnownBuiltIn{}
	for _, def := range registry.All() {
		for _, cap := range def.Provides {
			known[cap] = def.Name
		}
	}
	return known
}

// findUnusedSteps returns built-in steps available for the current platform that
// are not referenced by the config. Output format matches the spec:
//
//	step-name — Description [platforms]
func findUnusedSteps(registry *steps.Registry, cfg *cfgpkg.Config, platform string) []string {
	// Collect step names referenced in the config (custom steps + any built-in steps
	// that have a matching config section with content).
	referenced := make(map[string]bool)
	for name := range cfg.CustomSteps {
		referenced[name] = true
	}

	// A built-in step is "in use" if it has a config section and that section
	// has content, OR if it has no config section (always runs).
	matching := registry.ForPlatform(platform)
	var unused []string
	for _, def := range matching {
		if def.ConfigSection == "" {
			// Steps without a config section always run — they are "in use".
			continue
		}
		if hasConfigContent(cfg, def.ConfigSection) {
			continue
		}
		platStr := strings.Join(def.Platforms, ", ")
		unused = append(unused, fmt.Sprintf("%s \u2014 %s [%s]", def.Name, def.Description, platStr))
	}

	return unused
}

// hasConfigContent checks whether the given config section has any content.
func hasConfigContent(cfg *cfgpkg.Config, section string) bool {
	switch section {
	case "packages.brew":
		return len(cfg.Packages.Brew) > 0
	case "packages.cask":
		return len(cfg.Packages.Cask) > 0
	case "packages.apt":
		return len(cfg.Packages.Apt) > 0
	case "defaults":
		return len(cfg.Defaults) > 0
	case "dock":
		return len(cfg.Dock.Apps) > 0
	case "shell":
		return cfg.Shell.Default != "" || cfg.Shell.Theme != "" || len(cfg.Shell.Plugins) > 0
	case "shell.plugins":
		return len(cfg.Shell.Plugins) > 0
	case "directories":
		return len(cfg.Directories) > 0
	case "identity":
		return cfg.Identity.GitName != "" || cfg.Identity.GitEmail != ""
	case "machine":
		return cfg.Machine.Hostname != ""
	default:
		return false
	}
}

// platformMatchesAny checks if any platform in the list matches.
func platformMatchesAny(platforms []string, platform string) bool {
	for _, p := range platforms {
		if p == "any" || p == platform {
			return true
		}
	}
	return false
}

// fixedReviewQuestions returns the fixed set of review prompts from the spec.
func fixedReviewQuestions() []string {
	return []string{
		"Are custom step dependencies declared correctly?",
		"Are there built-in steps that should replace custom steps?",
		"For platform-specific steps, suggest cross-platform equivalents.",
		"Review defaults settings for current OS version compatibility.",
		"Identify common tools/configs that might be missing.",
		"Check for deprecated packages or formulas.",
	}
}
