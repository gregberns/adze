package step

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestExecuteStepCheckSatisfied verifies that when check passes, apply is skipped.
func TestExecuteStepCheckSatisfied(t *testing.T) {
	applyCalled := false
	s := &mockStep{
		name: "already-satisfied",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusSatisfied}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			applyCalled = true
			return StepResult{Status: StatusApplied}, nil
		},
	}

	cfg := StepConfig{
		Name:  "already-satisfied",
		Check: &ShellCommand{Args: []string{"true"}},
		Apply: &ShellCommand{Args: []string{"true"}},
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
	if applyCalled {
		t.Error("apply should not have been called when check is satisfied")
	}
}

// TestExecuteStepFullLifecycle verifies check fail -> apply -> verify -> applied.
func TestExecuteStepFullLifecycle(t *testing.T) {
	checkCount := 0
	s := &mockStep{
		name: "full-lifecycle",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			checkCount++
			if checkCount == 1 {
				// First check: unsatisfied.
				return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
			}
			// Second check (verify): satisfied.
			return StepResult{Status: StatusSatisfied}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusApplied}, nil
		},
	}

	cfg := StepConfig{
		Name:  "full-lifecycle",
		Check: &ShellCommand{Args: []string{"test", "-f", "/tmp/test"}},
		Apply: &ShellCommand{Args: []string{"touch", "/tmp/test"}},
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
	if checkCount != 2 {
		t.Errorf("expected check called twice (check + verify), got %d", checkCount)
	}
}

// TestExecuteStepApplyFails verifies that failed apply returns StatusFailed.
func TestExecuteStepApplyFails(t *testing.T) {
	s := &mockStep{
		name: "apply-fails",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusFailed, Reason: "apply exited with code 1"}, nil
		},
	}

	cfg := StepConfig{
		Name:  "apply-fails",
		Check: &ShellCommand{Args: []string{"false"}},
		Apply: &ShellCommand{Args: []string{"false"}},
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
}

// TestExecuteStepVerifyFails verifies StatusVerifyFailed.
func TestExecuteStepVerifyFails(t *testing.T) {
	checkCount := 0
	s := &mockStep{
		name: "verify-fails",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			checkCount++
			if checkCount == 1 {
				return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
			}
			// Verify fails.
			return StepResult{Status: StatusFailed, Reason: "still not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusApplied}, nil
		},
	}

	cfg := StepConfig{
		Name:  "verify-fails",
		Check: &ShellCommand{Args: []string{"false"}},
		Apply: &ShellCommand{Args: []string{"true"}},
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusVerifyFailed {
		t.Errorf("expected verify_failed, got %s", result.Status)
	}
}

// TestExecuteStepMissingEnvVar verifies pre-flight skip for missing required env.
func TestExecuteStepMissingEnvVar(t *testing.T) {
	s := &mockStep{
		name: "needs-env",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("check should not be called when env is missing")
			return StepResult{}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("apply should not be called when env is missing")
			return StepResult{}, nil
		},
	}

	cfg := StepConfig{
		Name: "needs-env",
		Env:  []string{"MISSING_VAR"},
	}

	envChecker := func(name string) (string, bool, bool) {
		if name == "MISSING_VAR" {
			return "", true, false // required but not present
		}
		return "", false, false
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", envChecker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSkipped {
		t.Errorf("expected skipped, got %s", result.Status)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason for skipped step")
	}
	expectedReason := "missing required env var: MISSING_VAR"
	if result.Reason != expectedReason {
		t.Errorf("expected reason %q, got %q", expectedReason, result.Reason)
	}
}

// TestExecuteStepEnvPresent verifies that present env vars allow normal execution.
func TestExecuteStepEnvPresent(t *testing.T) {
	s := &mockStep{
		name: "env-present",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusSatisfied}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusApplied}, nil
		},
	}

	cfg := StepConfig{
		Name: "env-present",
		Env:  []string{"PRESENT_VAR"},
	}

	envChecker := func(name string) (string, bool, bool) {
		return "value", true, true
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", envChecker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

// TestExecuteStepPlatformDispatch verifies platform-specific apply resolution.
func TestExecuteStepPlatformDispatch(t *testing.T) {
	var appliedWith string
	s := &mockStep{
		name: "platform-dispatch",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			appliedWith = cfg.Apply.Args[0]
			return StepResult{Status: StatusApplied}, nil
		},
	}

	cfg := StepConfig{
		Name:  "platform-dispatch",
		Check: &ShellCommand{Args: []string{"false"}},
		Apply: &ShellCommand{Args: []string{"generic-apply"}},
		PlatformApply: map[string]*ShellCommand{
			"darwin": {Args: []string{"darwin-apply"}},
			"ubuntu": {Args: []string{"ubuntu-apply"}},
		},
	}

	// Test darwin platform.
	checkCount := 0
	s.checkFunc = func(ctx context.Context, cfg StepConfig) (StepResult, error) {
		checkCount++
		if checkCount == 1 {
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		}
		return StepResult{Status: StatusSatisfied}, nil
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApplied {
		t.Errorf("expected applied, got %s", result.Status)
	}
	if appliedWith != "darwin-apply" {
		t.Errorf("expected darwin-apply, got %s", appliedWith)
	}

	// Test fallback to generic.
	checkCount = 0
	result, err = ExecuteStep(context.Background(), s, cfg, "freebsd", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appliedWith != "generic-apply" {
		t.Errorf("expected generic-apply, got %s", appliedWith)
	}
}

// TestExecuteStepNoPlatformApply verifies skip when no apply command for platform.
func TestExecuteStepNoPlatformApply(t *testing.T) {
	s := &mockStep{
		name: "no-platform",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("apply should not be called")
			return StepResult{}, nil
		},
	}

	cfg := StepConfig{
		Name:  "no-platform",
		Check: &ShellCommand{Args: []string{"false"}},
		// No Apply, no PlatformApply.
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSkipped {
		t.Errorf("expected skipped, got %s", result.Status)
	}
	if result.Reason == "" {
		t.Error("expected reason for skipped step")
	}
}

// TestExecuteStepNilCheckAlwaysUnsatisfied verifies nil check -> always unsatisfied.
func TestExecuteStepNilCheckAlwaysUnsatisfied(t *testing.T) {
	s := NewShellStep("nil-check-exec")
	cfg := StepConfig{
		Name:  "nil-check-exec",
		Apply: &ShellCommand{Args: []string{"true"}},
		// Check is nil -> always unsatisfied -> proceed to apply.
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Apply runs, but verify re-runs nil check -> verify_failed.
	if result.Status != StatusVerifyFailed {
		t.Errorf("expected verify_failed (nil check for verify), got %s", result.Status)
	}
}

// TestExecuteStepCheckTimeout verifies that check timeout is treated as unsatisfied.
func TestExecuteStepCheckTimeout(t *testing.T) {
	checkCount := 0
	s := &mockStep{
		name: "check-timeout",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			checkCount++
			if checkCount == 1 {
				// Simulate a blocking check that will be interrupted by timeout.
				<-ctx.Done()
				return StepResult{Status: StatusFailed, Reason: "timed out"}, nil
			}
			// Verify pass.
			return StepResult{Status: StatusSatisfied}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusApplied}, nil
		},
	}

	cfg := StepConfig{
		Name:         "check-timeout",
		Check:        &ShellCommand{Args: []string{"sleep", "999"}},
		Apply:        &ShellCommand{Args: []string{"true"}},
		CheckTimeout: 100 * time.Millisecond, // Very short for test.
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check timeout -> treated as unsatisfied -> apply -> verify -> applied.
	if result.Status != StatusApplied {
		t.Errorf("expected applied after check timeout, got %s", result.Status)
	}
}

// TestExecuteStepApplyTimeout verifies that apply timeout returns StatusFailed.
func TestExecuteStepApplyTimeout(t *testing.T) {
	s := &mockStep{
		name: "apply-timeout",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			// Block until context is cancelled (timeout).
			<-ctx.Done()
			return StepResult{Status: StatusFailed, Reason: "apply timed out"}, nil
		},
	}

	cfg := StepConfig{
		Name:         "apply-timeout",
		Check:        &ShellCommand{Args: []string{"false"}},
		Apply:        &ShellCommand{Args: []string{"sleep", "999"}},
		ApplyTimeout: 100 * time.Millisecond,
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	expectedReason := fmt.Sprintf("timed out after %s", 100*time.Millisecond)
	if result.Reason != expectedReason {
		t.Errorf("expected reason %q, got %q", expectedReason, result.Reason)
	}
}

// TestExecuteStepDuration verifies that the result has a non-zero duration.
func TestExecuteStepDuration(t *testing.T) {
	s := &mockStep{
		name: "duration-test",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusSatisfied}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{}, nil
		},
	}

	cfg := StepConfig{Name: "duration-test"}
	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

// TestExecuteStepWithRealCommands tests the full lifecycle with real shell commands.
func TestExecuteStepWithRealCommands(t *testing.T) {
	s := NewShellStep("real-commands")
	cfg := StepConfig{
		Name:  "real-commands",
		Check: &ShellCommand{Args: []string{"true"}}, // Always satisfied.
		Apply: &ShellCommand{Args: []string{"echo", "applied"}},
	}

	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
}

func TestExecuteStepRealLifecycle(t *testing.T) {
	s := NewShellStep("real-lifecycle")
	cfg := StepConfig{
		Name:  "real-lifecycle",
		Check: &ShellCommand{Args: []string{"false"}}, // Always unsatisfied.
		Apply: &ShellCommand{Args: []string{"true"}},   // Apply succeeds.
	}

	// Check fails, apply succeeds, verify re-runs check which fails -> verify_failed.
	result, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusVerifyFailed {
		t.Errorf("expected verify_failed (check always fails), got %s", result.Status)
	}
}

// --- Batch step tests ---

func TestBatchStepAllSatisfied(t *testing.T) {
	s := &mockStep{
		name: "batch-all-satisfied",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusSatisfied}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("apply should not be called when all items satisfied")
			return StepResult{}, nil
		},
	}

	cfg := StepConfig{
		Name:  "batch-all-satisfied",
		Check: &ShellCommand{Args: []string{"true"}},
		Apply: &ShellCommand{Args: []string{"true"}},
		Items: []StepItem{
			{Name: "pkg1"},
			{Name: "pkg2"},
			{Name: "pkg3"},
		},
	}

	result, err := ExecuteBatchStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSatisfied {
		t.Errorf("expected satisfied, got %s", result.Status)
	}
	if len(result.ItemResults) != 3 {
		t.Fatalf("expected 3 item results, got %d", len(result.ItemResults))
	}
	for _, ir := range result.ItemResults {
		if ir.Status != StatusSatisfied {
			t.Errorf("expected item %s satisfied, got %s", ir.Item.Name, ir.Status)
		}
	}
}

func TestBatchStepAllFailed(t *testing.T) {
	checkCount := 0
	s := &mockStep{
		name: "batch-all-failed",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			checkCount++
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusFailed, Reason: "install failed"}, nil
		},
	}

	cfg := StepConfig{
		Name:  "batch-all-failed",
		Check: &ShellCommand{Args: []string{"false"}},
		Apply: &ShellCommand{Args: []string{"false"}},
		Items: []StepItem{
			{Name: "pkg1"},
			{Name: "pkg2"},
		},
	}

	result, err := ExecuteBatchStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	for _, ir := range result.ItemResults {
		if ir.Status != StatusFailed {
			t.Errorf("expected item %s failed, got %s", ir.Item.Name, ir.Status)
		}
	}
}

func TestBatchStepPartial(t *testing.T) {
	// Track check calls per item name to differentiate check vs verify.
	checkCalls := map[string]int{}

	s := &mockStep{
		name: "batch-partial",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			checkCalls[cfg.Name]++
			count := checkCalls[cfg.Name]

			// For pkg1: first check fails, second check (verify) succeeds.
			if cfg.Name == "batch-partial[pkg1]" {
				if count == 1 {
					return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
				}
				return StepResult{Status: StatusSatisfied}, nil
			}
			// For pkg2: check always fails.
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			if cfg.Name == "batch-partial[pkg1]" {
				return StepResult{Status: StatusApplied}, nil
			}
			// pkg2 apply fails.
			return StepResult{Status: StatusFailed, Reason: "install failed"}, nil
		},
	}

	cfg := StepConfig{
		Name:  "batch-partial",
		Check: &ShellCommand{Args: []string{"false"}},
		Apply: &ShellCommand{Args: []string{"true"}},
		Items: []StepItem{
			{Name: "pkg1"},
			{Name: "pkg2"},
		},
	}

	result, err := ExecuteBatchStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusPartial {
		t.Errorf("expected partial, got %s", result.Status)
	}
	if len(result.ItemResults) != 2 {
		t.Fatalf("expected 2 item results, got %d", len(result.ItemResults))
	}
	if result.ItemResults[0].Status != StatusApplied {
		t.Errorf("expected first item applied, got %s", result.ItemResults[0].Status)
	}
	if result.ItemResults[1].Status != StatusFailed {
		t.Errorf("expected second item failed, got %s", result.ItemResults[1].Status)
	}
}

func TestBatchStepEmptyItems(t *testing.T) {
	s := &mockStep{
		name: "batch-empty",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("should not be called for empty items")
			return StepResult{}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("should not be called for empty items")
			return StepResult{}, nil
		},
	}

	cfg := StepConfig{
		Name:  "batch-empty",
		Items: []StepItem{},
	}

	result, err := ExecuteBatchStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSatisfied {
		t.Errorf("expected satisfied for empty items, got %s", result.Status)
	}
}

func TestBatchStepFailedDoesNotHaltIteration(t *testing.T) {
	applyCount := 0
	s := &mockStep{
		name: "batch-no-halt",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{Status: StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			applyCount++
			return StepResult{Status: StatusFailed, Reason: "failed"}, nil
		},
	}

	cfg := StepConfig{
		Name:  "batch-no-halt",
		Check: &ShellCommand{Args: []string{"false"}},
		Apply: &ShellCommand{Args: []string{"false"}},
		Items: []StepItem{
			{Name: "pkg1"},
			{Name: "pkg2"},
			{Name: "pkg3"},
		},
	}

	_, err := ExecuteBatchStep(context.Background(), s, cfg, "darwin", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applyCount != 3 {
		t.Errorf("expected all 3 items attempted, got %d", applyCount)
	}
}

func TestBatchStepMissingEnv(t *testing.T) {
	s := &mockStep{
		name: "batch-missing-env",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("should not be called")
			return StepResult{}, nil
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			t.Fatal("should not be called")
			return StepResult{}, nil
		},
	}

	cfg := StepConfig{
		Name:  "batch-missing-env",
		Env:   []string{"MISSING"},
		Items: []StepItem{{Name: "pkg1"}},
	}

	envChecker := func(name string) (string, bool, bool) {
		return "", true, false
	}

	result, err := ExecuteBatchStep(context.Background(), s, cfg, "darwin", envChecker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusSkipped {
		t.Errorf("expected skipped, got %s", result.Status)
	}
	expectedReason := "missing required env var: MISSING"
	if result.Reason != expectedReason {
		t.Errorf("expected reason %q, got %q", expectedReason, result.Reason)
	}
}

// TestAggregateItemResults tests the aggregation logic directly.
func TestAggregateItemResults(t *testing.T) {
	tests := []struct {
		name     string
		results  []ItemResult
		expected StepStatus
	}{
		{
			name:     "empty",
			results:  []ItemResult{},
			expected: StatusSatisfied,
		},
		{
			name: "all satisfied",
			results: []ItemResult{
				{Status: StatusSatisfied},
				{Status: StatusSatisfied},
			},
			expected: StatusSatisfied,
		},
		{
			name: "all applied",
			results: []ItemResult{
				{Status: StatusApplied},
				{Status: StatusApplied},
			},
			expected: StatusApplied,
		},
		{
			name: "mix satisfied and applied",
			results: []ItemResult{
				{Status: StatusSatisfied},
				{Status: StatusApplied},
			},
			expected: StatusApplied,
		},
		{
			name: "all failed",
			results: []ItemResult{
				{Status: StatusFailed},
				{Status: StatusFailed},
			},
			expected: StatusFailed,
		},
		{
			name: "partial: one success one fail",
			results: []ItemResult{
				{Status: StatusApplied},
				{Status: StatusFailed},
			},
			expected: StatusPartial,
		},
		{
			name: "partial: satisfied and failed",
			results: []ItemResult{
				{Status: StatusSatisfied},
				{Status: StatusFailed},
			},
			expected: StatusPartial,
		},
		{
			name: "partial: mix of all three",
			results: []ItemResult{
				{Status: StatusSatisfied},
				{Status: StatusApplied},
				{Status: StatusFailed},
			},
			expected: StatusPartial,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregateItemResults(tt.results)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

// TestResolveApplyCommand tests platform dispatch resolution.
func TestResolveApplyCommand(t *testing.T) {
	generic := &ShellCommand{Args: []string{"generic"}}
	darwin := &ShellCommand{Args: []string{"darwin"}}
	ubuntu := &ShellCommand{Args: []string{"ubuntu"}}

	tests := []struct {
		name     string
		cfg      StepConfig
		platform string
		want     string // expected Args[0], or "" for nil
	}{
		{
			name:     "platform specific",
			cfg:      StepConfig{Apply: generic, PlatformApply: map[string]*ShellCommand{"darwin": darwin}},
			platform: "darwin",
			want:     "darwin",
		},
		{
			name:     "fallback to generic",
			cfg:      StepConfig{Apply: generic, PlatformApply: map[string]*ShellCommand{"ubuntu": ubuntu}},
			platform: "darwin",
			want:     "generic",
		},
		{
			name:     "no apply at all",
			cfg:      StepConfig{},
			platform: "darwin",
			want:     "",
		},
		{
			name:     "generic only",
			cfg:      StepConfig{Apply: generic},
			platform: "freebsd",
			want:     "generic",
		},
		{
			name:     "nil platform map",
			cfg:      StepConfig{Apply: generic},
			platform: "darwin",
			want:     "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveApplyCommand(tt.cfg, tt.platform)
			if tt.want == "" {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else {
				if got == nil {
					t.Fatal("expected non-nil command")
				}
				if got.Args[0] != tt.want {
					t.Errorf("expected %s, got %s", tt.want, got.Args[0])
				}
			}
		})
	}
}

// TestExecuteStepInfraError verifies that infra errors propagate correctly.
func TestExecuteStepInfraError(t *testing.T) {
	s := &mockStep{
		name: "infra-error",
		checkFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{}, fmt.Errorf("connection refused")
		},
		applyFunc: func(ctx context.Context, cfg StepConfig) (StepResult, error) {
			return StepResult{}, nil
		},
	}

	cfg := StepConfig{
		Name:  "infra-error",
		Check: &ShellCommand{Args: []string{"true"}},
		Apply: &ShellCommand{Args: []string{"true"}},
	}

	_, err := ExecuteStep(context.Background(), s, cfg, "darwin", nil)
	if err == nil {
		t.Fatal("expected infra error to propagate")
	}
}
