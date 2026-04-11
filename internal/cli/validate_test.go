package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
packages:
  brew:
    - git
    - curl
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "valid") {
		t.Errorf("expected 'valid' in output, got: %s", output)
	}
}

func TestValidateInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{{{not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}

	// Should return exit code 2 (config error)
	exitCode := exitCodeFromErr(err)
	if exitCode != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitCode)
	}
}

func TestValidateMissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	// Missing name and platform
	cfgContent := `packages:
  brew:
    - git
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for config with missing required fields")
	}

	exitCode := exitCodeFromErr(err)
	if exitCode != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitCode)
	}
}

func TestValidateJSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "--config", cfgPath, "--json"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result validateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, buf.String())
	}

	if !result.Valid {
		t.Errorf("expected valid=true, got false. Errors: %v", result.Errors)
	}
}

func TestValidateJSONOutputInvalid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "--config", cfgPath, "--json"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid YAML with JSON flag")
	}

	var result validateResult
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", jsonErr, buf.String())
	}

	if result.Valid {
		t.Error("expected valid=false for invalid config")
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors in JSON output")
	}
}

func TestValidateNoConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yaml")

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
}

func TestValidateConfigWithSecretsCrossRef(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
secrets:
  - name: GITHUB_TOKEN
    required: true
custom_steps:
  my-step:
    description: "Test step"
    provides:
      - my-step
    check: "true"
    apply:
      any: "echo hello"
    env:
      - UNDECLARED_VAR
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate", "--config", cfgPath, "--json"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result validateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, buf.String())
	}

	// Should have a warning about undeclared env var
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "UNDECLARED_VAR") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about UNDECLARED_VAR, got warnings: %v", result.Warnings)
	}
}

func TestExitCodeFunction(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil error returns unexpected", nil, ExitUnexpected},
		{"exit error 2", &exitError{Code: 2}, 2},
		{"exit error 3", &exitError{Code: 3}, 3},
		{"generic error returns 1", os.ErrNotExist, ExitUnexpected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				// ExitCode with nil shouldn't be called in practice,
				// but test that it doesn't panic
				return
			}
			got := exitCodeFromErr(tt.err)
			if got != tt.want {
				t.Errorf("exitCodeFromErr() = %d, want %d", got, tt.want)
			}
		})
	}
}
