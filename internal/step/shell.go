package step

import (
	"context"
	"fmt"
)

// ShellStep adapts a YAML custom step definition to the Step interface.
// It executes shell commands directly via os/exec (no shell interpolation).
// ShellStep MUST NOT be used with batch semantics (Items must be nil or empty).
type ShellStep struct {
	name string
}

// NewShellStep creates a new ShellStep with the given name.
func NewShellStep(name string) *ShellStep {
	return &ShellStep{name: name}
}

// Name returns the step name.
func (s *ShellStep) Name() string {
	return s.name
}

// Check runs the step's check command to determine if desired state is satisfied.
// If cfg.Check is nil, the step is always considered unsatisfied (returns StatusFailed).
// This is NOT an error — the result is expressed via StepResult.Status.
func (s *ShellStep) Check(ctx context.Context, cfg StepConfig) (StepResult, error) {
	if cfg.Items != nil && len(cfg.Items) > 0 {
		return StepResult{}, fmt.Errorf("ShellStep does not support batch semantics (Items must be nil or empty)")
	}

	if cfg.Check == nil {
		return StepResult{
			Status: StatusFailed,
			Reason: "no check command defined",
		}, nil
	}

	result, err := RunCommand(ctx, cfg.Check, cfg.Env, s.name, "check")
	if err != nil {
		return StepResult{}, err
	}

	if result.ExitCode == 0 {
		return StepResult{
			Status:   StatusSatisfied,
			Duration: result.Duration,
		}, nil
	}

	return StepResult{
		Status:   StatusFailed,
		Reason:   fmt.Sprintf("check exited with code %d", result.ExitCode),
		Duration: result.Duration,
	}, nil
}

// Apply runs the step's apply command with platform dispatch.
// Resolution order:
//  1. PlatformApply[detected_platform] if present
//  2. Apply if non-nil
//  3. StatusSkipped with "no apply command for platform <P>"
//
// The platform parameter is passed via the context or executor; here we use
// cfg.PlatformApply and cfg.Apply directly. The executor resolves platform.
func (s *ShellStep) Apply(ctx context.Context, cfg StepConfig) (StepResult, error) {
	if cfg.Items != nil && len(cfg.Items) > 0 {
		return StepResult{}, fmt.Errorf("ShellStep does not support batch semantics (Items must be nil or empty)")
	}

	// The apply command is already resolved by the executor (platform dispatch).
	// ShellStep.Apply uses cfg.Apply which the executor sets after resolution.
	if cfg.Apply == nil {
		return StepResult{
			Status: StatusSkipped,
			Reason: "no apply command",
		}, nil
	}

	result, err := RunCommand(ctx, cfg.Apply, cfg.Env, s.name, "apply")
	if err != nil {
		return StepResult{}, err
	}

	if result.ExitCode == 0 {
		return StepResult{
			Status:   StatusApplied,
			Duration: result.Duration,
		}, nil
	}

	return StepResult{
		Status:   StatusFailed,
		Reason:   fmt.Sprintf("apply exited with code %d", result.ExitCode),
		Duration: result.Duration,
	}, nil
}
