package steps

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gregberns/adze/internal/step"
)

// mockRunner creates a CommandRunner that returns configurable results.
func mockRunner(exitCode int) CommandRunner {
	return func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		return step.ExecResult{ExitCode: exitCode}, nil
	}
}

// mockRunnerError creates a CommandRunner that returns an error.
func mockRunnerError(err error) CommandRunner {
	return func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		return step.ExecResult{}, err
	}
}

// mockRunnerSequence creates a CommandRunner that returns different exit codes per call.
func mockRunnerSequence(codes ...int) CommandRunner {
	idx := 0
	return func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		code := 0
		if idx < len(codes) {
			code = codes[idx]
			idx++
		}
		return step.ExecResult{ExitCode: code}, nil
	}
}

func TestXcodeCLIToolsCheck(t *testing.T) {
	s := &XcodeCLIToolsStep{run: mockRunner(0)}
	result, err := s.Check(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}

	s = &XcodeCLIToolsStep{run: mockRunner(1)}
	result, err = s.Check(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
}

func TestHomebrewCheckAndApply(t *testing.T) {
	s := &HomebrewStep{run: mockRunner(0)}
	result, err := s.Check(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}

	result, err = s.Apply(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestBrewPackagesBatchCheck(t *testing.T) {
	s := &BrewPackagesStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Items: []step.StepItem{
			{Name: "git"},
			{Name: "jq"},
		},
	}
	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
	if len(result.ItemResults) != 2 {
		t.Errorf("expected 2 item results, got %d", len(result.ItemResults))
	}
}

func TestBrewPackagesBatchApply(t *testing.T) {
	// First call per item = check (fails), second = apply (succeeds), third = verify (succeeds)
	callCount := 0
	runner := func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		callCount++
		// Pattern for each item: check fails (1), apply succeeds (0), verify succeeds (0)
		switch callCount % 3 {
		case 1:
			return step.ExecResult{ExitCode: 1}, nil // check: not installed
		case 2:
			return step.ExecResult{ExitCode: 0}, nil // apply: success
		case 0:
			return step.ExecResult{ExitCode: 0}, nil // verify: success
		}
		return step.ExecResult{ExitCode: 0}, nil
	}

	s := &BrewPackagesStep{run: runner}
	cfg := step.StepConfig{
		Items: []step.StepItem{
			{Name: "git"},
			{Name: "jq", Version: "1.7"},
		},
	}
	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
	if len(result.ItemResults) != 2 {
		t.Errorf("expected 2 item results, got %d", len(result.ItemResults))
	}
}

func TestBrewPackagesEmptyItems(t *testing.T) {
	s := &BrewPackagesStep{run: mockRunner(0)}
	cfg := step.StepConfig{}
	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied for empty items, got %s", result.Status)
	}
}

func TestDirectoriesBatch(t *testing.T) {
	s := &DirectoriesStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Items: []step.StepItem{
			{Name: "~/projects"},
			{Name: "~/bin"},
		},
	}
	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

func TestDockLayoutRequiresDockutil(t *testing.T) {
	// dockutil not found (exit 1)
	s := &DockLayoutStep{run: mockRunner(1)}
	cfg := step.StepConfig{
		Items: []step.StepItem{{Name: "Safari"}},
	}
	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusFailed {
		t.Errorf("expected failed when dockutil missing, got %s", result.Status)
	}
	if result.Reason == "" {
		t.Error("expected reason message about dockutil")
	}
}

func TestDockLayoutRequiresDockutilApply(t *testing.T) {
	// dockutil not found (exit 1)
	s := &DockLayoutStep{run: mockRunner(1)}
	cfg := step.StepConfig{
		Items: []step.StepItem{{Name: "Safari"}},
	}
	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusFailed {
		t.Errorf("expected failed when dockutil missing, got %s", result.Status)
	}
}

func TestRustCheckAndApply(t *testing.T) {
	s := &RustStep{run: mockRunner(0)}
	result, err := s.Check(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}

	result, err = s.Apply(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestOhMyZshCheck(t *testing.T) {
	s := &OhMyZshStep{run: mockRunner(0)}
	result, err := s.Check(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

func TestSSHKeysDefaultApply(t *testing.T) {
	s := &SSHKeysStep{run: mockRunner(0)}
	result, err := s.Apply(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestSSHKeysCustomApply(t *testing.T) {
	s := &SSHKeysStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Apply: shellCmd(`ssh-keygen -t ed25519 -C "custom@example.com" -f "$HOME/.ssh/id_ed25519" -N ""`),
	}
	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestGitConfigWithCustomCommands(t *testing.T) {
	s := &GitConfigStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Check: shellCmd(`[ "$(git config --global user.name)" = "Test" ]`),
		Apply: shellCmd(`git config --global user.name "Test"`),
	}

	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}

	result, err = s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestGitConfigNoCheckCommand(t *testing.T) {
	s := &GitConfigStep{run: mockRunner(0)}
	result, err := s.Check(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusFailed {
		t.Errorf("expected failed with no check command, got %s", result.Status)
	}
}

func TestShellDefaultStep(t *testing.T) {
	s := &ShellDefaultStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Check: shellCmd(`[ "$SHELL" = "/bin/zsh" ]`),
		Apply: shellCmd("chsh -s /bin/zsh"),
	}

	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

func TestMachineNameStep(t *testing.T) {
	s := &MachineNameStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Check: shellCmd(`[ "$(scutil --get ComputerName)" = "my-mac" ]`),
		Apply: shellCmd(`sudo scutil --set ComputerName "my-mac"`),
	}

	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

func TestNodeFnmApplyWithPlatformCommand(t *testing.T) {
	s := &NodeFnmStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Apply: shellCmd("brew install fnm"),
	}
	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestNodeFnmApplyNoCommand(t *testing.T) {
	s := &NodeFnmStep{run: mockRunner(0)}
	result, err := s.Apply(context.Background(), step.StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSkipped {
		t.Errorf("expected skipped with no apply command, got %s", result.Status)
	}
}

func TestZshPluginsBatch(t *testing.T) {
	s := &ZshPluginsStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Items: []step.StepItem{
			{Name: "zsh-syntax-highlighting"},
			{Name: "zsh-autosuggestions"},
		},
	}
	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
	if len(result.ItemResults) != 2 {
		t.Errorf("expected 2 item results, got %d", len(result.ItemResults))
	}
}

func TestPluginRepoURL(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"zsh-syntax-highlighting", "https://github.com/zsh-users/zsh-syntax-highlighting.git"},
		{"zsh-autosuggestions", "https://github.com/zsh-users/zsh-autosuggestions.git"},
		{"unknown-plugin", ""},
	}
	for _, tt := range tests {
		got := pluginRepoURL(tt.name)
		if got != tt.want {
			t.Errorf("pluginRepoURL(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAggregateBatchResults(t *testing.T) {
	tests := []struct {
		name    string
		results []step.ItemResult
		want    step.StepStatus
	}{
		{
			"empty",
			nil,
			step.StatusSatisfied,
		},
		{
			"all satisfied",
			[]step.ItemResult{
				{Status: step.StatusSatisfied},
				{Status: step.StatusSatisfied},
			},
			step.StatusSatisfied,
		},
		{
			"all applied",
			[]step.ItemResult{
				{Status: step.StatusApplied},
				{Status: step.StatusApplied},
			},
			step.StatusApplied,
		},
		{
			"mixed satisfied and applied",
			[]step.ItemResult{
				{Status: step.StatusSatisfied},
				{Status: step.StatusApplied},
			},
			step.StatusApplied,
		},
		{
			"partial (some failed)",
			[]step.ItemResult{
				{Status: step.StatusApplied},
				{Status: step.StatusFailed},
			},
			step.StatusPartial,
		},
		{
			"all failed",
			[]step.ItemResult{
				{Status: step.StatusFailed},
				{Status: step.StatusFailed},
			},
			step.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregateBatchResults(tt.results)
			if got != tt.want {
				t.Errorf("aggregateBatchResults() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRunnerError(t *testing.T) {
	s := &HomebrewStep{run: mockRunnerError(fmt.Errorf("infrastructure failure"))}
	_, err := s.Check(context.Background(), step.StepConfig{})
	if err == nil {
		t.Error("expected error from runner")
	}
}

func TestStepNames(t *testing.T) {
	// Verify each step constructor returns a step with the correct name.
	steps := map[string]func() step.Step{
		"xcode-cli-tools": NewXcodeCLIToolsStep,
		"homebrew":        NewHomebrewStep,
		"apt-essentials":  NewAptEssentialsStep,
		"brew-packages":   NewBrewPackagesStep,
		"brew-casks":      NewBrewCasksStep,
		"apt-packages":    NewAptPackagesStep,
		"node-fnm":        NewNodeFnmStep,
		"python":          NewPythonStep,
		"go":              NewGoStep,
		"rust":            NewRustStep,
		"oh-my-zsh":       NewOhMyZshStep,
		"zsh-plugins":     NewZshPluginsStep,
		"shell-default":   NewShellDefaultStep,
		"macos-defaults":  NewMacOSDefaultsStep,
		"dock-layout":     NewDockLayoutStep,
		"machine-name":    NewMachineNameStep,
		"gsettings":       NewGSettingsStep,
		"directories":     NewDirectoriesStep,
		"git-config":      NewGitConfigStep,
		"ssh-keys":        NewSSHKeysStep,
	}

	for expectedName, constructor := range steps {
		s := constructor()
		if s.Name() != expectedName {
			t.Errorf("expected Name()=%q, got %q", expectedName, s.Name())
		}
	}
}

func TestExpandHome(t *testing.T) {
	result := expandHome("/absolute/path")
	if result != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %s", result)
	}

	// ~/something should be expanded.
	result = expandHome("~/projects")
	if result == "~/projects" {
		// This could happen if os.UserHomeDir fails, which is unlikely in tests.
		t.Log("expandHome did not expand ~, possibly no home dir available")
	}
}

func TestAptPackagesBatchApply(t *testing.T) {
	// check fails, apply succeeds, verify succeeds for each item
	callCount := 0
	runner := func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		callCount++
		switch callCount % 3 {
		case 1:
			return step.ExecResult{ExitCode: 1}, nil
		case 2:
			return step.ExecResult{ExitCode: 0}, nil
		case 0:
			return step.ExecResult{ExitCode: 0}, nil
		}
		return step.ExecResult{ExitCode: 0}, nil
	}

	s := &AptPackagesStep{run: runner}
	cfg := step.StepConfig{
		Items: []step.StepItem{
			{Name: "vim"},
			{Name: "curl", Version: "7.88.1-10+deb12u5"},
		},
	}
	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestGSettingsBatch(t *testing.T) {
	s := &GSettingsStep{run: mockRunner(0)}
	cfg := step.StepConfig{
		Items: []step.StepItem{
			{Name: "org.gnome.desktop.interface gtk-theme", Version: "'Adwaita-dark'"},
		},
	}
	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

// TestRunShellApply_FailureCapturesOutput verifies that runShellApply
// populates StepResult.Output with combined stdout+stderr on failure.
func TestRunShellApply_FailureCapturesOutput(t *testing.T) {
	runner := func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		return step.ExecResult{ExitCode: 1, Stdout: "out-line", Stderr: "err-line"}, nil
	}
	result, err := runShellApply(context.Background(), runner, "true", nil, "test-step")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != step.StatusFailed {
		t.Errorf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(result.Output, "out-line") {
		t.Errorf("Output missing stdout: %q", result.Output)
	}
	if !strings.Contains(result.Output, "err-line") {
		t.Errorf("Output missing stderr: %q", result.Output)
	}
}

// TestRunShellCheck_FailureCapturesOutput verifies the same for runShellCheck.
func TestRunShellCheck_FailureCapturesOutput(t *testing.T) {
	runner := func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		return step.ExecResult{ExitCode: 1, Stdout: "", Stderr: "check-err"}, nil
	}
	result, err := runShellCheck(context.Background(), runner, "false", nil, "test-step")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "check-err" {
		t.Errorf("Output = %q, want %q", result.Output, "check-err")
	}
}

// TestBatchApply_PerItemOutputCaptured verifies that batchApply preserves
// per-item Output in failed ItemResults.
func TestBatchApply_PerItemOutputCaptured(t *testing.T) {
	runner := func(ctx context.Context, cmd *step.ShellCommand, env []string, stepName, phase string) (step.ExecResult, error) {
		// Find ADZE_ITEM_NAME in env.
		itemName := ""
		for _, e := range env {
			if strings.HasPrefix(e, "ADZE_ITEM_NAME=") {
				itemName = strings.TrimPrefix(e, "ADZE_ITEM_NAME=")
				break
			}
		}
		if phase == "check" {
			return step.ExecResult{ExitCode: 1, Stderr: "not installed"}, nil
		}
		if itemName == "bad-pkg" {
			return step.ExecResult{ExitCode: 1, Stderr: "package bad-pkg not found"}, nil
		}
		return step.ExecResult{ExitCode: 0}, nil
	}
	cfg := step.StepConfig{
		Name:  "test-batch",
		Items: []step.StepItem{{Name: "good-pkg"}, {Name: "bad-pkg"}},
	}
	result, err := batchApply(context.Background(), runner, cfg, "test-batch",
		func(i step.StepItem) string { return "check " + i.Name },
		func(i step.StepItem) string { return "apply " + i.Name },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ItemResults) != 2 {
		t.Fatalf("got %d ItemResults, want 2", len(result.ItemResults))
	}
	var badItem *step.ItemResult
	for i := range result.ItemResults {
		if result.ItemResults[i].Item.Name == "bad-pkg" {
			badItem = &result.ItemResults[i]
			break
		}
	}
	if badItem == nil {
		t.Fatal("bad-pkg ItemResult not found")
	}
	if badItem.Status != step.StatusFailed {
		t.Errorf("bad-pkg status = %q, want failed", badItem.Status)
	}
	if !strings.Contains(badItem.Output, "package bad-pkg not found") {
		t.Errorf("bad-pkg Output = %q, want substring 'package bad-pkg not found'", badItem.Output)
	}
}

// TestExecuteBatchStep_RealBrewPackagesStep is an integration test exercising
// the real BrewPackagesStep through step.ExecuteBatchStep's nil-dispatch path.
// Catches wire-up regressions if someone reintroduces a per-item loop wrapper
// around the impl call.
func TestExecuteBatchStep_RealBrewPackagesStep(t *testing.T) {
	var applyCalls int
	installed := map[string]bool{}
	runner := func(ctx context.Context, cmd *step.ShellCommand, env []string, name, phase string) (step.ExecResult, error) {
		// Find the item name from env.
		itemName := ""
		for _, e := range env {
			if strings.HasPrefix(e, "ADZE_ITEM_NAME=") {
				itemName = strings.TrimPrefix(e, "ADZE_ITEM_NAME=")
				break
			}
		}
		switch phase {
		case "check":
			if installed[itemName] {
				return step.ExecResult{ExitCode: 0}, nil
			}
			return step.ExecResult{ExitCode: 1}, nil // not installed yet
		case "apply":
			applyCalls++
			installed[itemName] = true
			return step.ExecResult{ExitCode: 0}, nil
		}
		return step.ExecResult{}, nil
	}
	s := &BrewPackagesStep{run: runner}
	cfg := step.StepConfig{
		Name:         "brew-packages",
		Items:        []step.StepItem{{Name: "git"}, {Name: "jq"}},
		CheckTimeout: 5 * 1000 * 1000 * 1000, // 5s in ns
		ApplyTimeout: 5 * 1000 * 1000 * 1000,
	}
	result, err := step.ExecuteBatchStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("status = %q, want %q", result.Status, step.StatusApplied)
	}
	if len(result.ItemResults) != 2 {
		t.Fatalf("got %d items, want 2", len(result.ItemResults))
	}
	if applyCalls < 2 {
		t.Errorf("apply called %d times, want >= 2 (per-item)", applyCalls)
	}
}
