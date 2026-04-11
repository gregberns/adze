package steps

import (
	"testing"
)

func TestNewRegistryPopulatesAllSteps(t *testing.T) {
	reg := NewRegistry()

	// The spec defines 20 distinct built-in steps.
	expectedCount := 20
	if reg.Count() != expectedCount {
		t.Errorf("expected %d registered steps, got %d", expectedCount, reg.Count())
	}
}

func TestRegistryGetByName(t *testing.T) {
	reg := NewRegistry()

	tests := []struct {
		name     string
		wantOK   bool
		wantType string
	}{
		{"xcode-cli-tools", true, "atomic"},
		{"homebrew", true, "atomic"},
		{"apt-essentials", true, "atomic"},
		{"brew-packages", true, "batch"},
		{"brew-casks", true, "batch"},
		{"apt-packages", true, "batch"},
		{"node-fnm", true, "atomic"},
		{"python", true, "atomic"},
		{"go", true, "atomic"},
		{"rust", true, "atomic"},
		{"oh-my-zsh", true, "atomic"},
		{"zsh-plugins", true, "batch"},
		{"shell-default", true, "atomic"},
		{"macos-defaults", true, "batch"},
		{"dock-layout", true, "batch"},
		{"machine-name", true, "atomic"},
		{"gsettings", true, "batch"},
		{"directories", true, "batch"},
		{"git-config", true, "atomic"},
		{"ssh-keys", true, "atomic"},
		{"nonexistent", false, ""},
	}

	for _, tt := range tests {
		def, ok := reg.Get(tt.name)
		if ok != tt.wantOK {
			t.Errorf("Get(%q): got ok=%v, want ok=%v", tt.name, ok, tt.wantOK)
			continue
		}
		if ok && def.Type != tt.wantType {
			t.Errorf("Get(%q): got type=%q, want type=%q", tt.name, def.Type, tt.wantType)
		}
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("does-not-exist")
	if ok {
		t.Error("expected Get to return false for nonexistent step")
	}
}

func TestRegistryAll(t *testing.T) {
	reg := NewRegistry()
	all := reg.All()

	if len(all) != reg.Count() {
		t.Errorf("All() returned %d steps, Count() reports %d", len(all), reg.Count())
	}

	// Verify sorted by category order.
	prevOrder := -1
	prevName := ""
	for _, def := range all {
		order := categoryOrder(def.Category)
		if order < prevOrder {
			t.Errorf("steps out of order: %q (category %q, order %d) came after %q (order %d)",
				def.Name, def.Category, order, prevName, prevOrder)
		}
		if order == prevOrder && def.Name < prevName {
			t.Errorf("steps within same category out of order: %q came after %q", def.Name, prevName)
		}
		prevOrder = order
		prevName = def.Name
	}
}

func TestRegistryForPlatformDarwin(t *testing.T) {
	reg := NewRegistry()
	darwinSteps := reg.ForPlatform("darwin")

	// darwin should include darwin-specific steps and "any" platform steps.
	names := make(map[string]bool)
	for _, def := range darwinSteps {
		names[def.Name] = true
	}

	// Must include darwin-specific.
	for _, expected := range []string{"xcode-cli-tools", "homebrew", "brew-packages", "brew-casks", "macos-defaults", "dock-layout", "machine-name"} {
		if !names[expected] {
			t.Errorf("ForPlatform(darwin) should include %q", expected)
		}
	}

	// Must include "any" platform steps.
	for _, expected := range []string{"rust", "oh-my-zsh", "directories", "git-config", "ssh-keys"} {
		if !names[expected] {
			t.Errorf("ForPlatform(darwin) should include %q (any platform)", expected)
		}
	}

	// Must NOT include ubuntu/debian-only steps.
	for _, excluded := range []string{"apt-essentials", "apt-packages", "gsettings"} {
		if names[excluded] {
			t.Errorf("ForPlatform(darwin) should NOT include %q", excluded)
		}
	}
}

func TestRegistryForPlatformUbuntu(t *testing.T) {
	reg := NewRegistry()
	ubuntuSteps := reg.ForPlatform("ubuntu")

	names := make(map[string]bool)
	for _, def := range ubuntuSteps {
		names[def.Name] = true
	}

	// Must include ubuntu-specific.
	for _, expected := range []string{"apt-essentials", "apt-packages", "gsettings"} {
		if !names[expected] {
			t.Errorf("ForPlatform(ubuntu) should include %q", expected)
		}
	}

	// Must include "any" platform steps.
	for _, expected := range []string{"rust", "oh-my-zsh", "directories", "git-config", "ssh-keys"} {
		if !names[expected] {
			t.Errorf("ForPlatform(ubuntu) should include %q (any platform)", expected)
		}
	}

	// Must NOT include darwin-only steps.
	for _, excluded := range []string{"xcode-cli-tools", "homebrew", "brew-packages", "brew-casks", "macos-defaults", "dock-layout", "machine-name"} {
		if names[excluded] {
			t.Errorf("ForPlatform(ubuntu) should NOT include %q", excluded)
		}
	}
}

func TestRegistryStepConstructors(t *testing.T) {
	reg := NewRegistry()
	all := reg.All()

	for _, def := range all {
		if def.Constructor == nil {
			t.Errorf("step %q has nil Constructor", def.Name)
			continue
		}
		s := def.Constructor()
		if s == nil {
			t.Errorf("step %q Constructor returned nil", def.Name)
			continue
		}
		if s.Name() != def.Name {
			t.Errorf("step %q Constructor returned step with Name()=%q", def.Name, s.Name())
		}
	}
}

func TestRegistryStepProvidesNonEmpty(t *testing.T) {
	reg := NewRegistry()
	all := reg.All()

	for _, def := range all {
		if len(def.Provides) == 0 {
			t.Errorf("step %q has empty Provides", def.Name)
		}
	}
}

func TestRegistryCategories(t *testing.T) {
	reg := NewRegistry()
	all := reg.All()

	validCategories := map[string]bool{
		"core": true, "packages": true, "languages": true,
		"shell": true, "system": true, "generic": true,
	}

	for _, def := range all {
		if !validCategories[def.Category] {
			t.Errorf("step %q has invalid category %q", def.Name, def.Category)
		}
	}
}

func TestRegistryMultiProvides(t *testing.T) {
	reg := NewRegistry()

	// node-fnm provides both node and fnm.
	def, ok := reg.Get("node-fnm")
	if !ok {
		t.Fatal("node-fnm not found in registry")
	}
	if len(def.Provides) != 2 {
		t.Errorf("node-fnm should provide 2 capabilities, got %d", len(def.Provides))
	}
	provides := make(map[string]bool)
	for _, p := range def.Provides {
		provides[p] = true
	}
	if !provides["node"] || !provides["fnm"] {
		t.Errorf("node-fnm should provide node and fnm, got %v", def.Provides)
	}

	// rust provides both rust and cargo.
	def, ok = reg.Get("rust")
	if !ok {
		t.Fatal("rust not found in registry")
	}
	if len(def.Provides) != 2 {
		t.Errorf("rust should provide 2 capabilities, got %d", len(def.Provides))
	}
	provides = make(map[string]bool)
	for _, p := range def.Provides {
		provides[p] = true
	}
	if !provides["rust"] || !provides["cargo"] {
		t.Errorf("rust should provide rust and cargo, got %v", def.Provides)
	}
}
