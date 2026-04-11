package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/secrets"
	"github.com/gregberns/adze/internal/step"
	"github.com/gregberns/adze/internal/steps"
	"github.com/gregberns/adze/internal/ui"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what changes would be made",
		Long:  "Resolve the dependency graph, check current state against config, and show what would change. No mutations are performed.",
		RunE:  runPlan,
	}
	return cmd
}

// planResult holds the structured result of a plan operation, used for both
// human and JSON output.
type planResult struct {
	Platform   string            `json:"platform"`
	ConfigFile string            `json:"config"`
	PreFlight  []preFlightCheck  `json:"pre_flight"`
	Steps      []planStepResult  `json:"steps"`
	Summary    planSummary       `json:"summary"`
}

type preFlightCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type planStepResult struct {
	Index       int    `json:"index"`
	Action      string `json:"action"`  // "skip", "install", "blocked"
	Name        string `json:"name"`
	Description string `json:"description"`
}

type planSummary struct {
	ToApply   int `json:"to_apply"`
	Satisfied int `json:"satisfied"`
	Blocked   int `json:"blocked"`
}

// runPlan implements the plan command logic.
func runPlan(cmd *cobra.Command, args []string) error {
	gf := ResolveGlobalFlags(cmd)
	w := cmd.OutOrStdout()
	colorOn := ColorEnabled(gf.NoColor)
	jsonMode := DetectOutputMode(gf.JSON) == OutputJSON

	// 1. Resolve config
	configPath, isURL, cleanup, err := resolveConfigPath(gf.Config)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return &exitError{Code: ExitConfigError, Err: err}
	}

	// 2. Load and validate config
	cfg, valErrs, _, loadErr := config.LoadConfig(configPath, isURL)
	if loadErr != nil {
		return &exitError{Code: ExitConfigError, Err: loadErr}
	}
	if len(valErrs) > 0 {
		var msgs []string
		for _, ve := range valErrs {
			msgs = append(msgs, ve.Error())
		}
		return &exitError{
			Code: ExitConfigError,
			Err:  fmt.Errorf("config validation errors:\n  %s", strings.Join(msgs, "\n  ")),
		}
	}

	// 3. Detect platform
	platform, err := adapter.DetectPlatform()
	if err != nil {
		return &exitError{Code: ExitPreFlightFail, Err: fmt.Errorf("platform detection: %w", err)}
	}

	// Validate platform matches config
	if cfg.Platform != "any" && cfg.Platform != platform {
		return &exitError{
			Code: ExitPreFlightFail,
			Err:  fmt.Errorf("platform mismatch: config requires %q, detected %q", cfg.Platform, platform),
		}
	}

	// 4. Build step configs from config + registry
	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, platform, reg)

	// 5. Resolve DAG
	dagInputs := stepConfigsToDagInputs(stepConfigs)
	graph, dagErrs := dag.Resolve(dagInputs, platform, nil)
	if len(dagErrs) > 0 {
		var msgs []string
		for _, e := range dagErrs {
			msgs = append(msgs, e.Error())
		}
		return &exitError{
			Code: ExitPreFlightFail,
			Err:  fmt.Errorf("dependency graph errors:\n  %s", strings.Join(msgs, "\n  ")),
		}
	}

	// 6. Validate secrets
	sm := secrets.NewSecretManager(cfg.Secrets)
	ctx := context.Background()
	secretResults := sm.Validate(ctx, false) // never interactive in plan

	// Build pre-flight checks
	var preFlights []preFlightCheck
	preFlights = append(preFlights, preFlightCheck{
		Name:    "Platform compatible",
		OK:      true,
		Message: fmt.Sprintf("%s detected", platform),
	})
	preFlights = append(preFlights, preFlightCheck{
		Name:    "Dependency graph valid",
		OK:      true,
		Message: fmt.Sprintf("%d steps, 0 cycles", len(graph.Steps)),
	})

	hasRequiredMissing := false
	for _, sr := range secretResults {
		switch sr.Status {
		case "valid", "prompted":
			preFlights = append(preFlights, preFlightCheck{
				Name:    fmt.Sprintf("Secret %s", sr.Name),
				OK:      true,
				Message: "set",
			})
		case "missing":
			hasRequiredMissing = true
			preFlights = append(preFlights, preFlightCheck{
				Name:    fmt.Sprintf("Secret %s", sr.Name),
				OK:      false,
				Message: "not set (required)",
			})
		case "missing_optional":
			preFlights = append(preFlights, preFlightCheck{
				Name:    fmt.Sprintf("Secret %s", sr.Name),
				OK:      true,
				Message: "not set (optional, skipping)",
			})
		case "invalid":
			hasRequiredMissing = true
			preFlights = append(preFlights, preFlightCheck{
				Name:    fmt.Sprintf("Secret %s", sr.Name),
				OK:      false,
				Message: fmt.Sprintf("set but validation failed: %s", sr.ValidateError),
			})
		}
	}

	// 7. Run check-only for each step to determine current state
	stepConfigByName := make(map[string]step.StepConfig, len(stepConfigs))
	for _, sc := range stepConfigs {
		stepConfigByName[sc.Name] = sc
	}

	// Build step impls from registry
	stepImplByName := buildStepImpls(reg)

	var planSteps []planStepResult
	toApply := 0
	satisfied := 0
	blocked := 0

	for i, rs := range graph.Steps {
		sc, scOK := stepConfigByName[rs.Name]
		impl, implOK := stepImplByName[rs.Name]

		pr := planStepResult{
			Index: i + 1,
			Name:  rs.Name,
		}

		if !scOK || !implOK {
			// Custom step or no impl available -- check if it has a StepConfig
			if scOK && sc.Check != nil {
				checkResult := runCheckOnly(ctx, impl, sc, implOK)
				pr.Action, pr.Description = classifyCheckResult(checkResult, sc)
			} else {
				pr.Action = "install"
				pr.Description = "will be applied"
			}
		} else {
			// Check if this step is blocked by missing required env vars
			blockedBySecret := false
			for _, envName := range sc.Env {
				if sm.IsRequired(envName) && !sm.IsAvailable(envName) {
					blockedBySecret = true
					pr.Action = "blocked"
					pr.Description = fmt.Sprintf("blocked by missing secret %s", envName)
					break
				}
			}

			if !blockedBySecret {
				checkResult := runCheckOnly(ctx, impl, sc, implOK)
				pr.Action, pr.Description = classifyCheckResult(checkResult, sc)
			}
		}

		switch pr.Action {
		case "skip":
			satisfied++
		case "install":
			toApply++
		case "blocked":
			blocked++
		}

		planSteps = append(planSteps, pr)
	}

	result := planResult{
		Platform:   platform,
		ConfigFile: configPath,
		PreFlight:  preFlights,
		Steps:      planSteps,
		Summary: planSummary{
			ToApply:   toApply,
			Satisfied: satisfied,
			Blocked:   blocked,
		},
	}

	// 8. Format and print output
	if jsonMode {
		return outputPlanJSON(w, result)
	}
	outputPlanHuman(w, result, colorOn)

	// 9. Return appropriate exit code
	if hasRequiredMissing {
		return &exitError{Code: ExitPreFlightFail, Err: fmt.Errorf("required secrets are missing")}
	}
	if toApply > 0 {
		return &exitError{Code: ExitChangesPlanned, Err: fmt.Errorf("%d changes would be made", toApply)}
	}
	return nil // exit 0: no changes needed
}

// runCheckOnly runs only the check phase for a step. Returns the StepResult.
func runCheckOnly(ctx context.Context, impl step.Step, sc step.StepConfig, hasImpl bool) step.StepResult {
	if !hasImpl || impl == nil {
		return step.StepResult{Status: step.StatusFailed, Reason: "no step implementation"}
	}

	if sc.Items != nil && len(sc.Items) > 0 {
		// For batch steps, check each item
		checkResult, err := impl.Check(ctx, sc)
		if err != nil {
			return step.StepResult{Status: step.StatusFailed, Reason: err.Error()}
		}
		return checkResult
	}

	checkResult, err := impl.Check(ctx, sc)
	if err != nil {
		return step.StepResult{Status: step.StatusFailed, Reason: err.Error()}
	}
	return checkResult
}

// classifyCheckResult converts a check result into an action and description for plan output.
func classifyCheckResult(result step.StepResult, sc step.StepConfig) (action, description string) {
	switch result.Status {
	case step.StatusSatisfied:
		return "skip", "already satisfied"
	default:
		desc := "will be applied"
		if sc.Apply != nil && len(sc.Apply.Args) > 0 {
			// Show the command that would be run
			if sc.Apply.Args[0] == "sh" && len(sc.Apply.Args) >= 3 && sc.Apply.Args[1] == "-c" {
				desc = sc.Apply.Args[2]
				// Truncate long commands
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
			}
		}
		return "install", desc
	}
}

// outputPlanHuman writes the plan output in human-readable format.
func outputPlanHuman(w io.Writer, result planResult, colorOn bool) {
	fmt.Fprintf(w, "Platform: %s\n", result.Platform)
	fmt.Fprintf(w, "Config: %s\n", result.ConfigFile)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Pre-flight:")
	for _, pf := range result.PreFlight {
		sym := ui.SuccessSymbol(colorOn)
		if !pf.OK {
			sym = ui.FailureSymbol(colorOn)
		}
		fmt.Fprintf(w, "  %s %s", sym, pf.Name)
		if pf.Message != "" {
			fmt.Fprintf(w, " (%s)", pf.Message)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Plan:")
	for _, ps := range result.Steps {
		actionLabel := formatAction(ps.Action, colorOn)
		fmt.Fprintf(w, "  %2d. %s %-20s %s\n", ps.Index, actionLabel, ps.Name, ps.Description)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Summary: %d to apply, %d already satisfied, %d blocked by missing secrets\n",
		result.Summary.ToApply, result.Summary.Satisfied, result.Summary.Blocked)
}

// formatAction formats the action tag for plan output, e.g., "[skip]", "[install]", "[blocked]".
func formatAction(action string, colorOn bool) string {
	tag := fmt.Sprintf("[%s]", action)
	// Pad to fixed width
	padded := fmt.Sprintf("%-10s", tag)
	switch action {
	case "skip":
		return ui.Colorize(padded, ui.ColorDim, colorOn)
	case "install":
		return ui.Colorize(padded, ui.ColorGreen, colorOn)
	case "blocked":
		return ui.Colorize(padded, ui.ColorRed, colorOn)
	default:
		return padded
	}
}

// outputPlanJSON writes the plan output as a single JSON object.
func outputPlanJSON(w io.Writer, result planResult) error {
	enc := json.NewEncoder(w)
	if IsInteractive() {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(result); err != nil {
		return &exitError{Code: ExitUnexpected, Err: fmt.Errorf("JSON encoding: %w", err)}
	}
	return nil
}

// stepConfigsToDagInputs converts step.StepConfig slices to dag.StepInput slices.
func stepConfigsToDagInputs(cfgs []step.StepConfig) []dag.StepInput {
	inputs := make([]dag.StepInput, len(cfgs))
	for i, sc := range cfgs {
		inputs[i] = dag.StepInput{
			Name:      sc.Name,
			Provides:  sc.Provides,
			Requires:  sc.Requires,
			Platforms: sc.Platforms,
			BuiltIn:   true, // all registry steps are built-in; custom steps are not but DAG doesn't differentiate much
		}
	}
	return inputs
}

// buildStepImpls creates step implementations from the registry.
func buildStepImpls(reg *steps.Registry) map[string]step.Step {
	implByName := make(map[string]step.Step)
	for _, def := range reg.All() {
		if def.Constructor != nil {
			implByName[def.Name] = def.Constructor()
		}
	}
	return implByName
}

// resolveConfigPath resolves the config file path, handling URL downloads.
// Returns the local path, whether it was from a URL, a cleanup function (may be nil), and error.
func resolveConfigPath(flagValue string) (path string, isURL bool, cleanup func(), err error) {
	if flagValue != "" && (strings.HasPrefix(flagValue, "http://") || strings.HasPrefix(flagValue, "https://")) {
		// Download to temp file
		tmpFile, dlErr := downloadConfig(flagValue)
		if dlErr != nil {
			return "", true, nil, dlErr
		}
		return tmpFile, true, func() { os.Remove(tmpFile) }, nil
	}

	resolved, resolveErr := ResolveConfig(flagValue)
	if resolveErr != nil {
		return "", false, nil, resolveErr
	}
	return resolved, false, nil, nil
}

// downloadConfig downloads a config file from a URL to a temp file and returns the path.
func downloadConfig(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading config from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading config from %s: HTTP %d", url, resp.StatusCode)
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "adze-config-remote.yaml")
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading config from %s: %w", url, err)
	}

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return "", fmt.Errorf("writing temp config: %w", err)
	}

	return tmpFile, nil
}

// exitError wraps an error with an exit code. cobra's RunE detects this
// to set the process exit code.
type exitError struct {
	Code int
	Err  error
}

func (e *exitError) Error() string {
	return e.Err.Error()
}
