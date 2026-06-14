package cli

import (
	"context"
	"testing"
	"time"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/step"
	"github.com/gregberns/adze/internal/steps"
)

// resolveGraph exercises the full wiring pipeline (BuildStepConfigs →
// stepConfigsToDagInputs → dag.Resolve) and returns the resolved step names
// as a set. Mirrors the path taken by `adze plan` and `adze apply`.
func resolveGraph(t *testing.T, cfg *config.Config, platform string) map[string]bool {
	t.Helper()
	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, platform, reg)
	inputs := stepConfigsToDagInputs(stepConfigs)
	graph, errs := dag.Resolve(inputs, platform, nil)
	if len(errs) > 0 {
		t.Fatalf("dag.Resolve errors: %v", errs)
	}
	names := make(map[string]bool, len(graph.Steps))
	for _, rs := range graph.Steps {
		names[rs.Name] = true
	}
	return names
}

// R5.1 — greg-berns-shaped config (no language requests, no SSH key flag).
// Must not surface rust/go/python/node-fnm/ssh-keys; must include the brew
// + defaults + identity stack.
func TestScenario_GregBernsShaped_NoSurprises(t *testing.T) {
	cfg := &config.Config{
		Name:     "GREG-BERNS",
		Platform: "darwin",
		Identity: config.IdentityConfig{
			GitName:    "Greg Berns",
			GitEmail:   "greg.berns@omaticsoftware.com",
			GithubUser: "gregberns",
		},
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{
				{Name: "git"}, {Name: "gh"}, {Name: "jq"},
			},
			Cask: []config.PackageEntry{
				{Name: "iterm2"}, {Name: "visual-studio-code"},
			},
		},
		Defaults: map[string]map[string]config.DefaultValue{
			"com.apple.finder": {"ShowPathbar": {Value: true}},
		},
		Directories: []string{"~/repos", "~/github"},
		Shell: config.ShellConfig{
			Default: "zsh",
		},
	}
	names := resolveGraph(t, cfg, "darwin")

	for _, expected := range []string{
		"xcode-cli-tools", "homebrew",
		"brew-packages", "brew-casks",
		"macos-defaults", "git-config", "directories", "shell-default",
	} {
		if !names[expected] {
			t.Errorf("expected %q in resolved graph; got %v", expected, names)
		}
	}

	for _, forbidden := range []string{"rust", "go", "python", "node-fnm", "ssh-keys"} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear in greg-berns plan; got %v", forbidden, names)
		}
	}
}

// R5.2 — minimal config produces a zero-step plan.
func TestScenario_EmptyConfig(t *testing.T) {
	cfg := &config.Config{Name: "minimal", Platform: "darwin"}
	names := resolveGraph(t, cfg, "darwin")

	if len(names) != 0 {
		t.Errorf("empty config should produce zero-step plan; got %d steps: %v", len(names), names)
	}
}

// R5.3 — full dev config preserves expected steps; rust/ssh-keys gated out.
func TestScenario_FullDevConfig(t *testing.T) {
	cfg := &config.Config{
		Name:     "dev",
		Platform: "darwin",
		Identity: config.IdentityConfig{
			GitName:  "Dev User",
			GitEmail: "dev@example.com",
		},
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{{Name: "git"}, {Name: "jq"}},
			Cask: []config.PackageEntry{{Name: "iterm2"}},
		},
		Defaults: map[string]map[string]config.DefaultValue{
			"NSGlobalDomain": {"AppleShowAllExtensions": {Value: true}},
		},
		Shell: config.ShellConfig{
			Default: "zsh",
			OhMyZsh: true,
			Plugins: []string{"git"},
		},
		Directories: []string{"~/Projects"},
	}
	names := resolveGraph(t, cfg, "darwin")

	for _, expected := range []string{
		"xcode-cli-tools", "homebrew",
		"brew-packages", "brew-casks",
		"macos-defaults",
		"oh-my-zsh", "zsh-plugins", "shell-default",
		"git-config", "directories",
	} {
		if !names[expected] {
			t.Errorf("expected %q in full-dev plan; got %v", expected, names)
		}
	}

	for _, forbidden := range []string{"rust", "ssh-keys"} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear in full-dev plan (not requested); got %v", forbidden, names)
		}
	}
}

// R5.3b — full dev config with the SSH flag → ssh-keys appears.
func TestScenario_FullDevConfig_WithSSHFlag(t *testing.T) {
	cfg := &config.Config{
		Name:     "dev",
		Platform: "darwin",
		Identity: config.IdentityConfig{
			GitEmail:       "dev@example.com",
			GenerateSSHKey: true,
		},
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{{Name: "git"}},
		},
	}
	names := resolveGraph(t, cfg, "darwin")

	if !names["ssh-keys"] {
		t.Errorf("ssh-keys must appear when generate_ssh_key is true; got %v", names)
	}
}

// R5.4 — transitive language pull through the full pipeline (depth 3).
func TestScenario_TransitiveLanguagePull(t *testing.T) {
	cfg := &config.Config{
		Name:     "needs-go",
		Platform: "darwin",
		CustomSteps: map[string]config.CustomStep{
			"go-app": {
				Description: "go application",
				Provides:    []string{"go-app"},
				Requires:    []string{"go"},
				Platform:    []string{"darwin"},
				Check:       "command -v go-app",
				Apply:       map[string]string{"darwin": "echo install"},
			},
		},
	}
	names := resolveGraph(t, cfg, "darwin")

	for _, expected := range []string{"go", "homebrew", "xcode-cli-tools", "go-app"} {
		if !names[expected] {
			t.Errorf("expected %q in plan; got %v", expected, names)
		}
	}

	for _, forbidden := range []string{"rust", "python", "node-fnm"} {
		if names[forbidden] {
			t.Errorf("step %q must NOT appear (only go was required); got %v", forbidden, names)
		}
	}
}

// R5.5 — examples/macos-dev.yaml on disk parses cleanly and produces an
// expected plan. Anchors regression coverage on a real config file.
func TestScenario_MacOSDevExampleFile(t *testing.T) {
	cfg, valErrs, _, err := config.LoadConfig("../../examples/macos-dev.yaml", false)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(valErrs) > 0 {
		t.Fatalf("validation errors: %v", valErrs)
	}

	names := resolveGraph(t, cfg, "darwin")

	for _, expected := range []string{
		"homebrew", "brew-packages", "brew-casks",
		"macos-defaults", "dock-layout",
		"oh-my-zsh", "zsh-plugins",
		"git-config", "directories",
	} {
		if !names[expected] {
			t.Errorf("expected %q in plan from examples/macos-dev.yaml; got %v", expected, names)
		}
	}

	if names["rust"] {
		t.Error("rust must NOT appear in plan from examples/macos-dev.yaml — regression of the wiring bug")
	}
}

// fakeStep is a minimal Step impl for scenario dispatch tests.
type fakeStep struct {
	name        string
	applyResult step.StepResult
	applyCalled int
}

func (f *fakeStep) Name() string { return f.name }
func (f *fakeStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	return step.StepResult{Status: step.StatusFailed, Reason: "not satisfied"}, nil
}
func (f *fakeStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	f.applyCalled++
	return f.applyResult, nil
}

// TestScenario_OhMyZshDispatched verifies that the executor calls
// the impl's Apply (rather than short-circuiting with "no apply command")
// for built-in atomic steps whose StepConfig has nil Apply.
func TestScenario_OhMyZshDispatched(t *testing.T) {
	checkCount := 0
	fake := &fakeStep2{
		applyResult: step.StepResult{Status: step.StatusApplied},
		checkFlip:   &checkCount,
	}
	cfg := step.StepConfig{
		Name:         "oh-my-zsh",
		Provides:     []string{"oh-my-zsh"},
		CheckTimeout: time.Second,
		ApplyTimeout: time.Second,
	}
	result, err := step.ExecuteStep(context.Background(), fake, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.applyCalled != 1 {
		t.Errorf("Apply called %d times, want 1", fake.applyCalled)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("status = %q, want %q", result.Status, step.StatusApplied)
	}
}

// fakeStep2 flips check between Failed (first call) and Satisfied (verify).
type fakeStep2 struct {
	applyResult step.StepResult
	applyCalled int
	checkFlip   *int
}

func (f *fakeStep2) Name() string { return "fake" }
func (f *fakeStep2) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	*f.checkFlip++
	if *f.checkFlip == 1 {
		return step.StepResult{Status: step.StatusFailed}, nil
	}
	return step.StepResult{Status: step.StatusSatisfied}, nil
}
func (f *fakeStep2) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	f.applyCalled++
	return f.applyResult, nil
}

// TestScenario_BrewPackagesDispatched verifies the same for batch steps:
// a built-in batch impl with nil Apply in its StepConfig still gets its
// Apply method called once for the whole batch.
func TestScenario_BrewPackagesDispatched(t *testing.T) {
	fake := &fakeStep{
		name: "brew-packages",
		applyResult: step.StepResult{
			Status: step.StatusApplied,
			ItemResults: []step.ItemResult{
				{Item: step.StepItem{Name: "git"}, Status: step.StatusApplied},
			},
		},
	}
	cfg := step.StepConfig{
		Name:         "brew-packages",
		Items:        []step.StepItem{{Name: "git"}},
		CheckTimeout: time.Second,
		ApplyTimeout: time.Second,
	}
	result, err := step.ExecuteBatchStep(context.Background(), fake, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.applyCalled != 1 {
		t.Errorf("Apply called %d times, want 1", fake.applyCalled)
	}
	if result.Status != step.StatusApplied {
		t.Errorf("status = %q, want %q", result.Status, step.StatusApplied)
	}
}

// TestScenario_SudoWarningInPlan verifies plan output includes a Notes
// section when the config requests a known sudo-requiring cask.
func TestScenario_SudoWarningInPlan(t *testing.T) {
	notices := steps.SudoStepsForConfig(&config.Config{
		Packages: config.PackagesConfig{
			Cask: []config.PackageEntry{{Name: "docker"}},
		},
	})
	if len(notices) == 0 {
		t.Fatal("expected sudo notices for docker, got none")
	}
	// The plan formatter is exercised by outputPlanHuman; here we just
	// verify the data plumbing produces a non-empty notice slice.
	if notices[0].ItemName != "docker" {
		t.Errorf("first notice item = %q, want docker", notices[0].ItemName)
	}
}

// TestScenario_NoSudoWarning_NoMatches verifies no notices when no
// sudo-requiring casks/steps are configured.
func TestScenario_NoSudoWarning_NoMatches(t *testing.T) {
	notices := steps.SudoStepsForConfig(&config.Config{
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{{Name: "git"}},
			Cask: []config.PackageEntry{{Name: "iterm2"}},
		},
	})
	if len(notices) != 0 {
		t.Errorf("expected zero notices, got %+v", notices)
	}
}
