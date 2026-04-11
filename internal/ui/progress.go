package ui

import (
	"fmt"
	"io"
	"time"
)

// Progress tracks step execution progress.
type Progress struct {
	total   int
	current int
	writer  io.Writer
	color   bool
	tty     bool
	spinner *Spinner
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
	p.current++
	if p.tty && p.spinner != nil {
		label := fmt.Sprintf("[%d/%d] %s", p.current, p.total, name)
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
	prefix := fmt.Sprintf("[%d/%d]", p.current, p.total)

	line := fmt.Sprintf("%s %s %s", prefix, symbol, name)

	if duration > 2*time.Second {
		line += fmt.Sprintf(" (%.1fs)", duration.Seconds())
	}

	if reason != "" {
		line += fmt.Sprintf(" — %s", reason)
	}

	fmt.Fprintln(p.writer, line)
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
