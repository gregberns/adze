package steps

import (
	"context"
	"fmt"

	"github.com/gregberns/adze/internal/step"
)

// --- oh-my-zsh ---

// OhMyZshStep installs Oh My Zsh.
type OhMyZshStep struct {
	run CommandRunner
}

func NewOhMyZshStep() step.Step {
	return &OhMyZshStep{run: defaultRunner}
}

func (s *OhMyZshStep) Name() string { return "oh-my-zsh" }

func (s *OhMyZshStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellCheck(ctx, s.run, `[ -d "$HOME/.oh-my-zsh" ]`, cfg.Env, s.Name())
}

func (s *OhMyZshStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return runShellApply(ctx, s.run,
		`sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended`,
		cfg.Env, s.Name())
}

// --- zsh-plugins ---

// ZshPluginsStep installs Zsh plugins into Oh My Zsh custom directory.
type ZshPluginsStep struct {
	run CommandRunner
}

func NewZshPluginsStep() step.Step {
	return &ZshPluginsStep{run: defaultRunner}
}

func (s *ZshPluginsStep) Name() string { return "zsh-plugins" }

// wellKnownPluginRepos maps well-known Zsh plugin names to their Git repo URLs.
var wellKnownPluginRepos = map[string]string{
	"zsh-syntax-highlighting":  "https://github.com/zsh-users/zsh-syntax-highlighting.git",
	"zsh-autosuggestions":      "https://github.com/zsh-users/zsh-autosuggestions.git",
	"zsh-completions":          "https://github.com/zsh-users/zsh-completions.git",
	"zsh-history-substring-search": "https://github.com/zsh-users/zsh-history-substring-search.git",
	"fast-syntax-highlighting": "https://github.com/zdharma-continuum/fast-syntax-highlighting.git",
	"you-should-use":           "https://github.com/MichaelAqworking/zsh-you-should-use.git",
}

// pluginRepoURL returns the Git clone URL for a plugin, or empty string if unknown.
func pluginRepoURL(name string) string {
	if url, ok := wellKnownPluginRepos[name]; ok {
		return url
	}
	// Try common pattern: github.com/zsh-users/<name>
	return ""
}

func (s *ZshPluginsStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchCheck(ctx, s.run, cfg, s.Name(), func(item step.StepItem) string {
		return fmt.Sprintf(`[ -d "$HOME/.oh-my-zsh/custom/plugins/%s" ]`, item.Name)
	})
}

func (s *ZshPluginsStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return batchApply(ctx, s.run, cfg, s.Name(),
		func(item step.StepItem) string {
			return fmt.Sprintf(`[ -d "$HOME/.oh-my-zsh/custom/plugins/%s" ]`, item.Name)
		},
		func(item step.StepItem) string {
			repoURL := pluginRepoURL(item.Name)
			if repoURL == "" {
				// Unknown plugin — attempt a reasonable default URL
				repoURL = fmt.Sprintf("https://github.com/zsh-users/%s.git", item.Name)
			}
			return fmt.Sprintf(`git clone %s "$HOME/.oh-my-zsh/custom/plugins/%s"`, repoURL, item.Name)
		},
	)
}

// --- shell-default ---

// ShellDefaultStep sets the user's default shell.
type ShellDefaultStep struct {
	run CommandRunner
}

func NewShellDefaultStep() step.Step {
	return &ShellDefaultStep{run: defaultRunner}
}

func (s *ShellDefaultStep) Name() string { return "shell-default" }

// shellPaths maps shell names to their typical binary paths.
var shellPaths = map[string]string{
	"zsh":  "/bin/zsh",
	"bash": "/bin/bash",
	"fish": "/usr/local/bin/fish",
}

func (s *ShellDefaultStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	// The configured shell is passed via the step config's Check command.
	if cfg.Check != nil {
		result, err := s.run(ctx, cfg.Check, cfg.Env, s.Name(), "check")
		if err != nil {
			return step.StepResult{}, err
		}
		if result.ExitCode == 0 {
			return step.StepResult{Status: step.StatusSatisfied, Duration: result.Duration}, nil
		}
		return step.StepResult{
			Status: step.StatusFailed,
			Reason: fmt.Sprintf("check exited with code %d", result.ExitCode),
			Duration: result.Duration,
		}, nil
	}
	return step.StepResult{
		Status: step.StatusFailed,
		Reason: "no check command configured",
	}, nil
}

func (s *ShellDefaultStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
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
			Reason: fmt.Sprintf("apply exited with code %d", result.ExitCode),
			Duration: result.Duration,
		}, nil
	}
	return step.StepResult{
		Status: step.StatusSkipped,
		Reason: "no apply command configured",
	}, nil
}
