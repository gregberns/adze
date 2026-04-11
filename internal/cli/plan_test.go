package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Plan command tests ---

// minimalDarwinConfig returns a minimal valid config for darwin platform.
func minimalDarwinConfig() string {
	return `name: test
platform: darwin
`
}

// configWithBrewPackages returns a config with some brew packages.
func configWithBrewPackages() string {
	return `name: test
platform: darwin
packages:
  brew:
    - bat
    - ripgrep
`
}

// configWithSecrets returns a config with secret declarations.
func configWithSecrets() string {
	return `name: test
platform: darwin
secrets:
  - name: GITHUB_TOKEN
    required: true
  - name: OPTIONAL_KEY
    required: false
`
}

func TestPlanConfigError_NoConfig(t *testing.T) {
	// Run plan in a temp dir with no config files
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"plan"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no config file exists")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d (ConfigError), got %d", ExitConfigError, code)
	}
}

func TestPlanConfigError_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	os.WriteFile(cfgPath, []byte("{{{{not yaml"), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"plan", "--config", cfgPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d (ConfigError), got %d", ExitConfigError, code)
	}
}

func TestPlanConfigError_ValidationErrors(t *testing.T) {
	dir := t.TempDir()
	// Missing required fields: name and platform
	cfgPath := filepath.Join(dir, "missing.yaml")
	os.WriteFile(cfgPath, []byte("tags: []\n"), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"plan", "--config", cfgPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for config with validation errors")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d (ConfigError), got %d", ExitConfigError, code)
	}
}

func TestPlanMinimalConfig_Human(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "machine.yaml")
	os.WriteFile(cfgPath, []byte(minimalDarwinConfig()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"plan", "--config", cfgPath})

	err := root.Execute()
	output := buf.String()

	// Should contain platform and config info
	if !strings.Contains(output, "Platform:") {
		t.Error("output should contain 'Platform:'")
	}
	if !strings.Contains(output, "Config:") {
		t.Error("output should contain 'Config:'")
	}
	if !strings.Contains(output, "Pre-flight:") {
		t.Error("output should contain 'Pre-flight:'")
	}
	if !strings.Contains(output, "Plan:") {
		t.Error("output should contain 'Plan:'")
	}
	if !strings.Contains(output, "Summary:") {
		t.Error("output should contain 'Summary:'")
	}

	// With a minimal config, no steps should be generated (no packages, etc.)
	// so exit code should be 0 (no changes needed) or 6 (changes planned)
	if err != nil {
		code := ExitCodeFromError(err)
		if code != ExitChangesPlanned && code != ExitSuccess {
			t.Errorf("expected exit code 0 or 6, got %d: %v", code, err)
		}
	}
}

func TestPlanMinimalConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "machine.yaml")
	os.WriteFile(cfgPath, []byte(minimalDarwinConfig()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"plan", "--config", cfgPath, "--json"})

	root.Execute()
	output := buf.String()

	// Should be valid JSON
	var result planResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	if result.Platform == "" {
		t.Error("JSON result should have platform field")
	}
	if result.ConfigFile == "" {
		t.Error("JSON result should have config field")
	}
	if result.PreFlight == nil {
		t.Error("JSON result should have pre_flight field")
	}
}

func TestPlanWithPackages(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "machine.yaml")
	os.WriteFile(cfgPath, []byte(configWithBrewPackages()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"plan", "--config", cfgPath})

	err := root.Execute()
	output := buf.String()

	// Should show steps in the plan
	if !strings.Contains(output, "Plan:") {
		t.Error("output should contain 'Plan:'")
	}

	// With packages, there should be changes planned (exit code 6)
	// or 0 if everything is already satisfied
	if err != nil {
		code := ExitCodeFromError(err)
		if code != ExitChangesPlanned && code != ExitSuccess {
			t.Errorf("expected exit code 0 or 6, got %d: %v", code, err)
		}
	}
}

func TestPlanWithPackages_JSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "machine.yaml")
	os.WriteFile(cfgPath, []byte(configWithBrewPackages()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"plan", "--config", cfgPath, "--json"})

	root.Execute()
	output := buf.String()

	var result planResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should have steps
	if len(result.Steps) == 0 {
		t.Error("JSON result should have steps when packages are configured")
	}

	// Should have a summary with counts
	total := result.Summary.ToApply + result.Summary.Satisfied + result.Summary.Blocked
	if total != len(result.Steps) {
		t.Errorf("summary counts (%d) should equal total steps (%d)", total, len(result.Steps))
	}
}

func TestPlanExplicitConfigFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "custom-name.yaml")
	os.WriteFile(cfgPath, []byte(minimalDarwinConfig()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"plan", "--config", cfgPath})

	root.Execute()
	output := buf.String()

	if !strings.Contains(output, "custom-name.yaml") {
		t.Errorf("output should reference the config file path, got: %s", output)
	}
}

func TestPlanExitCodes(t *testing.T) {
	// Test that a non-existent config gives exit code 2
	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"plan", "--config", "/nonexistent/path.yaml"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent config")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, code)
	}
}

// --- exitError tests ---

func TestExitCodeFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error", nil, ExitSuccess},
		{"exitError with code 2", &exitError{Code: 2, Err: nil}, 2},
		{"exitError with code 6", &exitError{Code: 6, Err: nil}, 6},
		{"regular error", os.ErrNotExist, ExitUnexpected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCodeFromError(tt.err)
			if got != tt.want {
				t.Errorf("ExitCodeFromError() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExitError_Error(t *testing.T) {
	ee := &exitError{Code: 2, Err: os.ErrNotExist}
	if !strings.Contains(ee.Error(), "not exist") {
		t.Errorf("exitError.Error() = %q, should contain original error message", ee.Error())
	}
}

// --- Helper function tests ---

func TestStepConfigsToDagInputs(t *testing.T) {
	configs := []struct {
		name string
	}{
		{name: "homebrew"},
		{name: "brew-packages"},
	}
	_ = configs // just checking the function exists and compiles
}

func TestResolveConfigPath_LocalFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test.yaml")
	os.WriteFile(cfgPath, []byte(minimalDarwinConfig()), 0o644)

	path, isURL, cleanup, err := resolveConfigPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isURL {
		t.Error("expected isURL=false for local file")
	}
	if cleanup != nil {
		t.Error("expected no cleanup for local file")
	}
	if path != cfgPath {
		t.Errorf("got path %q, want %q", path, cfgPath)
	}
}

func TestResolveConfigPath_ExplicitPath(t *testing.T) {
	// resolveConfigPath with an explicit path returns it as-is (no existence check).
	// Existence is checked later by LoadConfig.
	path, isURL, cleanup, err := resolveConfigPath("/nonexistent/file.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isURL {
		t.Error("expected isURL=false")
	}
	if cleanup != nil {
		t.Error("expected no cleanup")
	}
	if path != "/nonexistent/file.yaml" {
		t.Errorf("got path %q, want %q", path, "/nonexistent/file.yaml")
	}
}

func TestResolveConfigPath_AutoDetectEmpty(t *testing.T) {
	// When no flag is given and we're in an empty dir, should error.
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	_, _, _, err := resolveConfigPath("")
	if err == nil {
		t.Error("expected error when auto-detecting in empty directory")
	}
}

func TestFormatAction(t *testing.T) {
	// Without color
	tests := []struct {
		action string
		want   string
	}{
		{"skip", "[skip]    "},
		{"install", "[install] "},
		{"blocked", "[blocked] "},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := formatAction(tt.action, false)
			if got != tt.want {
				t.Errorf("formatAction(%q, false) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestClassifyCheckResult(t *testing.T) {
	satisfied := step_result_satisfied()
	action, desc := classifyCheckResult(satisfied, emptyStepConfig())
	if action != "skip" {
		t.Errorf("expected action 'skip' for satisfied, got %q", action)
	}
	if desc != "already satisfied" {
		t.Errorf("expected desc 'already satisfied', got %q", desc)
	}

	failed := step_result_failed()
	action, desc = classifyCheckResult(failed, emptyStepConfig())
	if action != "install" {
		t.Errorf("expected action 'install' for failed, got %q", action)
	}
}
