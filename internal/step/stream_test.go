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

// TestRunCommand_StdinPassthrough verifies that when a stdin reader is
// attached to the context, RunCommand feeds it to the subprocess as stdin.
// Without this, interactive commands (gh auth login, git credential prompts)
// would read EOF and fail or hang.
func TestRunCommand_StdinPassthrough(t *testing.T) {
	in := strings.NewReader("hello world\n")
	ctx := ContextWithStdin(context.Background(), in)

	cmd := &ShellCommand{Args: []string{"cat"}}
	result, err := RunCommand(ctx, cmd, nil, "test", "apply")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; stderr=%q", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("ExecResult.Stdout missing 'hello world': %q", result.Stdout)
	}
}

// TestRunCommand_NoStdin verifies that without a stdin reader in context,
// subprocess stdin is nil (reads return EOF immediately) — the pre-existing
// default behavior.
func TestRunCommand_NoStdin(t *testing.T) {
	ctx := context.Background()
	// `cat` with no stdin should exit 0 immediately on EOF.
	cmd := &ShellCommand{Args: []string{"cat"}}
	result, err := RunCommand(ctx, cmd, nil, "test", "apply")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "" {
		t.Errorf("expected empty stdout, got %q", result.Stdout)
	}
}

// TestStdinFromContext_Nil verifies nil-safety mirroring stream writer.
func TestStdinFromContext_Nil(t *testing.T) {
	ctx := context.Background()
	if r := StdinFromContext(ctx); r != nil {
		t.Errorf("expected nil reader for empty context, got %T", r)
	}
	ctx2 := ContextWithStdin(ctx, nil)
	if r := StdinFromContext(ctx2); r != nil {
		t.Errorf("expected nil reader after setting nil, got %T", r)
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
