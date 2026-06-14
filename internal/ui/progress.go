package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Progress tracks step execution progress.
type Progress struct {
	total   int
	writer  io.Writer
	color   bool
	tty     bool
	spinner *Spinner

	// mu guards current, currentName, streamedThisStep. The runner main
	// goroutine writes via StartStep/CompleteStep; the subprocess pipe
	// goroutine reads/writes via the StreamWriter wrapper.
	mu               sync.Mutex
	current          int
	currentName      string
	streamedThisStep bool
}

// NewProgress creates a new Progress tracker. It writes output to w, expects
// total steps, and uses color/tty to determine output formatting. When tty is
// true, a spinner is displayed for in-progress steps.
func NewProgress(w io.Writer, total int, color bool, tty bool) *Progress {
	p := &Progress{
		total:  total,
		writer: w,
		color:  color,
		tty:    tty,
	}
	if tty {
		p.spinner = NewSpinner(w)
	}
	return p
}

// StartStep begins display for a new step. In TTY mode, this starts the
// spinner animation. In non-TTY mode, nothing is displayed until completion.
func (p *Progress) StartStep(name string) {
	p.mu.Lock()
	p.current++
	p.currentName = name
	p.streamedThisStep = false
	label := fmt.Sprintf("[%d/%d] %s", p.current, p.total, name)
	p.mu.Unlock()

	if p.tty && p.spinner != nil {
		p.spinner.Start(label)
	}
}

// CompleteStep marks the current step as done and prints the completion line.
// status should be one of: "success", "failure", "warning", "skip".
// reason is an optional explanation (shown for failure/warning/skip).
// duration is shown if > 2 seconds.
func (p *Progress) CompleteStep(name string, status string, reason string, duration time.Duration) {
	if p.tty && p.spinner != nil {
		p.spinner.Stop()
	}

	symbol := p.symbolForStatus(status)

	var line string
	if p.wasStreamed() {
		// Indented form — header and output already showed the step name
		// above; this is the conclusion line.
		line = fmt.Sprintf("    └─ %s %s", symbol, name)
	} else {
		prefix := fmt.Sprintf("[%d/%d]", p.current, p.total)
		line = fmt.Sprintf("%s %s %s", prefix, symbol, name)
	}

	if duration > 2*time.Second {
		line += fmt.Sprintf(" (%.1fs)", duration.Seconds())
	}
	if reason != "" {
		line += fmt.Sprintf(" — %s", reason)
	}

	fmt.Fprintln(p.writer, line)
}

// StreamWriter returns an io.Writer for the currently-active step's subprocess
// output. The writer is "lazy": until the first byte is written, the spinner
// keeps running. On first write, the spinner is stopped, a header line
// "[N/M] step-name" is printed once, and subsequent bytes flow to the terminal.
//
// Steps that produce no output keep showing the spinner through CompleteStep,
// which prints the single result line — preserving compactness for the common
// "already satisfied" path.
//
// In non-TTY mode, returns os.Stdout directly (no spinner, no header
// coordination — line-buffered streaming).
func (p *Progress) StreamWriter() io.Writer {
	if !p.tty {
		return os.Stdout
	}
	return &progressStreamWriter{p: p}
}

// currentLabel returns "[N/M] name" for the active step.
func (p *Progress) currentLabel() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("[%d/%d] %s", p.current, p.total, p.currentName)
}

// markStreamed records that the current step produced streamed output.
func (p *Progress) markStreamed() {
	p.mu.Lock()
	p.streamedThisStep = true
	p.mu.Unlock()
}

// wasStreamed reports whether the current step produced streamed output.
func (p *Progress) wasStreamed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.streamedThisStep
}

// progressStreamWriter wraps Progress and coordinates the first-byte transition
// from spinner display to streamed output.
type progressStreamWriter struct {
	p             *Progress
	mu            sync.Mutex
	headerWritten bool
}

func (w *progressStreamWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	w.mu.Lock()
	if !w.headerWritten {
		// Stop the spinner (clears its line), then commit the header.
		if w.p.spinner != nil {
			w.p.spinner.Stop()
		}
		header := w.p.currentLabel()
		if header != "" {
			if _, err := io.WriteString(w.p.writer, header+"\n"); err != nil {
				w.mu.Unlock()
				return 0, err
			}
		}
		w.headerWritten = true
		w.p.markStreamed()
	}
	w.mu.Unlock()
	return w.p.writer.Write(b)
}

// Finish prints the final summary line after all steps have completed.
func (p *Progress) Finish() {
	// Summary line intentionally left minimal. Could be extended with
	// counts of success/fail/skip if needed.
}

func (p *Progress) symbolForStatus(status string) string {
	switch status {
	case "success":
		return SuccessSymbol(p.color)
	case "failure":
		return FailureSymbol(p.color)
	case "warning":
		return WarningSymbol(p.color)
	case "skip":
		return SkipSymbol(p.color)
	default:
		return SuccessSymbol(p.color)
	}
}
