package steps

import (
	"testing"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/step"
)

func TestBuildStepConfigsMinimal(t *testing.T) {
	// Minimal config: no packages, no defaults, etc.
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	// Should produce step configs for core/infra steps that don't require config.
	// Batch steps with no items should be filtered out.
	for _, sc := range configs {
		if sc.Name == "brew-packages" {
			t.Error("brew-packages should be filtered out with no packages.brew config")
		}
		if sc.Name == "brew-casks" {
			t.Error("brew-casks should be filtered out with no packages.cask config")
		}
		if sc.Name == "directories" {
			t.Error("directories should be filtered out with no directories config")
		}
		if sc.Name == "macos-defaults" {
			t.Error("macos-defaults should be filtered out with no defaults config")
		}
		if sc.Name == "dock-layout" {
			t.Error("dock-layout should be filtered out with no dock config")
		}
	}

	// Core infrastructure steps should always be present (on darwin).
	names := stepConfigNames(configs)
	for _, expected := range []string{"xcode-cli-tools", "homebrew"} {
		if !names[expected] {
			t.Errorf("expected step %q in minimal darwin config", expected)
		}
	}
}

func TestBuildStepConfigsWithPackages(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{
				{Name: "git"},
				{Name: "jq", Version: "1.7"},
			},
			Cask: []config.PackageEntry{
				{Name: "visual-studio-code"},
			},
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["brew-packages"] {
		t.Error("expected brew-packages step with packages.brew config")
	}
	if !names["brew-casks"] {
		t.Error("expected brew-casks step with packages.cask config")
	}

	// Check that brew-packages has the right items.
	for _, sc := range configs {
		if sc.Name == "brew-packages" {
			if len(sc.Items) != 2 {
				t.Errorf("expected 2 brew-packages items, got %d", len(sc.Items))
			}
			if sc.Items[0].Name != "git" {
				t.Errorf("expected first item 'git', got %q", sc.Items[0].Name)
			}
			if sc.Items[1].Version != "1.7" {
				t.Errorf("expected second item version '1.7', got %q", sc.Items[1].Version)
			}
		}
		if sc.Name == "brew-casks" {
			if len(sc.Items) != 1 {
				t.Errorf("expected 1 brew-casks item, got %d", len(sc.Items))
			}
		}
	}
}

func TestBuildStepConfigsWithShell(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Shell: config.ShellConfig{
			Default: "zsh",
			OhMyZsh: true,
			Plugins: []string{"zsh-syntax-highlighting", "zsh-autosuggestions"},
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["shell-default"] {
		t.Error("expected shell-default step")
	}
	if !names["oh-my-zsh"] {
		t.Error("expected oh-my-zsh step")
	}
	if !names["zsh-plugins"] {
		t.Error("expected zsh-plugins step")
	}

	// Check shell-default has proper check/apply.
	for _, sc := range configs {
		if sc.Name == "shell-default" {
			if sc.Check == nil {
				t.Error("shell-default should have Check command")
			}
			if sc.Apply == nil {
				t.Error("shell-default should have Apply command")
			}
		}
		if sc.Name == "zsh-plugins" {
			if len(sc.Items) != 2 {
				t.Errorf("expected 2 zsh-plugins items, got %d", len(sc.Items))
			}
		}
	}
}

func TestBuildStepConfigsWithIdentity(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Identity: config.IdentityConfig{
			GitName:    "Test User",
			GitEmail:   "test@example.com",
			GithubUser: "testuser",
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["git-config"] {
		t.Error("expected git-config step with identity config")
	}

	for _, sc := range configs {
		if sc.Name == "git-config" {
			if sc.Check == nil {
				t.Error("git-config should have Check command")
			}
			if sc.Apply == nil {
				t.Error("git-config should have Apply command")
			}
		}
	}
}

func TestBuildStepConfigsWithMachineName(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Machine: config.MachineConfig{
			Hostname: "my-mac",
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["machine-name"] {
		t.Error("expected machine-name step with machine.hostname config")
	}

	for _, sc := range configs {
		if sc.Name == "machine-name" {
			if sc.Check == nil {
				t.Error("machine-name should have Check command")
			}
			if sc.Apply == nil {
				t.Error("machine-name should have Apply command")
			}
		}
	}
}

func TestBuildStepConfigsWithDirectories(t *testing.T) {
	cfg := &config.Config{
		Name:        "test",
		Platform:    "darwin",
		Directories: []string{"~/projects", "~/bin"},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["directories"] {
		t.Error("expected directories step with directories config")
	}

	for _, sc := range configs {
		if sc.Name == "directories" {
			if len(sc.Items) != 2 {
				t.Errorf("expected 2 directory items, got %d", len(sc.Items))
			}
		}
	}
}

func TestBuildStepConfigsWithDock(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Dock: config.DockConfig{
			Apps: []string{"Safari", "Terminal", "Visual Studio Code"},
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["dock-layout"] {
		t.Error("expected dock-layout step with dock config")
	}

	for _, sc := range configs {
		if sc.Name == "dock-layout" {
			if len(sc.Items) != 3 {
				t.Errorf("expected 3 dock-layout items, got %d", len(sc.Items))
			}
		}
	}
}

func TestBuildStepConfigsWithCustomSteps(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		CustomSteps: map[string]config.CustomStep{
			"my-custom-step": {
				Description: "A custom step",
				Provides:    []string{"custom"},
				Check:       "test -f /tmp/custom",
				Apply: map[string]string{
					"darwin": "touch /tmp/custom",
				},
				Platform: []string{"darwin"},
			},
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["my-custom-step"] {
		t.Error("expected my-custom-step in configs")
	}

	for _, sc := range configs {
		if sc.Name == "my-custom-step" {
			if sc.Description != "A custom step" {
				t.Errorf("expected description 'A custom step', got %q", sc.Description)
			}
			if len(sc.Provides) != 1 || sc.Provides[0] != "custom" {
				t.Errorf("expected provides=[custom], got %v", sc.Provides)
			}
			if sc.Check == nil {
				t.Error("custom step should have Check command")
			}
			if sc.Apply == nil {
				t.Error("custom step should have Apply command for darwin")
			}
		}
	}
}

func TestBuildStepConfigsCustomStepPlatformFilter(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "ubuntu",
		CustomSteps: map[string]config.CustomStep{
			"darwin-only-step": {
				Description: "Darwin only",
				Check:       "true",
				Apply:       map[string]string{"darwin": "true"},
				Platform:    []string{"darwin"},
			},
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "ubuntu", reg)

	names := stepConfigNames(configs)
	if names["darwin-only-step"] {
		t.Error("darwin-only custom step should be filtered out on ubuntu")
	}
}

func TestBuildStepConfigsDefaultTimeouts(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	for _, sc := range configs {
		if sc.CheckTimeout != step.DefaultCheckTimeout {
			t.Errorf("step %q: expected CheckTimeout %s, got %s", sc.Name, step.DefaultCheckTimeout, sc.CheckTimeout)
		}
		if sc.ApplyTimeout != step.DefaultApplyTimeout {
			t.Errorf("step %q: expected ApplyTimeout %s, got %s", sc.Name, step.DefaultApplyTimeout, sc.ApplyTimeout)
		}
	}
}

func TestBuildStepConfigsUbuntuPlatform(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "ubuntu",
		Packages: config.PackagesConfig{
			Apt: []config.PackageEntry{
				{Name: "vim"},
				{Name: "tmux"},
			},
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "ubuntu", reg)

	names := stepConfigNames(configs)
	if !names["apt-essentials"] {
		t.Error("expected apt-essentials step on ubuntu")
	}
	if !names["apt-packages"] {
		t.Error("expected apt-packages step on ubuntu")
	}
	if names["brew-packages"] {
		t.Error("brew-packages should not appear on ubuntu")
	}
	if names["homebrew"] {
		t.Error("homebrew should not appear on ubuntu")
	}
}

func TestBuildStepConfigsWithDefaults(t *testing.T) {
	cfg := &config.Config{
		Name:     "test",
		Platform: "darwin",
		Defaults: map[string]map[string]config.DefaultValue{
			"com.apple.dock": {
				"autohide": {Value: true},
				"tilesize": {Value: 48},
			},
		},
	}
	reg := NewRegistry()
	configs := BuildStepConfigs(cfg, "darwin", reg)

	names := stepConfigNames(configs)
	if !names["macos-defaults"] {
		t.Error("expected macos-defaults step with defaults config")
	}

	for _, sc := range configs {
		if sc.Name == "macos-defaults" {
			if len(sc.Items) != 2 {
				t.Errorf("expected 2 macos-defaults items, got %d", len(sc.Items))
			}
		}
	}
}

func TestResolveShellPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"zsh", "/bin/zsh"},
		{"bash", "/bin/bash"},
		{"fish", "/usr/local/bin/fish"},
		{"/usr/bin/zsh", "/usr/bin/zsh"},
		{"ksh", "/bin/ksh"},
	}
	for _, tt := range tests {
		got := resolveShellPath(tt.input)
		if got != tt.want {
			t.Errorf("resolveShellPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildGitConfigCheck(t *testing.T) {
	id := config.IdentityConfig{
		GitName:    "Test User",
		GitEmail:   "test@example.com",
		GithubUser: "testuser",
	}
	cmd := buildGitConfigCheck(id)
	if cmd == nil {
		t.Fatal("expected non-nil check command")
	}
	// Should be sh -c "..."
	if len(cmd.Args) != 3 || cmd.Args[0] != "sh" {
		t.Errorf("expected sh -c command, got %v", cmd.Args)
	}
}

func TestBuildGitConfigApply(t *testing.T) {
	id := config.IdentityConfig{
		GitName:  "Test User",
		GitEmail: "test@example.com",
	}
	cmd := buildGitConfigApply(id)
	if cmd == nil {
		t.Fatal("expected non-nil apply command")
	}
	if len(cmd.Args) != 3 || cmd.Args[0] != "sh" {
		t.Errorf("expected sh -c command, got %v", cmd.Args)
	}
}

func TestBuildGitConfigCheckEmpty(t *testing.T) {
	id := config.IdentityConfig{}
	cmd := buildGitConfigCheck(id)
	if cmd != nil {
		t.Error("expected nil check command for empty identity")
	}
}

func TestDefaultsToItemsDarwin(t *testing.T) {
	defaults := map[string]map[string]config.DefaultValue{
		"com.apple.dock": {
			"autohide": {Value: true},
			"tilesize": {Value: 48},
		},
	}
	items := defaultsToItems(defaults, "darwin")
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// Check that items have proper format.
	for _, item := range items {
		if item.Name == "" {
			t.Error("item Name should not be empty")
		}
		if item.Version == "" {
			t.Error("item Version should not be empty")
		}
	}
}

func TestDefaultsToItemsNil(t *testing.T) {
	items := defaultsToItems(nil, "darwin")
	if items != nil {
		t.Error("expected nil items for nil defaults")
	}
}

// stepConfigNames returns a set of step config names for easy lookup.
func stepConfigNames(configs []step.StepConfig) map[string]bool {
	names := make(map[string]bool)
	for _, sc := range configs {
		names[sc.Name] = true
	}
	return names
}
