package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/adze/internal/dag"
)

func TestGraphTextFormat(t *testing.T) {
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
	root.SetArgs([]string{"graph", "--config", cfgPath, "--format", "text"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should contain step names
	if !strings.Contains(output, "xcode-cli-tools") {
		t.Errorf("expected xcode-cli-tools in text output, got:\n%s", output)
	}
	if !strings.Contains(output, "homebrew") {
		t.Errorf("expected homebrew in text output, got:\n%s", output)
	}
	if !strings.Contains(output, "brew-packages") {
		t.Errorf("expected brew-packages in text output, got:\n%s", output)
	}
}

func TestGraphDotFormat(t *testing.T) {
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
	root.SetArgs([]string{"graph", "--config", cfgPath, "--format", "dot"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "digraph {") {
		t.Errorf("expected DOT output to start with 'digraph {', got:\n%s", output)
	}
	if !strings.Contains(output, "->") {
		t.Errorf("expected edges in DOT output, got:\n%s", output)
	}
	if !strings.HasSuffix(strings.TrimSpace(output), "}") {
		t.Errorf("expected DOT output to end with '}', got:\n%s", output)
	}
}

func TestGraphInvalidFormat(t *testing.T) {
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
	root.SetArgs([]string{"graph", "--config", cfgPath, "--format", "invalid"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("expected 'invalid format' in error, got: %v", err)
	}
}

func TestGraphNoConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yaml")

	root := NewRootCmd("dev", "none", "unknown")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"graph", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

// Unit tests for the rendering functions directly

func TestRenderTextTreeSimple(t *testing.T) {
	graph := &dag.ResolvedGraph{
		Steps: []dag.ResolvedStep{
			{Name: "root", DependsOn: map[string]string{}, Depth: 0},
			{Name: "child-a", DependsOn: map[string]string{"root-cap": "root"}, Depth: 1},
			{Name: "child-b", DependsOn: map[string]string{"root-cap": "root"}, Depth: 1},
		},
	}

	output := renderTextTree(graph)

	if !strings.Contains(output, "root") {
		t.Errorf("expected 'root' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "child-a") {
		t.Errorf("expected 'child-a' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "child-b") {
		t.Errorf("expected 'child-b' in output, got:\n%s", output)
	}
}

func TestRenderTextTreeEmpty(t *testing.T) {
	graph := &dag.ResolvedGraph{Steps: []dag.ResolvedStep{}}
	output := renderTextTree(graph)
	if !strings.Contains(output, "no steps") {
		t.Errorf("expected 'no steps' for empty graph, got:\n%s", output)
	}
}

func TestRenderDotSimple(t *testing.T) {
	graph := &dag.ResolvedGraph{
		Steps: []dag.ResolvedStep{
			{Name: "a", DependsOn: map[string]string{}, Depth: 0},
			{Name: "b", DependsOn: map[string]string{"a-cap": "a"}, Depth: 1},
		},
	}

	output := renderDot(graph)

	if !strings.Contains(output, "digraph {") {
		t.Errorf("expected 'digraph {' in output, got:\n%s", output)
	}
	if !strings.Contains(output, `"a" -> "b"`) {
		t.Errorf("expected edge 'a -> b' in output, got:\n%s", output)
	}
}

func TestRenderDotIsolatedNode(t *testing.T) {
	graph := &dag.ResolvedGraph{
		Steps: []dag.ResolvedStep{
			{Name: "standalone", DependsOn: map[string]string{}, RequiredBy: map[string]string{}, Depth: 0},
		},
	}

	output := renderDot(graph)

	if !strings.Contains(output, `"standalone"`) {
		t.Errorf("expected isolated node in output, got:\n%s", output)
	}
}

func TestRenderTextTreeDeep(t *testing.T) {
	graph := &dag.ResolvedGraph{
		Steps: []dag.ResolvedStep{
			{Name: "a", DependsOn: map[string]string{}, Depth: 0},
			{Name: "b", DependsOn: map[string]string{"a-cap": "a"}, Depth: 1},
			{Name: "c", DependsOn: map[string]string{"b-cap": "b"}, Depth: 2},
		},
	}

	output := renderTextTree(graph)

	// Verify the tree structure shows nesting
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d:\n%s", len(lines), output)
	}
	if lines[0] != "a" {
		t.Errorf("expected first line to be 'a', got %q", lines[0])
	}
}
