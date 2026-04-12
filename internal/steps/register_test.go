package steps

import (
	"testing"
)

func TestRequiresForPlatform_WithPlatformRequires(t *testing.T) {
	def := StepDefinition{
		Name:      "test-step",
		Platforms: []string{"darwin", "ubuntu"},
		PlatformRequires: map[string][]string{
			"darwin": {"homebrew"},
			"ubuntu": {"apt-essentials"},
		},
	}

	darwinReqs := def.RequiresForPlatform("darwin")
	if len(darwinReqs) != 1 || darwinReqs[0] != "homebrew" {
		t.Errorf("darwin requires = %v, want [homebrew]", darwinReqs)
	}

	ubuntuReqs := def.RequiresForPlatform("ubuntu")
	if len(ubuntuReqs) != 1 || ubuntuReqs[0] != "apt-essentials" {
		t.Errorf("ubuntu requires = %v, want [apt-essentials]", ubuntuReqs)
	}
}

func TestRequiresForPlatform_FallbackToDefault(t *testing.T) {
	def := StepDefinition{
		Name:     "test-step",
		Requires: []string{"something"},
	}

	reqs := def.RequiresForPlatform("darwin")
	if len(reqs) != 1 || reqs[0] != "something" {
		t.Errorf("darwin requires = %v, want [something]", reqs)
	}
}

func TestRequiresForPlatform_UnknownPlatformFallsBack(t *testing.T) {
	def := StepDefinition{
		Name:     "test-step",
		Requires: []string{"default-dep"},
		PlatformRequires: map[string][]string{
			"darwin": {"homebrew"},
		},
	}

	// Unknown platform should fall back to default Requires
	reqs := def.RequiresForPlatform("freebsd")
	if len(reqs) != 1 || reqs[0] != "default-dep" {
		t.Errorf("freebsd requires = %v, want [default-dep]", reqs)
	}
}

func TestRequiresForPlatform_NilPlatformRequires(t *testing.T) {
	def := StepDefinition{
		Name: "test-step",
	}

	reqs := def.RequiresForPlatform("darwin")
	if reqs != nil {
		t.Errorf("requires = %v, want nil", reqs)
	}
}

func TestNodeFnmPythonGoRequireCorrectPlatformDeps(t *testing.T) {
	reg := NewRegistry()

	tests := []struct {
		stepName       string
		darwinRequires string
		ubuntuRequires string
	}{
		{"node-fnm", "homebrew", "apt-essentials"},
		{"python", "homebrew", "apt-essentials"},
		{"go", "homebrew", "apt-essentials"},
	}

	for _, tt := range tests {
		def, ok := reg.Get(tt.stepName)
		if !ok {
			t.Fatalf("step %q not found in registry", tt.stepName)
		}

		darwinReqs := def.RequiresForPlatform("darwin")
		if len(darwinReqs) != 1 || darwinReqs[0] != tt.darwinRequires {
			t.Errorf("%s darwin requires = %v, want [%s]", tt.stepName, darwinReqs, tt.darwinRequires)
		}

		ubuntuReqs := def.RequiresForPlatform("ubuntu")
		if len(ubuntuReqs) != 1 || ubuntuReqs[0] != tt.ubuntuRequires {
			t.Errorf("%s ubuntu requires = %v, want [%s]", tt.stepName, ubuntuReqs, tt.ubuntuRequires)
		}

		// Default Requires should be nil since these use PlatformRequires exclusively
		if def.Requires != nil {
			t.Errorf("%s has non-nil default Requires %v, want nil", tt.stepName, def.Requires)
		}
	}
}

func TestLanguageStepsNoDefaultRequires(t *testing.T) {
	// Ensure that node-fnm, python, go do NOT have a bare Requires field set,
	// which would incorrectly apply to all platforms.
	reg := NewRegistry()

	for _, name := range []string{"node-fnm", "python", "go"} {
		def, ok := reg.Get(name)
		if !ok {
			t.Fatalf("step %q not found in registry", name)
		}
		if len(def.Requires) > 0 {
			t.Errorf("%s has default Requires %v; should use PlatformRequires instead", name, def.Requires)
		}
		if def.PlatformRequires == nil {
			t.Errorf("%s has nil PlatformRequires; expected platform-specific requires", name)
		}
	}
}
