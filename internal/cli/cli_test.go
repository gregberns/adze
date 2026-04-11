package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- Exit code constants ----

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"Success", ExitSuccess, 0},
		{"Unexpected", ExitUnexpected, 1},
		{"ConfigError", ExitConfigError, 2},
		{"PreFlightFail", ExitPreFlightFail, 3},
		{"ExecFailure", ExitExecFailure, 4},
		{"PartialSuccess", ExitPartialSuccess, 5},
		{"ChangesPlanned", ExitChangesPlanned, 6},
		{"DriftDetected", ExitDriftDetected, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("Exit%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

// ---- Color detection ----

func TestColorEnabled(t *testing.T) {
	tests := []struct {
		name        string
		noColorFlag bool
		env         map[string]string
		isTTY       bool
		want        bool
	}{
		{
			name:  "all conditions met",
			env:   map[string]string{},
			isTTY: true,
			want:  true,
		},
		{
			name:        "no-color flag disables",
			noColorFlag: true,
			env:         map[string]string{},
			isTTY:       true,
			want:        false,
		},
		{
			name:  "NO_COLOR env disables",
			env:   map[string]string{"NO_COLOR": ""},
			isTTY: true,
			want:  false,
		},
		{
			name:  "TERM=dumb disables",
			env:   map[string]string{"TERM": "dumb"},
			isTTY: true,
			want:  false,
		},
		{
			name:  "non-TTY disables",
			env:   map[string]string{},
			isTTY: false,
			want:  false,
		},
		{
			name:  "CI env disables",
			env:   map[string]string{"CI": "true"},
			isTTY: true,
			want:  false,
		},
		{
			name:        "FORCE_COLOR overrides no-color flag",
			noColorFlag: true,
			env:         map[string]string{"FORCE_COLOR": "1"},
			isTTY:       false,
			want:        true,
		},
		{
			name:  "FORCE_COLOR overrides NO_COLOR",
			env:   map[string]string{"FORCE_COLOR": "1", "NO_COLOR": "1"},
			isTTY: false,
			want:  true,
		},
		{
			name:  "FORCE_COLOR overrides CI",
			env:   map[string]string{"FORCE_COLOR": "1", "CI": "true"},
			isTTY: false,
			want:  true,
		},
		{
			name:  "FORCE_COLOR overrides TERM dumb",
			env:   map[string]string{"FORCE_COLOR": "1", "TERM": "dumb"},
			isTTY: false,
			want:  true,
		},
		{
			name:  "TERM xterm allows color",
			env:   map[string]string{"TERM": "xterm-256color"},
			isTTY: true,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				val, ok := tt.env[key]
				return val, ok
			}
			got := colorEnabledWithEnv(tt.noColorFlag, lookup, tt.isTTY)
			if got != tt.want {
				t.Errorf("colorEnabledWithEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---- Output mode detection ----

func TestDetectOutputMode(t *testing.T) {
	if DetectOutputMode(true) != OutputJSON {
		t.Error("expected OutputJSON when jsonFlag=true")
	}
	if DetectOutputMode(false) != OutputHuman {
		t.Error("expected OutputHuman when jsonFlag=false")
	}
}

func TestOutputModeConstants(t *testing.T) {
	if OutputHuman != 0 {
		t.Errorf("OutputHuman = %d, want 0", OutputHuman)
	}
	if OutputJSON != 1 {
		t.Errorf("OutputJSON = %d, want 1", OutputJSON)
	}
}

// ---- Version command ----

func TestVersionCommand(t *testing.T) {
	root := NewRootCmd("1.2.3", "abc1234", "2024-01-15")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	expected := "adze 1.2.3 (commit: abc1234, built: 2024-01-15)\n"
	if output != expected {
		t.Errorf("version output = %q, want %q", output, expected)
	}
}

func TestVersionCommandDevDefaults(t *testing.T) {
	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "dev") {
		t.Error("expected 'dev' in version output")
	}
	if !strings.Contains(output, "none") {
		t.Error("expected 'none' in version output")
	}
}

// ---- Root command / help ----

func TestRootCommandHelp(t *testing.T) {
	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()

	// All expected subcommands should appear in help.
	expectedCmds := []string{
		"init", "plan", "apply", "status", "capture",
		"install", "remove", "upgrade", "validate",
		"graph", "render", "doctor", "version", "step",
	}
	for _, cmd := range expectedCmds {
		if !strings.Contains(output, cmd) {
			t.Errorf("help output missing command %q", cmd)
		}
	}

	// Global flags should appear.
	expectedFlags := []string{"--config", "--json", "--verbose", "--quiet", "--no-color"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("help output missing flag %q", flag)
		}
	}
}

func TestStepSubcommands(t *testing.T) {
	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"step", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("step help failed: %v", err)
	}

	output := buf.String()
	for _, sub := range []string{"list", "info", "add"} {
		if !strings.Contains(output, sub) {
			t.Errorf("step help missing subcommand %q", sub)
		}
	}
}

// ---- Stub commands return errors ----

// All commands are now fully implemented — no stubs remain.

// ---- Config auto-detection ----

func TestResolveConfigExplicit(t *testing.T) {
	path, err := ResolveConfig("/some/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/some/path/config.yaml" {
		t.Errorf("got %q, want %q", path, "/some/path/config.yaml")
	}
}

func TestAutoDetectConfigNoFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err == nil {
		t.Fatal("expected error when no config files exist")
	}
	if !strings.Contains(err.Error(), "no adze config file found") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "adze init") {
		t.Errorf("error should mention 'adze init': %v", err)
	}
}

func TestAutoDetectConfigSingleMatch(t *testing.T) {
	dir := t.TempDir()

	// Write a valid adze config with platform: darwin.
	cfg := `name: test
platform: darwin
`
	err := os.WriteFile(filepath.Join(dir, "machine.yaml"), []byte(cfg), 0644)
	if err != nil {
		t.Fatal(err)
	}

	path, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "machine.yaml" {
		t.Errorf("got %q, want machine.yaml", filepath.Base(path))
	}
}

func TestAutoDetectConfigMultipleMatches(t *testing.T) {
	dir := t.TempDir()

	cfg := `name: test
platform: darwin
`
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(cfg), 0644)
	os.WriteFile(filepath.Join(dir, "b.yml"), []byte(cfg), 0644)

	_, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err == nil {
		t.Fatal("expected error for multiple matches")
	}
	if !strings.Contains(err.Error(), "multiple adze config files found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAutoDetectConfigPlatformFiltering(t *testing.T) {
	dir := t.TempDir()

	darwinCfg := `name: mac
platform: darwin
`
	ubuntuCfg := `name: linux
platform: ubuntu
`
	os.WriteFile(filepath.Join(dir, "mac.yaml"), []byte(darwinCfg), 0644)
	os.WriteFile(filepath.Join(dir, "linux.yaml"), []byte(ubuntuCfg), 0644)

	// On darwin, should pick only the darwin config.
	path, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "mac.yaml" {
		t.Errorf("got %q, want mac.yaml", filepath.Base(path))
	}

	// On ubuntu, should pick only the ubuntu config.
	path, err = autoDetectConfigWithPlatform(dir, "ubuntu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "linux.yaml" {
		t.Errorf("got %q, want linux.yaml", filepath.Base(path))
	}
}

func TestAutoDetectConfigPlatformAny(t *testing.T) {
	dir := t.TempDir()

	cfg := `name: universal
platform: any
`
	os.WriteFile(filepath.Join(dir, "universal.yaml"), []byte(cfg), 0644)

	// "any" should match any platform.
	path, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "universal.yaml" {
		t.Errorf("got %q, want universal.yaml", filepath.Base(path))
	}
}

func TestAutoDetectConfigIgnoresNonAdzeYAML(t *testing.T) {
	dir := t.TempDir()

	// A YAML file without a platform field is not an adze config.
	nonAdze := `name: something
version: "1.0"
`
	adze := `name: real
platform: darwin
`
	os.WriteFile(filepath.Join(dir, "other.yaml"), []byte(nonAdze), 0644)
	os.WriteFile(filepath.Join(dir, "machine.yaml"), []byte(adze), 0644)

	path, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "machine.yaml" {
		t.Errorf("got %q, want machine.yaml", filepath.Base(path))
	}
}

func TestAutoDetectConfigIgnoresInvalidYAML(t *testing.T) {
	dir := t.TempDir()

	// Invalid YAML should be skipped.
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("{{{{not yaml"), 0644)

	adze := `name: good
platform: darwin
`
	os.WriteFile(filepath.Join(dir, "good.yaml"), []byte(adze), 0644)

	path, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "good.yaml" {
		t.Errorf("got %q, want good.yaml", filepath.Base(path))
	}
}

func TestAutoDetectConfigIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a directory with .yaml extension (unusual but possible).
	os.Mkdir(filepath.Join(dir, "subdir.yaml"), 0755)

	adze := `name: config
platform: darwin
`
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(adze), 0644)

	path, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("got %q, want config.yaml", filepath.Base(path))
	}
}

func TestAutoDetectConfigYmlExtension(t *testing.T) {
	dir := t.TempDir()

	cfg := `name: test
platform: darwin
`
	os.WriteFile(filepath.Join(dir, "config.yml"), []byte(cfg), 0644)

	path, err := autoDetectConfigWithPlatform(dir, "darwin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "config.yml" {
		t.Errorf("got %q, want config.yml", filepath.Base(path))
	}
}

// ---- Global flags ----

func TestGlobalFlagsRegistered(t *testing.T) {
	root := NewRootCmd("dev", "none", "unknown")

	flags := []string{"config", "json", "verbose", "quiet", "no-color"}
	for _, name := range flags {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("global flag %q not registered", name)
		}
	}
}

func TestGlobalFlagShortcuts(t *testing.T) {
	root := NewRootCmd("dev", "none", "unknown")

	shortcuts := map[string]string{
		"config":  "c",
		"verbose": "v",
		"quiet":   "q",
	}
	for name, short := range shortcuts {
		f := root.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("flag %q not found", name)
			continue
		}
		if f.Shorthand != short {
			t.Errorf("flag %q shorthand = %q, want %q", name, f.Shorthand, short)
		}
	}
}

// ---- Command-specific flags ----

func TestCommandSpecificFlags(t *testing.T) {
	root := NewRootCmd("dev", "none", "unknown")

	tests := []struct {
		cmdPath []string
		flag    string
	}{
		{[]string{"apply"}, "yes"},
		{[]string{"capture"}, "all"},
		{[]string{"install"}, "cask"},
		{[]string{"upgrade"}, "all"},
		{[]string{"graph"}, "format"},
		{[]string{"render"}, "output"},
		{[]string{"step", "list"}, "platform"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.cmdPath, "_")+"_"+tt.flag, func(t *testing.T) {
			cmd, _, err := root.Find(tt.cmdPath)
			if err != nil {
				t.Fatalf("command %v not found: %v", tt.cmdPath, err)
			}
			if cmd.Flags().Lookup(tt.flag) == nil {
				t.Errorf("flag %q not found on command %v", tt.flag, tt.cmdPath)
			}
		})
	}
}
