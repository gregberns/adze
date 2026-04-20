package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/runner"
	"github.com/gregberns/adze/internal/secrets"
	"github.com/gregberns/adze/internal/step"
	"github.com/gregberns/adze/internal/steps"
	"github.com/gregberns/adze/internal/ui"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the configuration to the machine",
		Long:  "Execute the plan. Pre-flight validation runs first, then each step is applied in dependency order.",
		RunE:  runApply,
	}
	cmd.Flags().Bool("yes", false, "non-interactive mode (skip prompts, assume yes)")
	return cmd
}

// applyEvent is a single NDJSON event during apply --json.
type applyEvent struct {
	Type     string `json:"type"`
	Step     string `json:"step,omitempty"`
	Index    int    `json:"index,omitempty"`
	Status   string `json:"status,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Duration string `json:"duration,omitempty"`

	// Summary fields (only on type=summary).
	Total     int `json:"total,omitempty"`
	Applied   int `json:"applied,omitempty"`
	Satisfied int `json:"satisfied,omitempty"`
	Failed    int `json:"failed,omitempty"`
	Skipped   int `json:"skipped,omitempty"`
	ExitCode  int `json:"exit_code,omitempty"`
}

// runApply implements the apply command logic.
func runApply(cmd *cobra.Command, args []string) error {
	gf := ResolveGlobalFlags(cmd)
	w := cmd.OutOrStdout()
	colorOn := ColorEnabled(gf.NoColor)
	jsonMode := DetectOutputMode(gf.JSON) == OutputJSON
	interactive := IsInteractive()

	yesFlag, _ := cmd.Flags().GetBool("yes")

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

	// 6. Validate secrets (interactive prompting unless --yes)
	sm := secrets.NewSecretManager(cfg.Secrets)
	ctx := context.Background()
	secretInteractive := interactive && !yesFlag
	sm.Validate(ctx, secretInteractive)

	// Wire the masking filter into the output writer so sensitive secret
	// values are replaced with *** in all progress and summary output.
	mask := sm.GetMask()
	w = mask.WrapWriter(w)

	// Note: we do NOT abort here for missing/invalid required secrets.
	// Per spec, only steps that reference the missing secret should be
	// skipped; other steps proceed normally. The runner's EnvChecker
	// callback (via makeEnvChecker) handles per-step skipping.

	// 7. If any steps require sudo, acquire privileges
	needsSudo := false
	for _, sc := range stepConfigs {
		if stepNeedsSudo(sc, platform) {
			needsSudo = true
			break
		}
	}

	if needsSudo {
		sudoMgr := adapter.NewSudoManager()
		if err := sudoMgr.AcquirePrivileges([]string{"apply"}); err != nil {
			return &exitError{Code: ExitPreFlightFail, Err: fmt.Errorf("sudo: %w", err)}
		}
		defer sudoMgr.Release()
	}

	// 8. Build step implementations and create Runner
	stepImpls := buildStepImplsForGraph(reg, graph)
	r := runner.NewRunner(stepImpls, graph, sm, platform)

	// Wire step configs so the runner has Check/Apply commands, Items, timeouts, etc.
	configMap := make(map[string]step.StepConfig, len(stepConfigs))
	for _, sc := range stepConfigs {
		configMap[sc.Name] = sc
	}
	r.SetStepConfigs(configMap)

	// 9. Execute with progress display
	if jsonMode {
		return applyWithJSON(ctx, w, r, graph)
	}
	return applyWithProgress(ctx, w, r, graph, colorOn, interactive)
}

// applyWithProgress runs the apply and displays progress using the UI.
func applyWithProgress(ctx context.Context, w io.Writer, r *runner.Runner, graph *dag.ResolvedGraph, colorOn bool, tty bool) error {
	progress := ui.NewProgress(w, len(graph.Steps), colorOn, tty)

	// Wire Runner callbacks to drive live progress display.
	r.OnStepStart = func(stepName string, index int, total int) {
		progress.StartStep(stepName)
	}
	r.OnStepComplete = func(stepName string, index int, total int, result step.StepResult) {
		status := mapStepStatus(result.Status)
		progress.CompleteStep(stepName, status, result.Reason, result.Duration)
	}

	result := r.Run(ctx)

	progress.Finish()

	// Print summary
	fmt.Fprintln(w)
	fmt.Fprint(w, runner.FormatSummary(result))

	return mapRunResultToExit(result)
}

// applyWithJSON runs the apply and outputs NDJSON events.
func applyWithJSON(ctx context.Context, w io.Writer, r *runner.Runner, graph *dag.ResolvedGraph) error {
	enc := json.NewEncoder(w)

	// Wire Runner callbacks to emit NDJSON events in real-time.
	r.OnStepStart = func(stepName string, index int, total int) {
		enc.Encode(applyEvent{
			Type:  "step_start",
			Step:  stepName,
			Index: index,
			Total: total,
		})
	}

	var applied, satisfied, failed, skipped int
	r.OnStepComplete = func(stepName string, index int, total int, result step.StepResult) {
		evt := applyEvent{
			Type:   "step_complete",
			Step:   stepName,
			Status: string(result.Status),
		}
		// Reason is shown only for skipped or failed steps.
		if result.Reason != "" {
			switch result.Status {
			case step.StatusSkipped, step.StatusFailed, step.StatusVerifyFailed, step.StatusPartial:
				evt.Reason = result.Reason
			}
		}
		if result.Duration > 0 {
			evt.Duration = result.Duration.String()
		}
		enc.Encode(evt)

		// Track counts for the summary event.
		switch result.Status {
		case step.StatusApplied:
			applied++
		case step.StatusSatisfied:
			satisfied++
		case step.StatusFailed, step.StatusVerifyFailed, step.StatusPartial:
			failed++
		case step.StatusSkipped:
			skipped++
		}
	}

	result := r.Run(ctx)

	// Emit summary event after all steps complete.
	enc.Encode(applyEvent{
		Type:      "summary",
		Total:     len(result.StepResults),
		Applied:   applied,
		Satisfied: satisfied,
		Failed:    failed,
		Skipped:   skipped,
		ExitCode:  result.ExitCode,
	})

	return mapRunResultToExit(result)
}

// mapStepStatus maps a step.StepStatus to a Progress status string.
func mapStepStatus(s step.StepStatus) string {
	switch s {
	case step.StatusSatisfied:
		return "success"
	case step.StatusApplied:
		return "success"
	case step.StatusFailed, step.StatusVerifyFailed:
		return "failure"
	case step.StatusPartial:
		return "warning"
	case step.StatusSkipped:
		return "skip"
	default:
		return "success"
	}
}

// mapRunResultToExit converts a RunResult exit code to an exitError.
func mapRunResultToExit(result *runner.RunResult) error {
	switch result.ExitCode {
	case 0:
		return nil
	case 4:
		return &exitError{Code: ExitExecFailure, Err: fmt.Errorf("all operations failed")}
	case 5:
		return &exitError{Code: ExitPartialSuccess, Err: fmt.Errorf("some operations failed")}
	default:
		return &exitError{Code: ExitUnexpected, Err: fmt.Errorf("unexpected exit code %d", result.ExitCode)}
	}
}

// buildStepImplsForGraph builds step.Step implementations for all steps in the graph.
// It tries the registry first, then falls back to ShellStep for custom steps.
func buildStepImplsForGraph(reg *steps.Registry, graph *dag.ResolvedGraph) []step.Step {
	var impls []step.Step
	for _, rs := range graph.Steps {
		def, ok := reg.Get(rs.Name)
		if ok && def.Constructor != nil {
			impls = append(impls, def.Constructor())
		} else {
			// Custom step: use ShellStep
			impls = append(impls, step.NewShellStep(rs.Name))
		}
	}
	return impls
}

// stepNeedsSudo heuristically checks if a step might need sudo.
// Checks if the apply command contains "sudo" or if the step is an apt operation.
func stepNeedsSudo(sc step.StepConfig, platform string) bool {
	if platform == "ubuntu" || platform == "debian" {
		// APT operations typically need sudo
		if strings.Contains(sc.Name, "apt") {
			return true
		}
	}

	// Check if any apply command contains sudo
	if sc.Apply != nil {
		for _, arg := range sc.Apply.Args {
			if strings.Contains(arg, "sudo") {
				return true
			}
		}
	}
	if sc.PlatformApply != nil {
		if cmd, ok := sc.PlatformApply[platform]; ok {
			for _, arg := range cmd.Args {
				if strings.Contains(arg, "sudo") {
					return true
				}
			}
		}
	}

	// macOS hostname changes need sudo
	if sc.Name == "machine-name" {
		return true
	}

	return false
}

// Ensure exitError is used in the main cmd handler for exit code propagation.
// This is a compile-time assertion that *exitError satisfies the error interface.
var _ error = (*exitError)(nil)

// ExitCodeFromError extracts the exit code from an error returned by plan or apply.
// If the error is not an *exitError, returns ExitUnexpected.
func ExitCodeFromError(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if ee, ok := err.(*exitError); ok {
		return ee.Code
	}
	return ExitUnexpected
}

// HandleExitError is a helper for main.go to convert RunE errors into proper exit codes.
// It returns the exit code that should be used.
func HandleExitError(err error) int {
	if err == nil {
		return ExitSuccess
	}
	code := ExitCodeFromError(err)
	// Print the error message to stderr
	fmt.Fprintln(os.Stderr, err.Error())
	return code
}
