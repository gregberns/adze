package step

import (
	"context"
	"testing"
	"time"
)

// mockStep is a test double for the Step interface.
type mockStep struct {
	name      string
	checkFunc func(ctx context.Context, cfg StepConfig) (StepResult, error)
	applyFunc func(ctx context.Context, cfg StepConfig) (StepResult, error)
}

func (m *mockStep) Name() string { return m.name }
func (m *mockStep) Check(ctx context.Context, cfg StepConfig) (StepResult, error) {
	return m.checkFunc(ctx, cfg)
}
func (m *mockStep) Apply(ctx context.Context, cfg StepConfig) (StepResult, error) {
	return m.applyFunc(ctx, cfg)
}

func TestStepInterface(t *testing.T) {
	// Verify that a mockStep satisfies the Step interface.
	var s Step = &mockStep{
		name: "test-mock",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusSatisfied}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusApplied}, nil
		},
	}

	if s.Name() != "test-mock" {
		t.Errorf("expected name 'test-mock', got %q", s.Name())
	}

	result, err := s.Check(context.Background(), StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}

	result, err = s.Apply(context.Background(), StepConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
}

func TestStepStatusValues(t *testing.T) {
	tests := []struct {
		status StepStatus
		value  string
	}{
		{StatusSatisfied, "satisfied"},
		{StatusApplied, "applied"},
		{StatusFailed, "failed"},
		{StatusPartial, "partial"},
		{StatusSkipped, "skipped"},
		{StatusVerifyFailed, "verify_failed"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.value {
			t.Errorf("expected %q, got %q", tt.value, string(tt.status))
		}
	}
}

func TestStepResultFields(t *testing.T) {
	r := StepResult{
		Status: StatusPartial,
		Reason: "some items failed",
		ItemResults: []ItemResult{
			{Item: StepItem{Name: "pkg1"}, Status: StatusApplied},
			{Item: StepItem{Name: "pkg2"}, Status: StatusFailed, Reason: "exit 1"},
		},
		Duration: 3 * time.Second,
	}

	if r.Status != StatusPartial {
		t.Errorf("expected partial, got %s", r.Status)
	}
	if len(r.ItemResults) != 2 {
		t.Fatalf("expected 2 item results, got %d", len(r.ItemResults))
	}
	if r.ItemResults[0].Item.Name != "pkg1" {
		t.Errorf("expected pkg1, got %s", r.ItemResults[0].Item.Name)
	}
	if r.ItemResults[1].Reason != "exit 1" {
		t.Errorf("expected 'exit 1', got %s", r.ItemResults[1].Reason)
	}
}
