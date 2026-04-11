// Package step defines the Step interface, status types, and result structures
// for the adze machine configuration tool's step execution system.
package step

import (
	"context"
	"time"
)

// Step is the interface that all steps must implement.
// Check inspects whether the desired state is already present.
// Apply makes changes to bring the system to the desired state.
//
// The error return is for infrastructure failures only (process couldn't launch,
// context cancelled). Desired state being absent or command exiting non-zero
// are expressed via StepResult.Status, not error.
type Step interface {
	Name() string
	Check(ctx context.Context, cfg StepConfig) (StepResult, error)
	Apply(ctx context.Context, cfg StepConfig) (StepResult, error)
}

// StepStatus represents the outcome of a step execution.
type StepStatus string

const (
	StatusSatisfied    StepStatus = "satisfied"     // check passed, no apply needed
	StatusApplied      StepStatus = "applied"       // apply ran, verify confirmed
	StatusFailed       StepStatus = "failed"        // apply non-zero or timed out
	StatusPartial      StepStatus = "partial"       // batch: some succeeded, some failed
	StatusSkipped      StepStatus = "skipped"       // not attempted (upstream fail or missing env)
	StatusVerifyFailed StepStatus = "verify_failed" // apply ok but verify failed
)

// StepResult holds the outcome of a step execution.
type StepResult struct {
	Status      StepStatus
	Reason      string        // MUST be set when Failed, Skipped, or VerifyFailed
	ItemResults []ItemResult  // nil for atomic steps
	Duration    time.Duration
}

// ItemResult holds the outcome of a single item in a batch step.
type ItemResult struct {
	Item   StepItem
	Status StepStatus // only: satisfied, applied, failed
	Reason string     // MUST be set when failed
}
