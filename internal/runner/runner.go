// Package runner orchestrates execution of the full step graph.
//
// The runner iterates steps in DAG-resolved order, executing each via the step
// package's ExecuteStep or ExecuteBatchStep. It tracks capability tainting for
// skip propagation and produces a RunResult with per-step outcomes and an exit code.
//
// Recovery is stateless: every step's check runs on every invocation. Already-satisfied
// steps complete instantly, so re-running after a failure auto-resumes.
package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/secrets"
	"github.com/gregberns/adze/internal/step"
)

// Runner orchestrates execution of a resolved step graph.
type Runner struct {
	steps    []step.Step
	graph    *dag.ResolvedGraph
	secrets  *secrets.SecretManager
	platform string
	logDir   string // ~/.config/adze/logs/

	// stepConfigs provides optional per-step StepConfig overrides.
	// When set, the config for a step name is used instead of buildStepConfig.
	// This allows callers (and tests) to supply full StepConfigs with Items,
	// Check/Apply commands, timeouts, etc.
	stepConfigs map[string]step.StepConfig

	// OnStepStart is called immediately before each step's execution begins.
	// index is the 1-based position in execution order; total is the count of steps.
	// If nil, the callback is not invoked.
	OnStepStart func(stepName string, index int, total int)

	// OnStepComplete is called immediately after each step's full lifecycle completes.
	// index is the 1-based position in execution order; total is the count of steps.
	// result carries the status, skip reason, duration, and per-item results.
	// If nil, the callback is not invoked.
	OnStepComplete func(stepName string, index int, total int, result step.StepResult)
}

// RunResult holds the outcome of a full graph execution.
type RunResult struct {
	StepResults []StepRunResult
	ExitCode    int
}

// StepRunResult holds the outcome of a single step's execution.
type StepRunResult struct {
	Name     string
	Status   step.StepStatus
	Reason   string
	Duration time.Duration
	Items    []step.ItemResult // nil for atomic
	LogPath  string            // non-empty only for failures
}

// NewRunner creates a Runner for the given steps, resolved graph, secret manager, and platform.
func NewRunner(steps []step.Step, graph *dag.ResolvedGraph, secrets *secrets.SecretManager, platform string) *Runner {
	logDir := filepath.Join(os.Getenv("HOME"), ".config", "adze", "logs")
	return &Runner{
		steps:    steps,
		graph:    graph,
		secrets:  secrets,
		platform: platform,
		logDir:   logDir,
	}
}

// SetStepConfigs provides full StepConfig overrides keyed by step name.
// When set, the runner uses these configs (with Check/Apply commands, Items,
// timeouts, etc.) instead of deriving minimal configs from the DAG.
func (r *Runner) SetStepConfigs(configs map[string]step.StepConfig) {
	r.stepConfigs = configs
}

// Run executes all steps in DAG order and returns the aggregate result.
func (r *Runner) Run(ctx context.Context) *RunResult {
	// Build step lookup by name.
	stepByName := make(map[string]step.Step, len(r.steps))
	for _, s := range r.steps {
		stepByName[s.Name()] = s
	}

	// Track tainted capabilities: capability → name of the step that failed.
	tainted := make(map[string]string)

	var results []StepRunResult

	envChecker := makeEnvChecker(r.secrets)

	total := len(r.graph.Steps)

	for i, rs := range r.graph.Steps {
		index := i + 1 // 1-based per spec

		// Invoke OnStepStart callback before any work on this step.
		if r.OnStepStart != nil {
			r.OnStepStart(rs.Name, index, total)
		}

		s, ok := stepByName[rs.Name]
		if !ok {
			// Step not found in the provided step list — record as failed.
			stepResult := step.StepResult{
				Status: step.StatusFailed,
				Reason: fmt.Sprintf("step %q not found in step list", rs.Name),
			}
			results = append(results, StepRunResult{
				Name:   rs.Name,
				Status: stepResult.Status,
				Reason: stepResult.Reason,
			})
			// Taint all provides.
			for _, cap := range rs.Config.Provides {
				tainted[cap] = rs.Name
			}
			if r.OnStepComplete != nil {
				r.OnStepComplete(rs.Name, index, total, stepResult)
			}
			continue
		}

		// Check skip propagation: if any required capability is tainted, skip.
		skipReason := ""
		for _, req := range rs.Config.Requires {
			if failedStep, isTainted := tainted[req]; isTainted {
				skipReason = fmt.Sprintf("skipped because %s failed", failedStep)
				break
			}
		}

		if skipReason != "" {
			stepResult := step.StepResult{
				Status: step.StatusSkipped,
				Reason: skipReason,
			}
			results = append(results, StepRunResult{
				Name:   rs.Name,
				Status: stepResult.Status,
				Reason: stepResult.Reason,
			})
			// Propagate taint transitively: this skipped step's provides are also tainted.
			for _, cap := range rs.Config.Provides {
				tainted[cap] = rs.Name
			}
			if r.OnStepComplete != nil {
				r.OnStepComplete(rs.Name, index, total, stepResult)
			}
			continue
		}

		// Build the StepConfig: use override if available, else derive from DAG.
		var cfg step.StepConfig
		if override, ok := r.stepConfigs[rs.Name]; ok {
			cfg = override
		} else {
			cfg = buildStepConfig(rs)
		}

		// Execute the step.
		var result step.StepResult
		var err error

		if cfg.Items != nil {
			result, err = step.ExecuteBatchStep(ctx, s, cfg, r.platform, envChecker)
		} else {
			result, err = step.ExecuteStep(ctx, s, cfg, r.platform, envChecker)
		}

		if err != nil {
			// Infrastructure error — treat as failed.
			infraResult := step.StepResult{
				Status: step.StatusFailed,
				Reason: fmt.Sprintf("infrastructure error: %v", err),
			}
			results = append(results, StepRunResult{
				Name:   rs.Name,
				Status: infraResult.Status,
				Reason: infraResult.Reason,
			})
			for _, cap := range rs.Config.Provides {
				tainted[cap] = rs.Name
			}
			if r.OnStepComplete != nil {
				r.OnStepComplete(rs.Name, index, total, infraResult)
			}
			continue
		}

		sr := StepRunResult{
			Name:     rs.Name,
			Status:   result.Status,
			Reason:   result.Reason,
			Duration: result.Duration,
			Items:    result.ItemResults,
		}

		// Write log files for failures and record the log path.
		switch result.Status {
		case step.StatusFailed, step.StatusVerifyFailed:
			logPath := r.writeLogFile(rs.Name, "", result.Reason)
			sr.LogPath = logPath
		case step.StatusPartial:
			// For partial batch steps, write logs for each failed item.
			for _, ir := range result.ItemResults {
				if ir.Status == step.StatusFailed {
					logPath := r.writeLogFile(rs.Name, ir.Item.Name, ir.Reason)
					sr.LogPath = logPath // last failed item's path; summary will show per-item
				}
			}
		}

		results = append(results, sr)

		// Taint capabilities for failed/skipped/verify_failed (but NOT partial).
		switch result.Status {
		case step.StatusFailed, step.StatusSkipped, step.StatusVerifyFailed:
			for _, cap := range rs.Config.Provides {
				tainted[cap] = rs.Name
			}
		}

		// Invoke OnStepComplete callback after the step's full lifecycle.
		if r.OnStepComplete != nil {
			r.OnStepComplete(rs.Name, index, total, result)
		}
	}

	exitCode := computeExitCode(results)

	return &RunResult{
		StepResults: results,
		ExitCode:    exitCode,
	}
}

// buildStepConfig converts a dag.ResolvedStep into a step.StepConfig.
// This is a bridge function since the runner receives dag.StepInput data
// but ExecuteStep/ExecuteBatchStep expect step.StepConfig.
//
// Note: In a full implementation, the config layer would produce StepConfig
// objects directly. Here we construct a minimal config from what the DAG provides,
// which is sufficient for the runner's own testing with mock steps.
func buildStepConfig(rs dag.ResolvedStep) step.StepConfig {
	return step.StepConfig{
		Name:     rs.Name,
		Provides: rs.Config.Provides,
		Requires: rs.Config.Requires,
	}
}

// makeEnvChecker creates an EnvChecker from the secret manager.
func makeEnvChecker(sm *secrets.SecretManager) step.EnvChecker {
	if sm == nil {
		return nil
	}
	return func(name string) (string, bool, bool) {
		value := os.Getenv(name)
		required := sm.IsRequired(name)
		present := sm.IsAvailable(name)
		return value, required, present
	}
}

// computeExitCode determines the exit code from the step results.
//
// Exit codes (execution only — pre-execution codes handled by CLI):
//   - 0: all steps succeeded or already satisfied
//   - 4: all attempted operations failed; no progress
//   - 5: some applied, some failed (partial progress)
func computeExitCode(results []StepRunResult) int {
	hasApplied := false
	hasFailed := false

	for _, r := range results {
		switch r.Status {
		case step.StatusApplied:
			hasApplied = true
		case step.StatusFailed, step.StatusVerifyFailed:
			hasFailed = true
		case step.StatusPartial:
			// Partial means some items applied, some failed.
			hasApplied = true
			hasFailed = true
		case step.StatusSatisfied, step.StatusSkipped:
			// No effect on exit code.
		}
	}

	switch {
	case !hasFailed:
		return 0
	case hasFailed && hasApplied:
		return 5
	case hasFailed && !hasApplied:
		return 4
	default:
		return 0
	}
}

// writeLogFile writes a log file for a failed step or item.
// Returns the log file path, or empty string on error.
func (r *Runner) writeLogFile(stepName, itemName, content string) string {
	if err := os.MkdirAll(r.logDir, 0o755); err != nil {
		return ""
	}

	var filename string
	if itemName != "" {
		filename = fmt.Sprintf("%s-%s.log", stepName, itemName)
	} else {
		filename = fmt.Sprintf("%s.log", stepName)
	}

	logPath := filepath.Join(r.logDir, filename)

	// Overwrite (not append) each run.
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		return ""
	}

	return logPath
}
