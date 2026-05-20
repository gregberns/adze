package cli

import (
	"testing"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
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
