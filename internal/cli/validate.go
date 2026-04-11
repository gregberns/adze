package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/steps"
	"github.com/spf13/cobra"
)

// validateResult holds all validation outcomes for JSON output.
type validateResult struct {
	Valid    bool              `json:"valid"`
	Errors   []validateError  `json:"errors,omitempty"`
	Warnings []validateWarn   `json:"warnings,omitempty"`
}

type validateError struct {
	Code    string `json:"code,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type validateWarn struct {
	Code    string `json:"code,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func newValidateCmdImpl() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the config without execution",
		Long:  "Deep validation of the config: YAML syntax, schema compliance, include resolution, dependency graph, secret declarations, and platform compatibility.",
		RunE:  runValidate,
	}
	cmd.Flags().Bool("json", false, "output validation results as JSON")
	return cmd
}

func runValidate(cmd *cobra.Command, args []string) error {
	gf := ResolveGlobalFlags(cmd)

	configPath, err := ResolveConfig(gf.Config)
	if err != nil {
		return err
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")
	if gf.JSON {
		jsonFlag = true
	}

	// Phase 1: Load config (YAML syntax + schema + includes)
	cfg, valErrs, valWarns, loadErr := config.LoadConfig(configPath, false)
	if loadErr != nil {
		// YAML syntax error or include resolution failure
		if jsonFlag {
			result := validateResult{
				Valid:  false,
				Errors: []validateError{{Message: loadErr.Error()}},
			}
			if err := outputJSON(cmd, result); err != nil {
				return err
			}
			return &exitError{Code: ExitConfigError, Err: loadErr}
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", loadErr)
		return &exitError{Code: ExitConfigError, Err: loadErr}
	}

	var allErrors []validateError
	var allWarnings []validateWarn

	// Collect config validation errors
	for _, ve := range valErrs {
		allErrors = append(allErrors, validateError{
			Code:    string(ve.Code),
			Field:   ve.Field,
			Message: ve.Message,
		})
	}

	// Collect config validation warnings
	for _, vw := range valWarns {
		allWarnings = append(allWarnings, validateWarn{
			Code:    string(vw.Code),
			Field:   vw.Field,
			Message: vw.Message,
		})
	}

	// If config parse failed (cfg is nil), stop here
	if cfg == nil {
		if jsonFlag {
			result := validateResult{Valid: false, Errors: allErrors, Warnings: allWarnings}
			return outputJSON(cmd, result)
		}
		printValidateHuman(cmd, allErrors, allWarnings)
		return &exitError{Code: ExitConfigError, Err: fmt.Errorf("config validation failed")}
	}

	// Phase 2: DAG resolution
	platform := resolvePlatform(cfg.Platform)
	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, platform, reg)

	// Build DAG inputs
	var dagInputs []dag.StepInput
	for _, sc := range stepConfigs {
		dagInputs = append(dagInputs, dag.StepInput{
			Name:      sc.Name,
			Provides:  sc.Provides,
			Requires:  sc.Requires,
			Platforms: sc.Platforms,
			BuiltIn:   sc.PlatformApply != nil || !isCustomStep(sc.Name, cfg),
		})
	}

	knownBuiltIns := buildKnownBuiltIns(reg)
	_, dagErrs := dag.Resolve(dagInputs, platform, knownBuiltIns)
	for _, de := range dagErrs {
		allErrors = append(allErrors, validateError{Message: de.Error()})
	}

	// Phase 3: Secrets cross-reference
	declaredSecrets := make(map[string]bool)
	for _, s := range cfg.Secrets {
		declaredSecrets[s.Name] = true
	}

	// Check custom step env vars against declared secrets
	for name, cs := range cfg.CustomSteps {
		for _, envVar := range cs.Env {
			if !declaredSecrets[envVar] {
				allWarnings = append(allWarnings, validateWarn{
					Message: fmt.Sprintf("step %q references env var %q not declared in secrets", name, envVar),
				})
			}
		}
	}

	// Phase 4: Platform compatibility check
	if cfg.Platform != "any" && cfg.Platform != runtime.GOOS {
		switch runtime.GOOS {
		case "darwin":
			if cfg.Platform != "darwin" {
				allWarnings = append(allWarnings, validateWarn{
					Message: fmt.Sprintf("config platform %q does not match current OS %q", cfg.Platform, runtime.GOOS),
				})
			}
		case "linux":
			if cfg.Platform != "ubuntu" && cfg.Platform != "debian" {
				allWarnings = append(allWarnings, validateWarn{
					Message: fmt.Sprintf("config platform %q does not match current OS %q", cfg.Platform, runtime.GOOS),
				})
			}
		}
	}

	// Determine exit code
	hasConfigErrors := len(valErrs) > 0
	hasGraphErrors := len(dagErrs) > 0

	if jsonFlag {
		result := validateResult{
			Valid:    len(allErrors) == 0,
			Errors:   allErrors,
			Warnings: allWarnings,
		}
		if err := outputJSON(cmd, result); err != nil {
			return err
		}
		if hasConfigErrors {
			return &exitError{Code: ExitConfigError, Err: fmt.Errorf("config validation failed")}
		}
		if hasGraphErrors {
			return &exitError{Code: ExitPreFlightFail, Err: fmt.Errorf("pre-flight validation failed")}
		}
		return nil
	}

	// Human output
	printValidateHuman(cmd, allErrors, allWarnings)

	if hasConfigErrors {
		return &exitError{Code: ExitConfigError, Err: fmt.Errorf("config validation failed")}
	}
	if hasGraphErrors {
		return &exitError{Code: ExitPreFlightFail, Err: fmt.Errorf("pre-flight validation failed")}
	}

	if len(allErrors) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Config is valid.")
	}
	return nil
}

func printValidateHuman(cmd *cobra.Command, errs []validateError, warns []validateWarn) {
	out := cmd.OutOrStderr()
	for _, e := range errs {
		if e.Code != "" {
			fmt.Fprintf(out, "[%s] %s\n", e.Code, e.Message)
		} else {
			fmt.Fprintf(out, "Error: %s\n", e.Message)
		}
	}
	for _, w := range warns {
		if w.Code != "" {
			fmt.Fprintf(out, "Warning [%s]: %s\n", w.Code, w.Message)
		} else {
			fmt.Fprintf(out, "Warning: %s\n", w.Message)
		}
	}
}

func outputJSON(cmd *cobra.Command, v interface{}) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// exitCodeFromErr returns the exit code from an exitError, or 1 for other errors.
func exitCodeFromErr(err error) int {
	if ee, ok := err.(*exitError); ok {
		return ee.Code
	}
	return ExitUnexpected
}

func resolvePlatform(cfgPlatform string) string {
	if cfgPlatform == "any" {
		switch runtime.GOOS {
		case "darwin":
			return "darwin"
		case "linux":
			return "linux"
		default:
			return runtime.GOOS
		}
	}
	return cfgPlatform
}

func isCustomStep(name string, cfg *config.Config) bool {
	_, ok := cfg.CustomSteps[name]
	return ok
}

func buildKnownBuiltIns(reg *steps.Registry) dag.KnownBuiltIn {
	known := dag.KnownBuiltIn{}
	for _, def := range reg.All() {
		for _, cap := range def.Provides {
			known[cap] = def.Name
		}
	}
	return known
}
