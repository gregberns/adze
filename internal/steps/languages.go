package steps

import (
	"context"

	"github.com/gregberns/adze/internal/step"
)

// --- node-fnm ---

// NodeFnmStep installs fnm (Fast Node Manager) for Node.js management.
type NodeFnmStep struct {
	run CommandRunner
}

func NewNodeFnmStep() step.Step {
	return &NodeFnmStep{run: defaultRunner}
}

func (s *NodeFnmStep) Name() string { return "node-fnm" }

func (s *NodeFnmStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, "command -v fnm", cfg.Env, s.Name())
}

func (s *NodeFnmStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// Platform dispatch is handled via cfg.Apply which is resolved by the executor.
	// If called directly, use the apply command from config.
	if cfg.Apply != nil {
		result, err := s.run(ctx, cfg.Apply, cfg.Env, s.Name(), "apply")
		if err != nil {
			return step.StepResult{}, err
		}
		if result.ExitCode == 0 {
			return step.StepResult{Status: step.StatusApplied, Duration: result.Duration}, nil
		}
		return step.StepResult{
			Status: step.StatusFailed,
			Reason: "apply exited with non-zero code",
		}, nil
	}
	return step.StepResult{
		Status: step.StatusSkipped,
		Reason: "no apply command configured",
	}, nil
}

// --- python ---

// PythonStep installs Python 3.
type PythonStep struct {
	run CommandRunner
}

func NewPythonStep() step.Step {
	return &PythonStep{run: defaultRunner}
}

func (s *PythonStep) Name() string { return "python" }

func (s *PythonStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, "command -v python3", cfg.Env, s.Name())
}

func (s *PythonStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	if cfg.Apply != nil {
		result, err := s.run(ctx, cfg.Apply, cfg.Env, s.Name(), "apply")
		if err != nil {
			return step.StepResult{}, err
		}
		if result.ExitCode == 0 {
			return step.StepResult{Status: step.StatusApplied, Duration: result.Duration}, nil
		}
		return step.StepResult{
			Status: step.StatusFailed,
			Reason: "apply exited with non-zero code",
		}, nil
	}
	return step.StepResult{
		Status: step.StatusSkipped,
		Reason: "no apply command configured",
	}, nil
}

// --- go ---

// GoStep installs the Go programming language.
type GoStep struct {
	run CommandRunner
}

func NewGoStep() step.Step {
	return &GoStep{run: defaultRunner}
}

func (s *GoStep) Name() string { return "go" }

func (s *GoStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, "command -v go", cfg.Env, s.Name())
}

func (s *GoStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	if cfg.Apply != nil {
		result, err := s.run(ctx, cfg.Apply, cfg.Env, s.Name(), "apply")
		if err != nil {
			return step.StepResult{}, err
		}
		if result.ExitCode == 0 {
			return step.StepResult{Status: step.StatusApplied, Duration: result.Duration}, nil
		}
		return step.StepResult{
			Status: step.StatusFailed,
			Reason: "apply exited with non-zero code",
		}, nil
	}
	return step.StepResult{
		Status: step.StatusSkipped,
		Reason: "no apply command configured",
	}, nil
}

// --- rust ---

// RustStep installs Rust via rustup.
type RustStep struct {
	run CommandRunner
}

func NewRustStep() step.Step {
	return &RustStep{run: defaultRunner}
}

func (s *RustStep) Name() string { return "rust" }

func (s *RustStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, "command -v rustc", cfg.Env, s.Name())
}

func (s *RustStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellApply(ctx, s.run,
		`curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y`,
		cfg.Env, s.Name())
}
