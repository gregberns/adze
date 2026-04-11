package step

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunCommandTrue(t *testing.T) {
	cmd := &ShellCommand{Args: []string{"true"}}
	result, err := RunCommand(context.Background(), cmd, nil, "test", "check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestRunCommandFalse(t *testing.T) {
	cmd := &ShellCommand{Args: []string{"false"}}
	result, err := RunCommand(context.Background(), cmd, nil, "test", "check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode == 0 {
		t.Errorf("expected non-zero exit code, got %d", result.ExitCode)
	}
}

func TestRunCommandStdout(t *testing.T) {
	cmd := &ShellCommand{Args: []string{"echo", "hello world"}}
	result, err := RunCommand(context.Background(), cmd, nil, "test", "check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
	if strings.TrimSpace(result.Stdout) != "hello world" {
		t.Errorf("expected 'hello world', got %q", result.Stdout)
	}
}

func TestRunCommandNilCommand(t *testing.T) {
	_, err := RunCommand(context.Background(), nil, nil, "test", "check")
	if err == nil {
		t.Fatal("expected error for nil command")
	}
}

func TestRunCommandEmptyArgs(t *testing.T) {
	cmd := &ShellCommand{Args: []string{}}
	_, err := RunCommand(context.Background(), cmd, nil, "test", "check")
	if err == nil {
		t.Fatal("expected error for empty args")
	}
}

func TestRunCommandNotFound(t *testing.T) {
	cmd := &ShellCommand{Args: []string{"/nonexistent/binary"}}
	_, err := RunCommand(context.Background(), cmd, nil, "test", "check")
	if err == nil {
		t.Fatal("expected error for non-existent binary")
	}
}

func TestRunCommandEnvMerging(t *testing.T) {
	// Set a unique env var in current process to verify inheritance.
	os.Setenv("ADZE_TEST_INHERIT", "inherited")
	defer os.Unsetenv("ADZE_TEST_INHERIT")

	cmd := &ShellCommand{
		Args: []string{"env"},
		Env:  map[string]string{"ADZE_TEST_CMD": "cmd_level"},
	}
	stepEnv := []string{"ADZE_TEST_STEP=step_level"}

	result, err := RunCommand(context.Background(), cmd, stepEnv, "test", "check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}

	// Verify all three levels of env vars are present.
	if !strings.Contains(result.Stdout, "ADZE_TEST_INHERIT=inherited") {
		t.Error("expected inherited env var in output")
	}
	if !strings.Contains(result.Stdout, "ADZE_TEST_STEP=step_level") {
		t.Error("expected step-level env var in output")
	}
	if !strings.Contains(result.Stdout, "ADZE_TEST_CMD=cmd_level") {
		t.Error("expected command-level env var in output")
	}
}

func TestRunCommandEnvDoesNotPersist(t *testing.T) {
	// Verify that command env vars don't leak into the calling process.
	cmd := &ShellCommand{
		Args: []string{"true"},
		Env:  map[string]string{"ADZE_TEMP_VAR": "should_not_persist"},
	}
	_, err := RunCommand(context.Background(), cmd, nil, "test", "check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if os.Getenv("ADZE_TEMP_VAR") != "" {
		t.Error("command env should not persist to calling process")
	}
}

func TestRunCommandTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	cmd := &ShellCommand{Args: []string{"sleep", "999"}}
	result, err := RunCommand(ctx, cmd, nil, "test-timeout", "check")
	if err != nil {
		t.Fatalf("unexpected error (timeout should not be infra error): %v", err)
	}
	// Timed-out process should have non-zero exit code.
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code after timeout")
	}
	if result.Duration < 100*time.Millisecond {
		t.Errorf("expected duration >= 100ms, got %s", result.Duration)
	}
}

func TestRunCommandNoShellInterpolation(t *testing.T) {
	// Verify that arguments are NOT passed through a shell.
	// If shell interpolation occurred, $HOME would be expanded.
	cmd := &ShellCommand{Args: []string{"echo", "$HOME"}}
	result, err := RunCommand(context.Background(), cmd, nil, "test", "check")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "$HOME" {
		t.Errorf("expected literal '$HOME', got %q (shell interpolation detected)", strings.TrimSpace(result.Stdout))
	}
}
