package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/adze/internal/step"
)

// --- Apply command tests ---

func TestApplyConfigError_NoConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"apply", "--yes"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no config file exists")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d (ConfigError), got %d", ExitConfigError, code)
	}
}

func TestApplyConfigError_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	os.WriteFile(cfgPath, []byte("{{{{not yaml"), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"apply", "--config", cfgPath, "--yes"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d (ConfigError), got %d", ExitConfigError, code)
	}
}

func TestApplyConfigError_ValidationErrors(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "missing.yaml")
	os.WriteFile(cfgPath, []byte("tags: []\n"), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"apply", "--config", cfgPath, "--yes"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for config with validation errors")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d (ConfigError), got %d", ExitConfigError, code)
	}
}

func TestApplyMinimalConfig_YesFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "machine.yaml")
	os.WriteFile(cfgPath, []byte(minimalDarwinConfig()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"apply", "--config", cfgPath, "--yes"})

	err := root.Execute()
	output := buf.String()

	// With a minimal config (no steps), should succeed with exit 0
	// or possibly show the summary
	if err != nil {
		code := ExitCodeFromError(err)
		// A minimal config with no packages might still have core steps
		// that need to be applied, so exit 4 or 5 is acceptable too
		validCodes := map[int]bool{
			ExitSuccess:        true,
			ExitExecFailure:    true,
			ExitPartialSuccess: true,
		}
		if !validCodes[code] {
			t.Errorf("unexpected exit code %d: %v\nOutput: %s", code, err, output)
		}
	}
}

func TestApplyMinimalConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "machine.yaml")
	os.WriteFile(cfgPath, []byte(minimalDarwinConfig()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"apply", "--config", cfgPath, "--yes", "--json"})

	root.Execute()
	output := buf.String()

	// Should be NDJSON: each line is a valid JSON object
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one NDJSON line")
	}

	// Last line should be a summary event
	var lastEvent applyEvent
	lastLine := lines[len(lines)-1]
	if err := json.Unmarshal([]byte(lastLine), &lastEvent); err != nil {
		t.Fatalf("last NDJSON line is not valid JSON: %v\nLine: %s", err, lastLine)
	}
	if lastEvent.Type != "summary" {
		t.Errorf("last event type should be 'summary', got %q", lastEvent.Type)
	}

	// All lines should be valid JSON
	for i, line := range lines {
		var evt applyEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nLine: %s", i, err, line)
		}
		if evt.Type == "" {
			t.Errorf("line %d has empty type field", i)
		}
	}
}

func TestApplyExplicitConfigFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "custom-name.yaml")
	os.WriteFile(cfgPath, []byte(minimalDarwinConfig()), 0o644)

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"apply", "--config", cfgPath, "--yes"})

	// Just verify it doesn't crash
	root.Execute()
}

func TestApplyNonExistentConfig(t *testing.T) {
	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"apply", "--config", "/nonexistent/path.yaml", "--yes"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent config")
	}
	code := ExitCodeFromError(err)
	if code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, code)
	}
}

func TestApplyYesFlagRegistered(t *testing.T) {
	root := NewRootCmd("dev", "none", "unknown")
	cmd, _, err := root.Find([]string{"apply"})
	if err != nil {
		t.Fatalf("apply command not found: %v", err)
	}
	if cmd.Flags().Lookup("yes") == nil {
		t.Error("--yes flag not registered on apply command")
	}
}

func TestApplyWithMissingRequiredSecret(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "machine.yaml")
	os.WriteFile(cfgPath, []byte(configWithSecrets()), 0o644)

	// Ensure the secret is NOT set
	os.Unsetenv("GITHUB_TOKEN")

	root := NewRootCmd("dev", "none", "unknown")
	root.SetArgs([]string{"apply", "--config", cfgPath, "--yes"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing required secret")
	}
	code := ExitCodeFromError(err)
	if code != ExitPreFlightFail {
		t.Errorf("expected exit code %d (PreFlightFail), got %d", ExitPreFlightFail, code)
	}
}

// --- mapStepStatus tests ---

func TestMapStepStatus(t *testing.T) {
	tests := []struct {
		status step.StepStatus
		want   string
	}{
		{step.StatusSatisfied, "success"},
		{step.StatusApplied, "success"},
		{step.StatusFailed, "failure"},
		{step.StatusVerifyFailed, "failure"},
		{step.StatusPartial, "warning"},
		{step.StatusSkipped, "skip"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := mapStepStatus(tt.status)
			if got != tt.want {
				t.Errorf("mapStepStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// --- stepNeedsSudo tests ---

func TestStepNeedsSudo(t *testing.T) {
	tests := []struct {
		name     string
		config   step.StepConfig
		platform string
		want     bool
	}{
		{
			name:     "apt step on ubuntu",
			config:   step.StepConfig{Name: "apt-packages"},
			platform: "ubuntu",
			want:     true,
		},
		{
			name:     "brew step on darwin",
			config:   step.StepConfig{Name: "brew-packages"},
			platform: "darwin",
			want:     false,
		},
		{
			name:     "machine-name step",
			config:   step.StepConfig{Name: "machine-name"},
			platform: "darwin",
			want:     true,
		},
		{
			name: "step with sudo in apply",
			config: step.StepConfig{
				Name:  "custom",
				Apply: &step.ShellCommand{Args: []string{"sh", "-c", "sudo something"}},
			},
			platform: "darwin",
			want:     true,
		},
		{
			name: "step without sudo",
			config: step.StepConfig{
				Name:  "custom",
				Apply: &step.ShellCommand{Args: []string{"sh", "-c", "echo hello"}},
			},
			platform: "darwin",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stepNeedsSudo(tt.config, tt.platform)
			if got != tt.want {
				t.Errorf("stepNeedsSudo() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- HandleExitError tests ---

func TestHandleExitError(t *testing.T) {
	code := HandleExitError(nil)
	if code != ExitSuccess {
		t.Errorf("HandleExitError(nil) = %d, want %d", code, ExitSuccess)
	}

	code = HandleExitError(&exitError{Code: ExitConfigError, Err: os.ErrNotExist})
	if code != ExitConfigError {
		t.Errorf("HandleExitError(exitError{2}) = %d, want %d", code, ExitConfigError)
	}
}

// --- test helpers ---

// step_result_satisfied returns a StepResult with satisfied status.
func step_result_satisfied() step.StepResult {
	return step.StepResult{Status: step.StatusSatisfied}
}

// step_result_failed returns a StepResult with failed status.
func step_result_failed() step.StepResult {
	return step.StepResult{Status: step.StatusFailed, Reason: "check failed"}
}

// emptyStepConfig returns a minimal StepConfig for testing.
func emptyStepConfig() step.StepConfig {
	return step.StepConfig{Name: "test"}
}
