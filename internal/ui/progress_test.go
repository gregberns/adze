package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgressNonTTY_Success(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 3, false, false)

	p.StartStep("install-homebrew")
	p.CompleteStep("install-homebrew", "success", "", 1*time.Second)

	output := buf.String()
	expected := "[1/3] ✓ install-homebrew\n"
	if output != expected {
		t.Errorf("got %q, want %q", output, expected)
	}
}

func TestProgressNonTTY_Failure(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 2, false, false)

	p.StartStep("install-package")
	p.CompleteStep("install-package", "failure", "command not found", 500*time.Millisecond)

	output := buf.String()
	expected := "[1/2] ✗ install-package — command not found\n"
	if output != expected {
		t.Errorf("got %q, want %q", output, expected)
	}
}

func TestProgressNonTTY_Skip(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 2, false, false)

	p.StartStep("setup-linux")
	p.CompleteStep("setup-linux", "skip", "not on linux", 0)

	output := buf.String()
	expected := "[1/2] - setup-linux — not on linux\n"
	if output != expected {
		t.Errorf("got %q, want %q", output, expected)
	}
}

func TestProgressNonTTY_Warning(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false)

	p.StartStep("check-config")
	p.CompleteStep("check-config", "warning", "deprecated field", 0)

	output := buf.String()
	expected := "[1/1] ⚠ check-config — deprecated field\n"
	if output != expected {
		t.Errorf("got %q, want %q", output, expected)
	}
}

func TestProgressStepCounter(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 3, false, false)

	steps := []string{"step-a", "step-b", "step-c"}
	for _, name := range steps {
		p.StartStep(name)
		p.CompleteStep(name, "success", "", 0)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}

	expectedPrefixes := []string{"[1/3]", "[2/3]", "[3/3]"}
	for i, line := range lines {
		if !strings.HasPrefix(line, expectedPrefixes[i]) {
			t.Errorf("line %d = %q, want prefix %q", i, line, expectedPrefixes[i])
		}
	}
}

func TestProgressElapsedTime_Over2s(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false)

	p.StartStep("slow-step")
	p.CompleteStep("slow-step", "success", "", 3200*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "(3.2s)") {
		t.Errorf("expected elapsed time (3.2s) in output, got %q", output)
	}
}

func TestProgressElapsedTime_Under2s(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false)

	p.StartStep("fast-step")
	p.CompleteStep("fast-step", "success", "", 1500*time.Millisecond)

	output := buf.String()
	if strings.Contains(output, "s)") {
		t.Errorf("should not show elapsed time for <2s, got %q", output)
	}
}

func TestProgressElapsedTime_Exactly2s(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false)

	p.StartStep("boundary-step")
	p.CompleteStep("boundary-step", "success", "", 2*time.Second)

	output := buf.String()
	// Exactly 2s should NOT show time (spec says > 2 seconds).
	if strings.Contains(output, "s)") {
		t.Errorf("should not show elapsed time for exactly 2s, got %q", output)
	}
}

func TestProgressElapsedTime_WithReason(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false)

	p.StartStep("slow-fail")
	p.CompleteStep("slow-fail", "failure", "timed out", 5*time.Second)

	output := buf.String()
	// Time comes before reason.
	expected := "[1/1] ✗ slow-fail (5.0s) — timed out\n"
	if output != expected {
		t.Errorf("got %q, want %q", output, expected)
	}
}

func TestProgressWithColor(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, true, false)

	p.StartStep("colored-step")
	p.CompleteStep("colored-step", "success", "", 0)

	output := buf.String()
	// Should contain green ANSI code for the success symbol.
	if !strings.Contains(output, "\033[32m") {
		t.Errorf("colored output should contain green ANSI code, got %q", output)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("output should contain ✓, got %q", output)
	}
}

func TestProgressTTY_NoSpinnerOutput(t *testing.T) {
	// In TTY mode, StartStep starts a spinner. CompleteStep stops it.
	// We mainly verify this doesn't panic. Detailed spinner testing is
	// in spinner_test.go.
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, true)

	p.StartStep("tty-step")
	p.CompleteStep("tty-step", "success", "", 0)

	output := buf.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("TTY output should contain ✓ after completion, got %q", output)
	}
}

func TestProgressFinish(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false)
	// Finish should not panic even if called without steps.
	p.Finish()
}

// TestProgress_StreamWriter_SilentStep verifies that a step which writes nothing
// to the stream writer still produces the compact single-line output (no header).
func TestProgress_StreamWriter_SilentStep(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false) // non-TTY for deterministic output

	p.StartStep("git-config")
	_ = p.StreamWriter() // obtain writer but never write to it
	p.CompleteStep("git-config", "success", "", 0)

	out := buf.String()
	if strings.Contains(out, "[1/1] git-config\n") && strings.Contains(out, "└─") {
		t.Errorf("silent step should not produce header + indented status; got: %q", out)
	}
	// Should contain a single result line.
	if !strings.Contains(out, "git-config") {
		t.Errorf("output missing step name: %q", out)
	}
}

// TestProgress_StreamWriter_NonTTY verifies that StreamWriter in non-TTY mode
// returns a writer that streams directly without header coordination.
func TestProgress_StreamWriter_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, false)
	p.StartStep("step-name")
	sw := p.StreamWriter()
	sw.Write([]byte("subprocess output\n"))
	// Non-TTY returns os.Stdout directly, not the coordinated wrapper.
	// So the buf will NOT contain the subprocess output (it went to stdout).
	// What matters is that the StreamWriter call didn't break the existing
	// completion path.
	p.CompleteStep("step-name", "success", "", 0)

	out := buf.String()
	if !strings.Contains(out, "step-name") {
		t.Errorf("output missing step name: %q", out)
	}
}

// TestProgress_StreamWriter_FirstByteCommitsHeader verifies that the first
// Write to the stream writer commits the header line "[N/M] step-name" once
// and that subsequent writes flow through without re-committing. This is the
// central UX mechanism that prevents the spinner from hiding subprocess output.
func TestProgress_StreamWriter_FirstByteCommitsHeader(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1, false, true) // TTY mode

	p.StartStep("brew-casks")
	sw := p.StreamWriter()

	// Give the spinner a tick or two to write something so we know it's
	// running before the first Write to the stream writer.
	time.Sleep(150 * time.Millisecond)

	sw.Write([]byte("downloading...\n"))
	sw.Write([]byte("installing...\n"))

	p.CompleteStep("brew-casks", "success", "", 0)

	out := buf.String()

	// Header must appear exactly once.
	if got := strings.Count(out, "[1/1] brew-casks\n"); got != 1 {
		t.Errorf("expected header exactly once, got %d times. Output:\n%s", got, out)
	}
	// Streamed bytes must be present.
	if !strings.Contains(out, "downloading...") {
		t.Errorf("missing first streamed line: %q", out)
	}
	if !strings.Contains(out, "installing...") {
		t.Errorf("missing second streamed line: %q", out)
	}
	// Completion line should use the indented form because streaming happened.
	if !strings.Contains(out, "└─") {
		t.Errorf("expected indented completion line (└─); got %q", out)
	}
	// wasStreamed should report true.
	if !p.wasStreamed() {
		t.Error("Progress.wasStreamed() should be true after streaming")
	}
}
