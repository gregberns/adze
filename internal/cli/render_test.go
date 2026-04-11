package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderCommandStdout(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
packages:
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
	root.SetArgs([]string{"render", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should be a valid bash script
	if !strings.HasPrefix(output, "#!/bin/bash\n") {
		t.Error("render output should start with shebang")
	}
	if !strings.Contains(output, "set -euo pipefail") {
		t.Error("render output should contain set -euo pipefail")
	}
}

func TestRenderCommandOutputFile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(dir, "setup.sh")

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"render", "--config", cfgPath, "--output", outputPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check file was created
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}

	// Check file is executable
	if info.Mode()&0100 == 0 {
		t.Error("output file should be executable")
	}

	// Check contents
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(string(data), "#!/bin/bash\n") {
		t.Error("output file should start with shebang")
	}

	// Check stdout message
	if !strings.Contains(buf.String(), "Wrote "+outputPath) {
		t.Errorf("expected 'Wrote' message in stdout, got: %s", buf.String())
	}
}

func TestRenderCommandInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"render", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestRenderCommandNoConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yaml")

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"render", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}
