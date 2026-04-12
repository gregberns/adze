package steps

import (
	"context"
	"fmt"

	"github.com/gregberns/adze/internal/step"
)

// --- brew-packages ---

// BrewPackagesStep installs Homebrew formulas from config.
type BrewPackagesStep struct {
	run CommandRunner
}

func NewBrewPackagesStep() step.Step {
	return &BrewPackagesStep{run: defaultRunner}
}

func (s *BrewPackagesStep) Name() string { return "brew-packages" }

func (s *BrewPackagesStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		return fmt.Sprintf("brew list --formula %s", item.Name)
	})
}

func (s *BrewPackagesStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			return fmt.Sprintf("brew list --formula %s", item.Name)
		},
		func(item step.StepItem) string {
			if item.Version != "" {
				return fmt.Sprintf("brew install %s@%s", item.Name, item.Version)
			}
			return fmt.Sprintf("brew install %s", item.Name)
		},
	)
}

// --- brew-casks ---

// BrewCasksStep installs Homebrew casks from config.
type BrewCasksStep struct {
	run CommandRunner
}

func NewBrewCasksStep() step.Step {
	return &BrewCasksStep{run: defaultRunner}
}

func (s *BrewCasksStep) Name() string { return "brew-casks" }

func (s *BrewCasksStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		return fmt.Sprintf("brew list --cask %s", item.Name)
	})
}

func (s *BrewCasksStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			return fmt.Sprintf("brew list --cask %s", item.Name)
		},
		func(item step.StepItem) string {
			return fmt.Sprintf("brew install --cask %s", item.Name)
		},
	)
}

// --- apt-packages ---

// AptPackagesStep installs APT packages from config.
type AptPackagesStep struct {
	run CommandRunner
}

func NewAptPackagesStep() step.Step {
	return &AptPackagesStep{run: defaultRunner}
}

func (s *AptPackagesStep) Name() string { return "apt-packages" }

func (s *AptPackagesStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		return fmt.Sprintf("dpkg-query -W %s", item.Name)
	})
}

func (s *AptPackagesStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			return fmt.Sprintf("dpkg-query -W %s", item.Name)
		},
		func(item step.StepItem) string {
			if item.Version != "" {
				return fmt.Sprintf("sudo apt-get install -y %s=%s", item.Name, item.Version)
			}
			return fmt.Sprintf("sudo apt-get install -y %s", item.Name)
		},
	)
}
