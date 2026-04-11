package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/gregberns/adze/internal/step"
)

// --- gsettings ---

// GSettingsStep writes GNOME/gsettings preferences on Ubuntu.
type GSettingsStep struct {
	run CommandRunner
}

func NewGSettingsStep() step.Step {
	return &GSettingsStep{run: defaultRunner}
}

func (s *GSettingsStep) Name() string { return "gsettings" }

func (s *GSettingsStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// For gsettings, items encode "schema key" pairs.
	// Item.Name is "schema key", Item.Version is the expected value.
	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		parts := strings.SplitN(item.Name, " ", 2)
		if len(parts) < 2 {
			return "false"
		}
		return fmt.Sprintf("gsettings get %s %s", parts[0], parts[1])
	})
}

func (s *GSettingsStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			parts := strings.SplitN(item.Name, " ", 2)
			if len(parts) < 2 {
				return "false"
			}
			return fmt.Sprintf("gsettings get %s %s", parts[0], parts[1])
		},
		func(item step.StepItem) string {
			parts := strings.SplitN(item.Name, " ", 2)
			if len(parts) < 2 {
				return "false"
			}
			return fmt.Sprintf("gsettings set %s %s %s", parts[0], parts[1], item.Version)
		},
	)
}
