package adapter

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// mockRunner creates a commandRunner that returns predefined responses based on
// the command and arguments.
type mockCall struct {
	name string
	args []string
	out  string
	err  error
}

func newMockRunner(calls []mockCall) commandRunner {
	idx := 0
	return func(name string, args ...string) ([]byte, error) {
		if idx >= len(calls) {
			return nil, fmt.Errorf("unexpected call #%d: %s %v", idx, name, args)
		}
		c := calls[idx]
		idx++
		return []byte(c.out), c.err
	}
}

// newSimpleMockRunner creates a mock runner that matches on command name and
// first argument patterns. More flexible for complex scenarios.
func newSimpleMockRunner(responses map[string]mockCall) commandRunner {
	return func(name string, args ...string) ([]byte, error) {
		// Build lookup keys from most specific to least.
		key := name
		if len(args) > 0 {
			key = name + " " + strings.Join(args, " ")
		}

		// Try exact match first.
		if c, ok := responses[key]; ok {
			return []byte(c.out), c.err
		}

		// Try prefix matches (name + first arg, name + first two args, etc.)
		for i := len(args); i > 0; i-- {
			partial := name + " " + strings.Join(args[:i], " ")
			if c, ok := responses[partial]; ok {
				return []byte(c.out), c.err
			}
		}

		// Try just the command name.
		if c, ok := responses[name]; ok {
			return []byte(c.out), c.err
		}

		return nil, fmt.Errorf("unexpected command: %s %v", name, args)
	}
}

// --- Darwin Formula Check Tests ---

func TestDarwinPackageCheck_FormulaInstalled(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"list", "--formula", "git"}, out: "git"},
	})
	d := NewDarwinAdapter(runner)
	installed, err := d.PackageCheck(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Fatal("expected package to be installed")
	}
}

func TestDarwinPackageCheck_FormulaNotInstalled(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"list", "--formula", "git"}, err: fmt.Errorf("exit 1")},
	})
	d := NewDarwinAdapter(runner)
	installed, err := d.PackageCheck(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatal("expected package to not be installed")
	}
}

func TestDarwinPackageCheck_VersionedFormula(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"list", "--formula", "node@20"}, out: "node@20"},
	})
	d := NewDarwinAdapter(runner)
	installed, err := d.PackageCheck(Package{Name: "node", Version: "20"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Fatal("expected versioned formula to be installed")
	}
}

func TestDarwinPackageCheck_BrewNotFound(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, err: fmt.Errorf("not found")},
	})
	d := NewDarwinAdapter(runner)
	_, err := d.PackageCheck(Package{Name: "git"})
	if !errors.Is(err, ErrBrewNotFound) {
		t.Fatalf("expected ErrBrewNotFound, got %v", err)
	}
}

// --- Darwin Formula Install Tests ---

func TestDarwinPackageInstall_Formula(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"install", "git"}, out: ""},
		{name: "brew", args: []string{"info", "--json", "git"}, out: `[{"keg_only":false}]`},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageInstall(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinPackageInstall_FormulaWithPin(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"install", "git"}, out: ""},
		{name: "brew", args: []string{"info", "--json", "git"}, out: `[{"keg_only":false}]`},
		{name: "brew", args: []string{"pin", "git"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageInstall(Package{Name: "git", Pinned: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinPackageInstall_VersionedFormula(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"install", "node@20"}, out: ""},
		{name: "brew", args: []string{"info", "--json", "node@20"}, out: `[{"keg_only":true}]`},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageInstall(Package{Name: "node", Version: "20"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinPackageInstall_CaskPinnedError(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageInstall(Package{Name: "firefox", Cask: true, Pinned: true})
	if !errors.Is(err, ErrCaskPinNotSupported) {
		t.Fatalf("expected ErrCaskPinNotSupported, got %v", err)
	}
}

func TestDarwinPackageInstall_Cask(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"install", "--cask", "firefox"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageInstall(Package{Name: "firefox", Cask: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinPackageInstall_BrewNotFound(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, err: fmt.Errorf("not found")},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageInstall(Package{Name: "git"})
	if !errors.Is(err, ErrBrewNotFound) {
		t.Fatalf("expected ErrBrewNotFound, got %v", err)
	}
}

// --- Darwin Cask Check Tests ---

func TestDarwinPackageCheck_CaskInstalled(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"list", "--cask", "firefox"}, out: "firefox"},
	})
	d := NewDarwinAdapter(runner)
	installed, err := d.PackageCheck(Package{Name: "firefox", Cask: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Fatal("expected cask to be installed")
	}
}

func TestDarwinPackageCheck_CaskNotInstalled(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"list", "--cask", "firefox"}, err: fmt.Errorf("exit 1")},
	})
	d := NewDarwinAdapter(runner)
	installed, err := d.PackageCheck(Package{Name: "firefox", Cask: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatal("expected cask to not be installed")
	}
}

// --- Darwin Upgrade Tests ---

func TestDarwinPackageUpgrade_Formula(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"upgrade", "git"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageUpgrade(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinPackageUpgrade_Pinned(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageUpgrade(Package{Name: "git", Pinned: true})
	if !errors.Is(err, ErrPackagePinned) {
		t.Fatalf("expected ErrPackagePinned, got %v", err)
	}
}

func TestDarwinPackageUpgrade_Cask(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"upgrade", "--cask", "firefox"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageUpgrade(Package{Name: "firefox", Cask: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Darwin Remove Tests ---

func TestDarwinPackageRemove_Formula(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"uninstall", "git"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageRemove(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinPackageRemove_PinnedFormula(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"unpin", "git"}, out: ""},
		{name: "brew", args: []string{"uninstall", "git"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageRemove(Package{Name: "git", Pinned: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinPackageRemove_Cask(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"uninstall", "--cask", "firefox"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.PackageRemove(Package{Name: "firefox", Cask: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Darwin Defaults Tests ---

func TestDarwinDefaultsRead(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"read", "com.apple.dock", "autohide"}, out: "1\n"},
		{name: "defaults", args: []string{"read-type", "com.apple.dock", "autohide"}, out: "Type is boolean\n"},
	})
	d := NewDarwinAdapter(runner)
	val, err := d.DefaultsRead("com.apple.dock", "autohide")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Type != "boolean" {
		t.Fatalf("expected type boolean, got %s", val.Type)
	}
	if val.Raw != "1" {
		t.Fatalf("expected raw value 1, got %s", val.Raw)
	}
}

func TestDarwinDefaultsRead_NotExist(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"read", "com.example", "missing"}, err: fmt.Errorf("exit 1")},
	})
	d := NewDarwinAdapter(runner)
	_, err := d.DefaultsRead("com.example", "missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestDarwinDefaultsWrite_Boolean(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "com.apple.dock", "autohide", "-bool", "true"}, out: ""},
		{name: "defaults", args: []string{"read-type", "com.apple.dock", "autohide"}, out: "Type is boolean\n"},
		{name: "defaults", args: []string{"read", "com.apple.dock", "autohide"}, out: "1\n"},
		{name: "killall", args: []string{"Dock"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("com.apple.dock", "autohide", DefaultsValue{Type: "boolean", Raw: "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinDefaultsWrite_String(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "NSGlobalDomain", "AppleLanguages", "-string", "en"}, out: ""},
		{name: "defaults", args: []string{"read-type", "NSGlobalDomain", "AppleLanguages"}, out: "Type is string\n"},
		{name: "defaults", args: []string{"read", "NSGlobalDomain", "AppleLanguages"}, out: "en\n"},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("NSGlobalDomain", "AppleLanguages", DefaultsValue{Type: "string", Raw: "en"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinDefaultsWrite_Integer(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "com.apple.dock", "tilesize", "-int", "48"}, out: ""},
		{name: "defaults", args: []string{"read-type", "com.apple.dock", "tilesize"}, out: "Type is integer\n"},
		{name: "defaults", args: []string{"read", "com.apple.dock", "tilesize"}, out: "48\n"},
		{name: "killall", args: []string{"Dock"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("com.apple.dock", "tilesize", DefaultsValue{Type: "integer", Raw: "48"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinDefaultsWrite_TypeMismatch(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "com.apple.dock", "autohide", "-bool", "true"}, out: ""},
		{name: "defaults", args: []string{"read-type", "com.apple.dock", "autohide"}, out: "Type is string\n"},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("com.apple.dock", "autohide", DefaultsValue{Type: "boolean", Raw: "true"})
	if !errors.Is(err, ErrDefaultsTypeMismatch) {
		t.Fatalf("expected ErrDefaultsTypeMismatch, got %v", err)
	}
}

func TestDarwinDefaultsWrite_ValueMismatch(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "com.apple.dock", "tilesize", "-int", "48"}, out: ""},
		{name: "defaults", args: []string{"read-type", "com.apple.dock", "tilesize"}, out: "Type is integer\n"},
		{name: "defaults", args: []string{"read", "com.apple.dock", "tilesize"}, out: "36\n"},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("com.apple.dock", "tilesize", DefaultsValue{Type: "integer", Raw: "48"})
	if !errors.Is(err, ErrDefaultsValueMismatch) {
		t.Fatalf("expected ErrDefaultsValueMismatch, got %v", err)
	}
}

// --- Darwin Boolean Normalization Tests ---

func TestNormalizeBool(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"true", "1"},
		{"True", "1"},
		{"TRUE", "1"},
		{"1", "1"},
		{"false", "0"},
		{"False", "0"},
		{"FALSE", "0"},
		{"0", "0"},
		{"other", "other"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeBool(tt.input)
			if got != tt.want {
				t.Errorf("normalizeBool(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultsValuesEqual_Boolean(t *testing.T) {
	tests := []struct {
		declared string
		actual   string
		want     bool
	}{
		{"true", "1", true},
		{"false", "0", true},
		{"true", "0", false},
		{"false", "1", false},
		{"1", "1", true},
		{"0", "0", true},
	}
	for _, tt := range tests {
		t.Run(tt.declared+"_"+tt.actual, func(t *testing.T) {
			got := defaultsValuesEqual("boolean", tt.declared, tt.actual)
			if got != tt.want {
				t.Errorf("defaultsValuesEqual(boolean, %q, %q) = %v, want %v",
					tt.declared, tt.actual, got, tt.want)
			}
		})
	}
}

func TestDefaultsValuesEqual_String(t *testing.T) {
	if !defaultsValuesEqual("string", "hello", "hello") {
		t.Error("expected equal strings to be equal")
	}
	if defaultsValuesEqual("string", "hello", "world") {
		t.Error("expected different strings to not be equal")
	}
}

// --- Darwin Process Restart Registry Tests ---

func TestProcessRestartRegistry(t *testing.T) {
	tests := []struct {
		domain  string
		process string
		exists  bool
	}{
		{"com.apple.dock", "Dock", true},
		{"com.apple.finder", "Finder", true},
		{"com.apple.SystemUIServer", "SystemUIServer", true},
		{"NSGlobalDomain", "", false},
		{"com.example.app", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			proc, ok := processRestartRegistry[tt.domain]
			if ok != tt.exists {
				t.Errorf("processRestartRegistry[%q] exists = %v, want %v", tt.domain, ok, tt.exists)
			}
			if ok && proc != tt.process {
				t.Errorf("processRestartRegistry[%q] = %q, want %q", tt.domain, proc, tt.process)
			}
		})
	}
}

// --- Darwin parseDefaultsType Tests ---

func TestParseDefaultsType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Type is boolean\n", "boolean"},
		{"Type is string\n", "string"},
		{"Type is integer\n", "integer"},
		{"Type is float\n", "float"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := parseDefaultsType(tt.input)
			if got != tt.want {
				t.Errorf("parseDefaultsType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Darwin Hostname Tests ---

func TestDarwinSetHostname(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "sudo", args: []string{"scutil", "--set", "HostName", "myhost"}, out: ""},
		{name: "sudo", args: []string{"scutil", "--set", "LocalHostName", "myhost"}, out: ""},
		{name: "sudo", args: []string{"scutil", "--set", "ComputerName", "myhost"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.SetHostname("myhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDarwinSetHostname_Failure(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "sudo", args: []string{"scutil", "--set", "HostName", "myhost"}, err: fmt.Errorf("permission denied")},
	})
	d := NewDarwinAdapter(runner)
	err := d.SetHostname("myhost")
	if err == nil {
		t.Fatal("expected error on hostname failure")
	}
}

// --- Darwin PackageList Tests ---

func TestDarwinPackageList(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"leaves"}, out: "git\ncurl\nwget\n"},
	})
	d := NewDarwinAdapter(runner)
	pkgs, err := d.PackageList()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(pkgs))
	}
	if pkgs[0].Name != "git" {
		t.Errorf("expected first package git, got %s", pkgs[0].Name)
	}
}

// --- Darwin ListLeaves Tests ---

func TestDarwinListLeaves(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", args: []string{"brew"}, out: "/opt/homebrew/bin/brew"},
		{name: "brew", args: []string{"leaves"}, out: "git\ncurl\n"},
		{name: "brew", args: []string{"list", "--cask"}, out: "firefox\niterm2\n"},
	})
	d := NewDarwinAdapter(runner)
	leaves, err := d.ListLeaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leaves) != 4 {
		t.Fatalf("expected 4 leaves, got %d: %v", len(leaves), leaves)
	}
}

// --- Darwin Service Tests ---

func TestDarwinServiceEnable_NotSupported(t *testing.T) {
	d := NewDarwinAdapter(nil)
	err := d.ServiceEnable("some-service")
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestDarwinServiceDisable_NotSupported(t *testing.T) {
	d := NewDarwinAdapter(nil)
	err := d.ServiceDisable("some-service")
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

// --- formulaName Tests ---

func TestFormulaName(t *testing.T) {
	tests := []struct {
		pkg  Package
		want string
	}{
		{Package{Name: "git"}, "git"},
		{Package{Name: "node", Version: "20"}, "node@20"},
		{Package{Name: "python", Version: "3"}, "python@3"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formulaName(tt.pkg)
			if got != tt.want {
				t.Errorf("formulaName(%+v) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}

// --- Darwin Defaults Write with Finder restart ---

func TestDarwinDefaultsWrite_FinderRestart(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "com.apple.finder", "ShowPathbar", "-bool", "true"}, out: ""},
		{name: "defaults", args: []string{"read-type", "com.apple.finder", "ShowPathbar"}, out: "Type is boolean\n"},
		{name: "defaults", args: []string{"read", "com.apple.finder", "ShowPathbar"}, out: "1\n"},
		{name: "killall", args: []string{"Finder"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("com.apple.finder", "ShowPathbar", DefaultsValue{Type: "boolean", Raw: "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Darwin Defaults Write with no restart (NSGlobalDomain) ---

func TestDarwinDefaultsWrite_NoRestart(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "NSGlobalDomain", "key1", "-string", "val1"}, out: ""},
		{name: "defaults", args: []string{"read-type", "NSGlobalDomain", "key1"}, out: "Type is string\n"},
		{name: "defaults", args: []string{"read", "NSGlobalDomain", "key1"}, out: "val1\n"},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("NSGlobalDomain", "key1", DefaultsValue{Type: "string", Raw: "val1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Darwin Float Defaults ---

func TestDarwinDefaultsWrite_Float(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "defaults", args: []string{"write", "com.apple.dock", "magnification-size", "-float", "128.5"}, out: ""},
		{name: "defaults", args: []string{"read-type", "com.apple.dock", "magnification-size"}, out: "Type is float\n"},
		{name: "defaults", args: []string{"read", "com.apple.dock", "magnification-size"}, out: "128.5\n"},
		{name: "killall", args: []string{"Dock"}, out: ""},
	})
	d := NewDarwinAdapter(runner)
	err := d.DefaultsWrite("com.apple.dock", "magnification-size", DefaultsValue{Type: "float", Raw: "128.5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
