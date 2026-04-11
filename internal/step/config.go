package step

import "time"

// Default timeouts for step phases.
const (
	DefaultCheckTimeout = 5 * time.Minute
	DefaultApplyTimeout = 15 * time.Minute
	PostSIGTERMGrace    = 5 * time.Second
)

// StepConfig holds the configuration for a step.
type StepConfig struct {
	Name        string
	Description string
	Tags        []string  // inert in v1, never used for filtering
	Provides    []string
	Requires    []string
	Platforms   []string
	Check       *ShellCommand
	Apply       *ShellCommand
	// Rollback is not executed in v1. See step-primitive-design.md.
	Rollback     *ShellCommand
	Env          []string
	CheckTimeout time.Duration // default 5 min
	ApplyTimeout time.Duration // default 15 min
	Items        []StepItem    // nil for atomic steps
	PlatformApply map[string]*ShellCommand
}

// ShellCommand describes a command to execute directly via os/exec.
// Args[0] is the executable, Args[1:] are arguments.
// MUST NOT pass through shell interpreter.
type ShellCommand struct {
	Args []string          // Args[0] is executable, Args[1:] are arguments
	Env  map[string]string // additional env vars for this command only
}

// StepItem represents a single item in a batch step (e.g., a package to install).
type StepItem struct {
	Name    string
	Version string
	Pinned  bool
}
