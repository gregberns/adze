package step

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestRunCommand_StreamWriter verifies that when a stream writer is attached
// to the context, RunCommand tees subprocess stdout+stderr to it while still
// populating ExecResult.Stdout/Stderr.
func TestRunCommand_StreamWriter(t *testing.T) {
	var streamed bytes.Buffer
	ctx := ContextWithStreamWriter(context.Background(), &streamed)

	cmd := &ShellCommand{Args: []string{"sh", "-c", "echo hello; echo err >&2; exit 1"}}
	result, err := RunCommand(ctx, cmd, nil, "test", "apply")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("ExecResult.Stdout missing 'hello': %q", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "err") {
		t.Errorf("ExecResult.Stderr missing 'err': %q", result.Stderr)
	}
	streamedStr := streamed.String()
	if !strings.Contains(streamedStr, "hello") {
		t.Errorf("stream writer missing 'hello': %q", streamedStr)
	}
	if !strings.Contains(streamedStr, "err") {
		t.Errorf("stream writer missing 'err': %q", streamedStr)
	}
}

// TestRunCommand_NoStreamWriter verifies the existing capture-only behavior
// when no stream writer is on the context.
func TestRunCommand_NoStreamWriter(t *testing.T) {
	ctx := context.Background()
	cmd := &ShellCommand{Args: []string{"sh", "-c", "echo hello"}}
	result, err := RunCommand(ctx, cmd, nil, "test", "apply")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("ExecResult.Stdout missing 'hello': %q", result.Stdout)
	}
}

// TestStreamWriterFromContext_Nil verifies nil writer returns nil.
func TestStreamWriterFromContext_Nil(t *testing.T) {
	ctx := context.Background()
	if w := StreamWriterFromContext(ctx); w != nil {
		t.Errorf("expected nil writer for empty context, got %T", w)
	}
	// Setting nil should not panic and should be a no-op.
	ctx2 := ContextWithStreamWriter(ctx, nil)
	if w := StreamWriterFromContext(ctx2); w != nil {
		t.Errorf("expected nil writer after setting nil, got %T", w)
	}
}
