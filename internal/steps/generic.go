package steps

import (
	"context"
	"fmt"

	"github.com/gregberns/adze/internal/step"
)

// --- directories ---

// DirectoriesStep creates directories from config.
type DirectoriesStep struct {
	run CommandRunner
}

func NewDirectoriesStep() step.Step {
	return &DirectoriesStep{run: defaultRunner}
}

func (s *DirectoriesStep) Name() string { return "directories" }

func (s *DirectoriesStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		return fmt.Sprintf(`[ -d "%s" ]`, expandHome(item.Name))
	})
}

func (s *DirectoriesStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			return fmt.Sprintf(`[ -d "%s" ]`, expandHome(item.Name))
		},
		func(item step.StepItem) string {
			return fmt.Sprintf(`mkdir -p "%s"`, expandHome(item.Name))
		},
	)
}

// --- git-config ---

// GitConfigStep sets global git configuration from identity settings.
type GitConfigStep struct {
	run CommandRunner
}

func NewGitConfigStep() step.Step {
	return &GitConfigStep{run: defaultRunner}
}

func (s *GitConfigStep) Name() string { return "git-config" }

func (s *GitConfigStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// The check command is built by bindings to verify user.name and user.email match config.
	if cfg.Check != nil {
		result, err := s.run(ctx, cfg.Check, cfg.Env, s.Name(), "check")
		if err != nil {
			return step.StepResult{}, err
		}
		if result.ExitCode == 0 {
			return step.StepResult{Status: step.StatusSatisfied, Duration: result.Duration}, nil
		}
		return step.StepResult{
			Status:   step.StatusFailed,
			Reason:   fmt.Sprintf("check exited with code %d", result.ExitCode),
			Duration: result.Duration,
		}, nil
	}
	return step.StepResult{
		Status: step.StatusFailed,
		Reason: "no check command configured",
	}, nil
}

func (s *GitConfigStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	if cfg.Apply != nil {
		result, err := s.run(ctx, cfg.Apply, cfg.Env, s.Name(), "apply")
		if err != nil {
			return step.StepResult{}, err
		}
		if result.ExitCode == 0 {
			return step.StepResult{Status: step.StatusApplied, Duration: result.Duration}, nil
		}
		return step.StepResult{
			Status:   step.StatusFailed,
			Reason:   fmt.Sprintf("apply exited with code %d", result.ExitCode),
			Duration: result.Duration,
		}, nil
	}
	return step.StepResult{
		Status: step.StatusSkipped,
		Reason: "no apply command configured",
	}, nil
}

// --- ssh-keys ---

// SSHKeysStep generates SSH keys.
type SSHKeysStep struct {
	run CommandRunner
}

func NewSSHKeysStep() step.Step {
	return &SSHKeysStep{run: defaultRunner}
}

func (s *SSHKeysStep) Name() string { return "ssh-keys" }

func (s *SSHKeysStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, `[ -f "$HOME/.ssh/id_ed25519" ]`, cfg.Env, s.Name())
}

func (s *SSHKeysStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// If a custom apply command is set (e.g., with specific email), use it.
	if cfg.Apply != nil {
		result, err := s.run(ctx, cfg.Apply, cfg.Env, s.Name(), "apply")
		if err != nil {
			return step.StepResult{}, err
		}
		if result.ExitCode == 0 {
			return step.StepResult{Status: step.StatusApplied, Duration: result.Duration}, nil
		}
		return step.StepResult{
			Status:   step.StatusFailed,
			Reason:   fmt.Sprintf("apply exited with code %d", result.ExitCode),
			Duration: result.Duration,
		}, nil
	}

	// Default: generate ed25519 key with empty passphrase.
	return runShellApply(ctx, s.run,
		`ssh-keygen -t ed25519 -f "$HOME/.ssh/id_ed25519" -N ""`,
		cfg.Env, s.Name())
}
