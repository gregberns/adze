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
)

// newDoctorCmd creates the "doctor" subcommand.
func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Dump full context for AI agent review",
		Long:  "Dump full context about the machine and config for AI agent review. Output is always to stdout.",
		RunE:  runDoctor,
	}
	return cmd
}

// runDoctor implements the doctor command.
func runDoctor(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	gf := ResolveGlobalFlags(cmd)
	registry := steps.NewRegistry()

	// Section 1: Platform info
	fmt.Fprintln(out, "=== Platform ===")
	fmt.Fprintf(out, "OS:   %s\n", runtime.GOOS)
	fmt.Fprintf(out, "Arch: %s\n", runtime.GOARCH)
	fmt.Fprintln(out)

	// Section 2: Config summary
	fmt.Fprintln(out, "=== Config ===")
	configPath, configErr := ResolveConfig(gf.Config)
	var cfg *cfgpkg.Config
	var validationErrs []cfgpkg.ValidationError
	var validationWarns []cfgpkg.ValidationWarning

	if configErr != nil {
		fmt.Fprintf(out, "Config: not found (%s)\n", configErr)
	} else {
		fmt.Fprintf(out, "File:   %s\n", configPath)
		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			fmt.Fprintf(out, "Read:   error (%s)\n", readErr)
		} else {
			var parseErr error
			cfg, validationErrs, validationWarns, parseErr = cfgpkg.Parse(data)
			if parseErr != nil {
				fmt.Fprintf(out, "Parse:  YAML syntax error (%s)\n", parseErr)
			} else {
				fmt.Fprintf(out, "Parse:  OK\n")
				fmt.Fprintf(out, "Errors: %d\n", len(validationErrs))
				fmt.Fprintf(out, "Warns:  %d\n", len(validationWarns))
			}
		}
	}
	fmt.Fprintln(out)

	// Section 3: Dependency graph
	fmt.Fprintln(out, "=== Dependency Graph ===")
	if cfg != nil {
		platform := cfg.Platform
		if platform == "any" {
			platform = currentPlatform()
		}

		stepInputs := buildStepInputs(registry, cfg, platform)
		resolved, dagErrs := dag.Resolve(stepInputs, platform, buildKnownBuiltIn(registry))
		if len(dagErrs) > 0 {
			fmt.Fprintf(out, "Status: errors (%d)\n", len(dagErrs))
			for _, e := range dagErrs {
				fmt.Fprintf(out, "  - %s\n", e)
			}
		} else {
			fmt.Fprintf(out, "Steps:  %d\n", len(resolved.Steps))
			fmt.Fprintf(out, "Cycles: none\n")
		}
	} else {
		fmt.Fprintln(out, "Status: no config loaded")
	}
	fmt.Fprintln(out)

	// Section 4: Validation results
	fmt.Fprintln(out, "=== Validation ===")
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

	// Section 5: Step inventory
	fmt.Fprintln(out, "=== Step Inventory ===")
	allSteps := registry.All()
	fmt.Fprintf(out, "Built-in steps: %d\n", len(allSteps))
	platform := currentPlatform()
	if cfg != nil && cfg.Platform != "" && cfg.Platform != "any" {
		platform = cfg.Platform
	}
	matching := registry.ForPlatform(platform)
	fmt.Fprintf(out, "Platform match (%s): %d\n", platform, len(matching))
	for _, s := range matching {
		fmt.Fprintf(out, "  %-25s %s\n", s.Name, s.Description)
	}
	fmt.Fprintln(out)

	// Section 6: Unused steps
	fmt.Fprintln(out, "=== Unused Steps ===")
	if cfg != nil {
		unused := findUnusedSteps(registry, cfg, platform)
		if len(unused) == 0 {
			fmt.Fprintln(out, "None.")
		} else {
			for _, name := range unused {
				fmt.Fprintf(out, "  - %s\n", name)
			}
		}
	} else {
		fmt.Fprintln(out, "No config loaded.")
	}
	fmt.Fprintln(out)

	// Section 7: Pre-flight status
	fmt.Fprintln(out, "=== Pre-flight ===")
	if cfg != nil && len(cfg.Secrets) > 0 {
		for _, s := range cfg.Secrets {
			val := os.Getenv(s.Name)
			status := "SET"
			if val == "" {
				if s.Required {
					status = "MISSING (required)"
				} else {
					status = "MISSING (optional)"
				}
			}
			fmt.Fprintf(out, "  %-30s %s\n", s.Name, status)
		}
	} else {
		fmt.Fprintln(out, "No secrets declared.")
	}
	fmt.Fprintln(out)

	// Section 8: Review questions
	fmt.Fprintln(out, "=== Review Questions ===")
	questions := generateReviewQuestions(cfg, validationErrs, configErr)
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
			Requires:  def.Requires,
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

// findUnusedSteps returns step names that are in the config but not applicable
// to the current platform.
func findUnusedSteps(registry *steps.Registry, cfg *cfgpkg.Config, platform string) []string {
	var unused []string

	// Check custom steps whose platform doesn't match
	for name, cs := range cfg.CustomSteps {
		matches := false
		for _, p := range cs.Platform {
			if p == "any" || p == platform {
				matches = true
				break
			}
		}
		if !matches {
			unused = append(unused, name+" (custom)")
		}
	}

	// Check built-in steps that exist but don't match the platform
	allSteps := registry.All()
	for _, def := range allSteps {
		if !platformMatchesAny(def.Platforms, platform) {
			// Only report if the step has a config section that might be referenced
			if def.ConfigSection != "" {
				unused = append(unused, def.Name+" (built-in, "+strings.Join(def.Platforms, "/")+"-only)")
			}
		}
	}

	return unused
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

// generateReviewQuestions produces suggested review areas based on the diagnostic results.
func generateReviewQuestions(cfg *cfgpkg.Config, errs []cfgpkg.ValidationError, configErr error) []string {
	var questions []string

	if configErr != nil {
		questions = append(questions, "No config file found. Run 'adze init' to create one.")
		return questions
	}

	if len(errs) > 0 {
		questions = append(questions, fmt.Sprintf("There are %d validation errors. Fix these before running apply.", len(errs)))
	}

	if cfg != nil {
		if cfg.Identity.GitName == "" || cfg.Identity.GitEmail == "" {
			questions = append(questions, "Git identity is incomplete. Consider adding git_name and git_email to the identity section.")
		}
		if len(cfg.Packages.Brew) == 0 && len(cfg.Packages.Cask) == 0 && len(cfg.Packages.Apt) == 0 {
			questions = append(questions, "No packages declared. Run 'adze init' to detect installed packages.")
		}
		if len(cfg.Secrets) > 0 {
			questions = append(questions, "Review secret declarations and ensure required environment variables are available.")
		}
		if len(cfg.CustomSteps) > 0 {
			questions = append(questions, "Custom steps defined. Verify their check and apply commands are correct.")
		}
	}

	if len(questions) == 0 {
		questions = append(questions, "Config looks good. Run 'adze plan' to preview changes.")
	}

	return questions
}
