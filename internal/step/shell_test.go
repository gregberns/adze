package step

import (
	"context"
	"testing"
)

func TestShellStepName(t *testing.T) {
	s := NewShellStep("my-step")
	if s.Name() != "my-step" {
		t.Errorf("expected 'my-step', got %q", s.Name())
	}
}

func TestShellStepCheckNilCommand(t *testing.T) {
	s := NewShellStep("nil-check")
	cfg := StepConfig{Name: "nil-check"}

	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("nil check should not return error, got: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if result.Reason != "no check command defined" {
		t.Errorf("expected 'no check command defined', got %q", result.Reason)
	}
}

func TestShellStepCheckExitZero(t *testing.T) {
	s := NewShellStep("check-ok")
	cfg := StepConfig{
		Name:  "check-ok",
		Check: &ShellCommand{Args: []string{"true"}},
	}

	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

func TestShellStepCheckExitNonZero(t *testing.T) {
	s := NewShellStep("check-fail")
	cfg := StepConfig{
		Name:  "check-fail",
		Check: &ShellCommand{Args: []string{"false"}},
	}

	result, err := s.Check(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason for failed check")
	}
}

func TestShellStepApplyNilCommand(t *testing.T) {
	s := NewShellStep("no-apply")
	cfg := StepConfig{Name: "no-apply"}

	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSkipped {
		t.Errorf("expected skipped, got %s", result.Status)
	}
}

func TestShellStepApplyExitZero(t *testing.T) {
	s := NewShellStep("apply-ok")
	cfg := StepConfig{
		Name:  "apply-ok",
		Apply: &ShellCommand{Args: []string{"true"}},
	}

	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestShellStepApplyExitNonZero(t *testing.T) {
	s := NewShellStep("apply-fail")
	cfg := StepConfig{
		Name:  "apply-fail",
		Apply: &ShellCommand{Args: []string{"false"}},
	}

	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
}

func TestShellStepRejectsBatchItems(t *testing.T) {
	s := NewShellStep("batch-reject")
	cfg := StepConfig{
		Name:  "batch-reject",
		Check: &ShellCommand{Args: []string{"true"}},
		Items: []StepItem{{Name: "pkg1"}},
	}

	_, err := s.Check(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for ShellStep with items")
	}

	cfg.Apply = &ShellCommand{Args: []string{"true"}}
	_, err = s.Apply(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for ShellStep with items on Apply")
	}
}

func TestShellStepPlatformDispatch(t *testing.T) {
	// Test that executor resolves platform apply correctly before calling ShellStep.
	// ShellStep itself just uses cfg.Apply (the executor pre-resolves it).
	s := NewShellStep("platform-test")

	// Simulate executor resolving platform apply to darwin command.
	cfg := StepConfig{
		Name:  "platform-test",
		Apply: &ShellCommand{Args: []string{"echo", "darwin-specific"}},
	}

	result, err := s.Apply(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}
