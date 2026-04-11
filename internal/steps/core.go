package steps

import (
	"context"

	"github.com/gregberns/adze/internal/step"
)

// --- xcode-cli-tools ---

// XcodeCLIToolsStep installs Xcode Command Line Tools on macOS.
type XcodeCLIToolsStep struct {
	run CommandRunner
}

func NewXcodeCLIToolsStep() step.Step {
	return &XcodeCLIToolsStep{run: defaultRunner}
}

func (s *XcodeCLIToolsStep) Name() string { return "xcode-cli-tools" }

func (s *XcodeCLIToolsStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, "xcode-select -p", cfg.Env, s.Name())
}

func (s *XcodeCLIToolsStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellApply(ctx, s.run, "xcode-select --install", cfg.Env, s.Name())
}

// --- homebrew ---

// HomebrewStep installs the Homebrew package manager on macOS.
type HomebrewStep struct {
	run CommandRunner
}

func NewHomebrewStep() step.Step {
	return &HomebrewStep{run: defaultRunner}
}

func (s *HomebrewStep) Name() string { return "homebrew" }

func (s *HomebrewStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, "command -v brew", cfg.Env, s.Name())
}

func (s *HomebrewStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellApply(ctx, s.run,
		`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`,
		cfg.Env, s.Name())
}

// --- apt-essentials ---

// AptEssentialsStep installs build-essential and common dependencies on Ubuntu/Debian.
type AptEssentialsStep struct {
	run CommandRunner
}

func NewAptEssentialsStep() step.Step {
	return &AptEssentialsStep{run: defaultRunner}
}

func (s *AptEssentialsStep) Name() string { return "apt-essentials" }

func (s *AptEssentialsStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, "dpkg-query -W build-essential", cfg.Env, s.Name())
}

func (s *AptEssentialsStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellApply(ctx, s.run,
		"sudo apt-get update && sudo apt-get install -y build-essential curl wget git",
		cfg.Env, s.Name())
}
