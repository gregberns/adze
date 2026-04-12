package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorSectionOrder(t *testing.T) {
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

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// Verify the 6 sections appear in the correct order per spec.
	sections := []string{
		"=== Config ===",
		"=== Dependency Graph ===",
		"=== Validation Results ===",
		"=== Platform Information ===",
		"=== Available Steps Not In Use ===",
		"=== Review Questions ===",
	}

	prevIdx := -1
	for _, section := range sections {
		idx := strings.Index(output, section)
		if idx == -1 {
			t.Errorf("missing section %q in output", section)
			continue
		}
		if idx <= prevIdx {
			t.Errorf("section %q appears out of order (at %d, previous section at %d)", section, idx, prevIdx)
		}
		prevIdx = idx
	}
}

func TestDoctorExactly6Sections(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()
	headerCount := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "=== ") && strings.HasSuffix(line, " ===") {
			headerCount++
		}
	}

	if headerCount != 6 {
		t.Errorf("expected exactly 6 section headers, got %d", headerCount)
	}
}

func TestDoctorNoOldSections(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// These old section names should NOT appear.
	oldSections := []string{
		"=== Platform ===",
		"=== Validation ===",
		"=== Step Inventory ===",
		"=== Unused Steps ===",
		"=== Pre-flight ===",
	}
	for _, s := range oldSections {
		if strings.Contains(output, s) {
			t.Errorf("old section %q should not appear in output", s)
		}
	}
}

func TestDoctorConfigSectionShowsYAML(t *testing.T) {
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

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// The Config section should contain the full YAML, including package names.
	// PackageEntry structs marshal with a "name:" field.
	if !strings.Contains(output, "name: test-machine") {
		t.Error("expected config YAML to contain 'name: test-machine'")
	}
	if !strings.Contains(output, "git") {
		t.Error("expected config YAML to contain 'git'")
	}
	if !strings.Contains(output, "curl") {
		t.Error("expected config YAML to contain 'curl'")
	}
}

func TestDoctorDependencyGraphFormat(t *testing.T) {
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

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// Each step in the dependency graph should have the format:
	// N. step-name [provides: ...] [requires: ...]
	if !strings.Contains(output, "[provides:") {
		t.Error("dependency graph should contain '[provides:' format")
	}
	if !strings.Contains(output, "[requires:") {
		t.Error("dependency graph should contain '[requires:' format")
	}
	// Should have numbered entries starting with "1."
	if !strings.Contains(output, "1. ") {
		t.Error("dependency graph should have numbered entries starting with '1.'")
	}
}

func TestDoctorPlatformInformation(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("1.2.3", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// Platform section should contain OS info and tool version.
	if !strings.Contains(output, "OS:") {
		t.Error("platform section should contain 'OS:'")
	}
	if !strings.Contains(output, "Tool version: 1.2.3") {
		t.Error("platform section should contain tool version")
	}
	if !strings.Contains(output, "Shell:") {
		t.Error("platform section should contain 'Shell:'")
	}
}

func TestDoctorFixedReviewQuestions(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// The spec defines exactly these 6 review questions.
	expectedQuestions := []string{
		"Are custom step dependencies declared correctly?",
		"Are there built-in steps that should replace custom steps?",
		"For platform-specific steps, suggest cross-platform equivalents.",
		"Review defaults settings for current OS version compatibility.",
		"Identify common tools/configs that might be missing.",
		"Check for deprecated packages or formulas.",
	}

	for i, q := range expectedQuestions {
		if !strings.Contains(output, q) {
			t.Errorf("missing review question %d: %q", i+1, q)
		}
	}

	// Verify they are numbered 1-6.
	for i, q := range expectedQuestions {
		numbered := fmt.Sprintf("%d. %s", i+1, q)
		if !strings.Contains(output, numbered) {
			t.Errorf("review question %d should be numbered: %q", i+1, numbered)
		}
	}
}

func TestDoctorNoConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yaml")

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor should not return an error even with missing config: %v", err)
	}

	output := buf.String()

	// Should still have all 6 sections.
	if !strings.Contains(output, "=== Config ===") {
		t.Error("missing Config section")
	}
	if !strings.Contains(output, "not found") {
		t.Error("Config section should indicate config not found")
	}
	if !strings.Contains(output, "=== Review Questions ===") {
		t.Error("missing Review Questions section")
	}
}

func TestDoctorAvailableStepsNotInUse(t *testing.T) {
	dir := t.TempDir()
	// Minimal config with no packages — many built-in steps should be unused.
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// With an empty config, there should be unused steps listed.
	section := extractSection(output, "=== Available Steps Not In Use ===")
	if section == "" {
		t.Fatal("could not extract Available Steps Not In Use section")
	}

	// The section should contain step entries with the em-dash format.
	if !strings.Contains(section, "\u2014") {
		t.Error("unused step entries should use em-dash separator")
	}
}

func TestDoctorValidationResults(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `name: test-machine
platform: darwin
`
	cfgPath := filepath.Join(dir, "machine.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd("1.0.0", "abc", "now")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor", "--config", cfgPath})

	err := root.Execute()
	if err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	output := buf.String()

	// The Validation Results section should exist.
	if !strings.Contains(output, "=== Validation Results ===") {
		t.Error("missing Validation Results section")
	}
}

// extractSection returns the content between the given section header and the next section header.
func extractSection(output, header string) string {
	idx := strings.Index(output, header)
	if idx == -1 {
		return ""
	}
	start := idx + len(header)
	rest := output[start:]
	nextSection := strings.Index(rest, "=== ")
	if nextSection == -1 {
		return rest
	}
	return rest[:nextSection]
}
