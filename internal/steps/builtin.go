package steps

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gregberns/adze/internal/step"
)

// CommandRunner is a function type for executing shell commands.
// It allows dependency injection for testing. The default implementation
// uses step.RunCommand.
type CommandRunner func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error)

// defaultRunner uses the real step.RunCommand.
func defaultRunner(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
	return step.RunCommand(ctx, cmd, env, stepName, phase)
}

// shellCmd creates a ShellCommand that runs the given command through sh -c.
// Use this for commands that require shell features (pipes, globbing, $HOME, etc.).
func shellCmd(command string) *step.ShellCommand {
	return &step.ShellCommand{
		Args: []string{"sh", "-c", command},
	}
}

// directCmd creates a ShellCommand that runs a binary directly with arguments.
func directCmd(args ...string) *step.ShellCommand {
	return &step.ShellCommand{
		Args: args,
	}
}

// runShellCheck runs a shell command as a check phase. Exit 0 = satisfied.
func runShellCheck(ctx context.Context, run CommandRunner, command string, env []string, stepName string) (step.StepResult, error) {
	cmd := shellCmd(command)
	result, err := run(ctx, cmd, env, stepName, "check")
	if err != nil {
		return step.StepResult{}, err
	}
	if result.ExitCode == 0 {
		return step.StepResult{
			Status:   step.StatusSatisfied,
			Duration: result.Duration,
		}, nil
	}
	return step.StepResult{
		Status:   step.StatusFailed,
		Reason:   fmt.Sprintf("check exited with code %d", result.ExitCode),
		Duration: result.Duration,
	}, nil
}

// runShellApply runs a shell command as an apply phase. Exit 0 = applied.
func runShellApply(ctx context.Context, run CommandRunner, command string, env []string, stepName string) (step.StepResult, error) {
	cmd := shellCmd(command)
	result, err := run(ctx, cmd, env, stepName, "apply")
	if err != nil {
		return step.StepResult{}, err
	}
	if result.ExitCode == 0 {
		return step.StepResult{
			Status:   step.StatusApplied,
			Duration: result.Duration,
		}, nil
	}
	return step.StepResult{
		Status:   step.StatusFailed,
		Reason:   fmt.Sprintf("apply exited with code %d", result.ExitCode),
		Duration: result.Duration,
	}, nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}

// itemEnvVars returns environment variables for a batch item.
func itemEnvVars(item step.StepItem) []string {
	return []string{
		fmt.Sprintf("ADZE_ITEM_NAME=%s", item.Name),
		fmt.Sprintf("ADZE_ITEM_VERSION=%s", item.Version),
	}
}

// batchCheck runs a per-item check for batch steps, returning aggregate results.
func batchCheck(ctx context.Context, run CommandRunner, cfg step.StepConfig, stepName string, checkFn func(item step.StepItem) string) (step.StepResult, error) {
	if len(cfg.Items) == 0 {
		return step.StepResult{Status: step.StatusSatisfied}, nil
	}

	var results []step.ItemResult
	for _, item := range cfg.Items {
		command := checkFn(item)
		env := append(append([]string{}, cfg.Env...), itemEnvVars(item)...)
		checkResult, err := runShellCheck(ctx, run, command, env, fmt.Sprintf("%s[%s]", stepName, item.Name))
		if err != nil {
			return step.StepResult{}, err
		}
		results = append(results, step.ItemResult{
			Item:   item,
			Status: checkResult.Status,
			Reason: checkResult.Reason,
		})
	}

	return step.StepResult{
		Status:      aggregateBatchResults(results),
		ItemResults: results,
	}, nil
}

// batchApply runs per-item check+apply for batch steps, returning aggregate results.
func batchApply(ctx context.Context, run CommandRunner, cfg step.StepConfig, stepName string,
	checkFn func(item step.StepItem) string, applyFn func(item step.StepItem) string) (step.StepResult, error) {
	if len(cfg.Items) == 0 {
		return step.StepResult{Status: step.StatusSatisfied}, nil
	}

	var results []step.ItemResult
	for _, item := range cfg.Items {
		env := append(append([]string{}, cfg.Env...), itemEnvVars(item)...)
		itemStepName := fmt.Sprintf("%s[%s]", stepName, item.Name)

		// Check first
		checkCmd := checkFn(item)
		checkResult, err := runShellCheck(ctx, run, checkCmd, env, itemStepName)
		if err != nil {
			return step.StepResult{}, err
		}
		if checkResult.Status == step.StatusSatisfied {
			results = append(results, step.ItemResult{
				Item:   item,
				Status: step.StatusSatisfied,
			})
			continue
		}

		// Apply
		applyCmd := applyFn(item)
		applyResult, err := runShellApply(ctx, run, applyCmd, env, itemStepName)
		if err != nil {
			return step.StepResult{}, err
		}
		if applyResult.Status != step.StatusApplied {
			results = append(results, step.ItemResult{
				Item:   item,
				Status: step.StatusFailed,
				Reason: applyResult.Reason,
			})
			continue
		}

		// Verify
		verifyResult, err := runShellCheck(ctx, run, checkCmd, env, itemStepName)
		if err != nil {
			return step.StepResult{}, err
		}
		if verifyResult.Status == step.StatusSatisfied {
			results = append(results, step.ItemResult{
				Item:   item,
				Status: step.StatusApplied,
			})
		} else {
			results = append(results, step.ItemResult{
				Item:   item,
				Status: step.StatusFailed,
				Reason: "verify failed",
			})
		}
	}

	return step.StepResult{
		Status:      aggregateBatchResults(results),
		ItemResults: results,
	}, nil
}

// aggregateBatchResults computes the overall status from item results.
func aggregateBatchResults(results []step.ItemResult) step.StepStatus {
	if len(results) == 0 {
		return step.StatusSatisfied
	}

	var hasApplied, hasFailed, hasSatisfied bool
	for _, r := range results {
		switch r.Status {
		case step.StatusSatisfied:
			hasSatisfied = true
		case step.StatusApplied:
			hasApplied = true
		case step.StatusFailed:
			hasFailed = true
		}
	}

	switch {
	case !hasApplied && !hasFailed && hasSatisfied:
		return step.StatusSatisfied
	case !hasFailed && (hasApplied || hasSatisfied):
		return step.StatusApplied
	case hasFailed && (hasApplied || hasSatisfied):
		return step.StatusPartial
	default:
		return step.StatusFailed
	}
}
