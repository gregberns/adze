package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/gregberns/adze/internal/step"
)

// --- macos-defaults ---

// MacOSDefaultsStep writes macOS defaults preferences.
type MacOSDefaultsStep struct {
	run CommandRunner
}

func NewMacOSDefaultsStep() step.Step {
	return &MacOSDefaultsStep{run: defaultRunner}
}

func (s *MacOSDefaultsStep) Name() string { return "macos-defaults" }

func (s *MacOSDefaultsStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// For macos-defaults, items encode domain.key pairs.
	// Item.Name is "domain key", Item.Version is the expected value.
	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		parts := strings.SplitN(item.Name, " ", 2)
		if len(parts) < 2 {
			return "false"
		}
		return fmt.Sprintf("defaults read %s %s", parts[0], parts[1])
	})
}

func (s *MacOSDefaultsStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	result, err := batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			parts := strings.SplitN(item.Name, " ", 2)
			if len(parts) < 2 {
				return "false"
			}
			return fmt.Sprintf("defaults read %s %s", parts[0], parts[1])
		},
		func(item step.StepItem) string {
			parts := strings.SplitN(item.Name, " ", 2)
			if len(parts) < 2 {
				return "false"
			}
			// Item.Version holds the "-type value" portion, e.g., "-bool true"
			return fmt.Sprintf("defaults write %s %s %s", parts[0], parts[1], item.Version)
		},
	)
	if err != nil {
		return result, err
	}

	// After writing defaults, restart only the processes whose domains were changed.
	// This avoids unnecessary restarts (e.g., don't restart Dock when only Finder prefs changed).
	restartNeeded := make(map[string]bool)
	for _, ir := range result.ItemResults {
		if ir.Status == step.StatusApplied {
			domain := strings.SplitN(ir.Item.Name, " ", 2)[0]
			if proc, ok := domainProcessMap[domain]; ok {
				restartNeeded[proc] = true
			}
		}
	}
	for proc := range restartNeeded {
		cmd := shellCmd(fmt.Sprintf("killall %s 2>/dev/null || true", proc))
		_, _ = s.run(ctx, cmd, cfg.Env, s.Name(), "restart")
	}

	return result, nil
}

// domainProcessMap maps macOS preference domains to the processes that must be
// restarted after a defaults write. Domains not listed here require no restart.
var domainProcessMap = map[string]string{
	"com.apple.dock":           "Dock",
	"com.apple.finder":         "Finder",
	"com.apple.SystemUIServer": "SystemUIServer",
}

// --- dock-layout ---

// DockLayoutStep configures the macOS Dock layout using dockutil.
type DockLayoutStep struct {
	run CommandRunner
}

func NewDockLayoutStep() step.Step {
	return &DockLayoutStep{run: defaultRunner}
}

func (s *DockLayoutStep) Name() string { return "dock-layout" }

func (s *DockLayoutStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// Runtime prerequisite: dockutil must be available.
	dockutilCheck, err := runShellCheck(ctx, s.run, "command -v dockutil", cfg.Env, s.Name())
	if err != nil {
		return step.StepResult{}, err
	}
	if dockutilCheck.Status != step.StatusSatisfied {
		return step.StepResult{
			Status: step.StatusFailed,
			Reason: "dockutil is required for dock configuration. Add 'dockutil' to packages.brew in your config.",
		}, nil
	}

	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		return fmt.Sprintf(`dockutil --find "%s"`, item.Name)
	})
}

func (s *DockLayoutStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// Runtime prerequisite: dockutil must be available.
	dockutilCheck, err := runShellCheck(ctx, s.run, "command -v dockutil", cfg.Env, s.Name())
	if err != nil {
		return step.StepResult{}, err
	}
	if dockutilCheck.Status != step.StatusSatisfied {
		return step.StepResult{
			Status: step.StatusFailed,
			Reason: "dockutil is required for dock configuration. Add 'dockutil' to packages.brew in your config.",
		}, nil
	}

	return batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			return fmt.Sprintf(`dockutil --find "%s"`, item.Name)
		},
		func(item step.StepItem) string {
			return fmt.Sprintf(`dockutil --add "%s"`, item.Name)
		},
	)
}

// --- machine-name ---

// MachineNameStep sets the macOS machine name (ComputerName, LocalHostName, HostName).
type MachineNameStep struct {
	run CommandRunner
}

func NewMachineNameStep() step.Step {
	return &MachineNameStep{run: defaultRunner}
}

func (s *MachineNameStep) Name() string { return "machine-name" }

func (s *MachineNameStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// The check command compares scutil --get ComputerName against the configured value.
	// The configured hostname is passed via cfg.Check.
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

func (s *MachineNameStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
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
