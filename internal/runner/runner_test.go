package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/step"
)

// --- Mock step implementation ---

type mockStep struct {
	name      string
	checkFunc func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error)
	applyFunc func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error)
}

func (m *mockStep) Name() string { return m.name }
func (m *mockStep) Check(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	if m.checkFunc != nil {
		return m.checkFunc(ctx, cfg)
	}
	return step.StepResult{Status: step.StatusSatisfied}, nil
}
func (m *mockStep) Apply(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
	if m.applyFunc != nil {
		return m.applyFunc(ctx, cfg)
	}
	return step.StepResult{Status: step.StatusApplied}, nil
}

// --- Helper to build test scenarios ---

// testScenario helps build a runner for test cases.
type testScenario struct {
	steps       []step.Step
	graph       *dag.ResolvedGraph
	stepConfigs map[string]step.StepConfig
}

func newScenario() *testScenario {
	return &testScenario{
		graph: &dag.ResolvedGraph{},
	}
}

func (ts *testScenario) addStep(name string, provides, requires []string,
	checkFn func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error),
	applyFn func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error),
) *testScenario {
	ms := &mockStep{
		name:      name,
		checkFunc: checkFn,
		applyFunc: applyFn,
	}
	ts.steps = append(ts.steps, ms)

	dependsOn := make(map[string]string)
	for _, req := range requires {
		// Find the provider from existing steps.
		for _, existing := range ts.graph.Steps {
			for _, cap := range existing.Config.Provides {
				if cap == req {
					dependsOn[req] = existing.Name
				}
			}
		}
	}

	ts.graph.Steps = append(ts.graph.Steps, dag.ResolvedStep{
		Name: name,
		Config: dag.StepInput{
			Name:     name,
			Provides: provides,
			Requires: requires,
		},
		DependsOn: dependsOn,
	})

	// Register a StepConfig override with dummy Check/Apply commands so the
	// executor doesn't short-circuit on nil commands. The mock step's
	// checkFunc/applyFunc handle the actual logic.
	if ts.stepConfigs == nil {
		ts.stepConfigs = make(map[string]step.StepConfig)
	}
	ts.stepConfigs[name] = step.StepConfig{
		Name:     name,
		Provides: provides,
		Requires: requires,
		Check:    &step.ShellCommand{Args: []string{"true"}},
		Apply:    &step.ShellCommand{Args: []string{"true"}},
	}

	return ts
}

func (ts *testScenario) build() *Runner {
	r := NewRunner(ts.steps, ts.graph, nil, "darwin")
	// Use a temp dir for logs in tests.
	r.logDir = filepath.Join(os.TempDir(), "adze-test-logs")
	// Propagate step config overrides.
	r.stepConfigs = ts.stepConfigs
	return r
}

// --- Exit code tests ---

func TestRunAllSatisfied(t *testing.T) {
	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		nil,
	)
	sc.addStep("step-b", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		nil,
	)

	r := sc.build()
	result := r.Run(context.Background())

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if len(result.StepResults) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.StepResults))
	}
	for _, sr := range result.StepResults {
		if sr.Status != step.StatusSatisfied {
			t.Errorf("expected satisfied for %s, got %s", sr.Name, sr.Status)
		}
	}
}

func TestRunAllApplied(t *testing.T) {
	checkCount := map[string]int{}
	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			checkCount[cfg.Name]++
			if checkCount[cfg.Name] == 1 {
				return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
			}
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusApplied}, nil
		},
	)

	r := sc.build()
	result := r.Run(context.Background())

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.StepResults[0].Status != step.StatusApplied {
		t.Errorf("expected applied, got %s", result.StepResults[0].Status)
	}
}

func TestRunExitCode4AllFailed(t *testing.T) {
	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "install failed"}, nil
		},
	)

	r := sc.build()
	result := r.Run(context.Background())

	if result.ExitCode != 4 {
		t.Errorf("expected exit code 4, got %d", result.ExitCode)
	}
}

func TestRunExitCode5PartialProgress(t *testing.T) {
	checkCount := map[string]int{}
	sc := newScenario()
	// step-a succeeds
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			checkCount[cfg.Name]++
			if checkCount[cfg.Name] == 1 {
				return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
			}
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusApplied}, nil
		},
	)
	// step-b fails
	sc.addStep("step-b", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "install failed"}, nil
		},
	)

	r := sc.build()
	result := r.Run(context.Background())

	if result.ExitCode != 5 {
		t.Errorf("expected exit code 5, got %d", result.ExitCode)
	}
}

func TestRunExitCode0WithSkips(t *testing.T) {
	// All satisfied plus some skipped (downstream of a step with no provides) — exit 0.
	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		nil,
	)

	r := sc.build()
	// Manually add a skipped step to the graph (simulating env skip from executor).
	r.graph.Steps = append(r.graph.Steps, dag.ResolvedStep{
		Name: "step-skipped",
		Config: dag.StepInput{
			Name: "step-skipped",
		},
	})
	// Add a mock step that returns skipped.
	r.steps = append(r.steps, &mockStep{
		name: "step-skipped",
		checkFunc: func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusSkipped, Reason: "no apply command for platform darwin"}, nil
		},
	})

	result := r.Run(context.Background())

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0 with skips, got %d", result.ExitCode)
	}
}

// --- Skip propagation tests ---

func TestSkipPropagationDirectDependency(t *testing.T) {
	sc := newScenario()
	// step-a fails and provides "cap-a"
	sc.addStep("step-a", []string{"cap-a"}, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "install failed"}, nil
		},
	)
	// step-b requires "cap-a" — should be skipped
	sc.addStep("step-b", nil, []string{"cap-a"},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			t.Fatal("step-b check should not be called")
			return step.StepResult{}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			t.Fatal("step-b apply should not be called")
			return step.StepResult{}, nil
		},
	)

	r := sc.build()
	result := r.Run(context.Background())

	if len(result.StepResults) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.StepResults))
	}

	srA := result.StepResults[0]
	if srA.Status != step.StatusFailed {
		t.Errorf("expected step-a failed, got %s", srA.Status)
	}

	srB := result.StepResults[1]
	if srB.Status != step.StatusSkipped {
		t.Errorf("expected step-b skipped, got %s", srB.Status)
	}
	if !strings.Contains(srB.Reason, "step-a failed") {
		t.Errorf("expected skip reason to mention step-a, got %q", srB.Reason)
	}
}

func TestSkipPropagationTransitive(t *testing.T) {
	sc := newScenario()
	// A fails, provides cap-a
	sc.addStep("step-a", []string{"cap-a"}, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "install failed"}, nil
		},
	)
	// B requires cap-a, provides cap-b — should be skipped
	sc.addStep("step-b", []string{"cap-b"}, []string{"cap-a"},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			t.Fatal("step-b should not be called")
			return step.StepResult{}, nil
		},
		nil,
	)
	// C requires cap-b — should be skipped transitively
	sc.addStep("step-c", nil, []string{"cap-b"},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			t.Fatal("step-c should not be called")
			return step.StepResult{}, nil
		},
		nil,
	)

	r := sc.build()
	result := r.Run(context.Background())

	if len(result.StepResults) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.StepResults))
	}

	if result.StepResults[0].Status != step.StatusFailed {
		t.Errorf("step-a: expected failed, got %s", result.StepResults[0].Status)
	}
	if result.StepResults[1].Status != step.StatusSkipped {
		t.Errorf("step-b: expected skipped, got %s", result.StepResults[1].Status)
	}
	if !strings.Contains(result.StepResults[1].Reason, "step-a failed") {
		t.Errorf("step-b: expected reason to mention step-a, got %q", result.StepResults[1].Reason)
	}
	if result.StepResults[2].Status != step.StatusSkipped {
		t.Errorf("step-c: expected skipped, got %s", result.StepResults[2].Status)
	}
	if !strings.Contains(result.StepResults[2].Reason, "step-b failed") {
		t.Errorf("step-c: expected reason to mention step-b, got %q", result.StepResults[2].Reason)
	}
}

func TestPartialDoesNotCauseSkipsExitCode(t *testing.T) {
	// Verify that partial status contributes both applied and failed
	// to the exit code calculation.
	results := []StepRunResult{
		{Name: "step-a", Status: step.StatusPartial},
		{Name: "step-b", Status: step.StatusSatisfied},
	}
	code := computeExitCode(results)
	if code != 5 {
		t.Errorf("partial + satisfied should give exit code 5, got %d", code)
	}
}

func TestPartialDoesNotCauseSkipsIntegration(t *testing.T) {
	// Integration test: verify that a partial step does not skip downstream steps.
	// We build a runner where step-a uses a mock step that produces partial
	// through the executor, and step-b should still run.
	//
	// To achieve batch execution, we add a StepConfigs override to the runner.

	graph := &dag.ResolvedGraph{
		Steps: []dag.ResolvedStep{
			{
				Name: "step-a",
				Config: dag.StepInput{
					Name:     "step-a",
					Provides: []string{"cap-a"},
				},
			},
			{
				Name: "step-a-downstream",
				Config: dag.StepInput{
					Name:     "step-a-downstream",
					Requires: []string{"cap-a"},
				},
				DependsOn: map[string]string{"cap-a": "step-a"},
			},
		},
	}

	// step-a: batch step that produces partial (some succeed, some fail).
	checkCalls := map[string]int{}
	stepA := &mockStep{
		name: "step-a",
		checkFunc: func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			checkCalls[cfg.Name]++
			count := checkCalls[cfg.Name]
			if strings.HasSuffix(cfg.Name, "[pkg1]") {
				if count == 1 {
					return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
				}
				return step.StepResult{Status: step.StatusSatisfied}, nil
			}
			// pkg2 always fails
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		applyFunc: func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			if strings.HasSuffix(cfg.Name, "[pkg1]") {
				return step.StepResult{Status: step.StatusApplied}, nil
			}
			return step.StepResult{Status: step.StatusFailed, Reason: "install failed"}, nil
		},
	}

	// step-a-downstream: should still run (partial does not taint).
	downstreamCalled := false
	stepB := &mockStep{
		name: "step-a-downstream",
		checkFunc: func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			downstreamCalled = true
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
	}

	steps := []step.Step{stepA, stepB}
	r := NewRunner(steps, graph, nil, "darwin")
	r.logDir = filepath.Join(os.TempDir(), "adze-test-logs-partial")

	// Inject step configs with items for step-a.
	r.stepConfigs = map[string]step.StepConfig{
		"step-a": {
			Name:     "step-a",
			Provides: []string{"cap-a"},
			Check:    &step.ShellCommand{Args: []string{"true"}},
			Apply:    &step.ShellCommand{Args: []string{"true"}},
			Items: []step.StepItem{
				{Name: "pkg1"},
				{Name: "pkg2"},
			},
		},
	}

	result := r.Run(context.Background())

	if !downstreamCalled {
		t.Error("expected downstream step to be called (partial should not skip)")
	}

	// step-a should be partial
	if result.StepResults[0].Status != step.StatusPartial {
		t.Errorf("expected step-a partial, got %s", result.StepResults[0].Status)
	}

	// step-a-downstream should be satisfied (not skipped)
	if result.StepResults[1].Status == step.StatusSkipped {
		t.Error("downstream step should NOT be skipped when upstream is partial")
	}
}

func TestSkipPropagationVerifyFailed(t *testing.T) {
	checkCount := map[string]int{}
	sc := newScenario()
	// step-a: verify_failed — check fails, apply succeeds, verify fails.
	sc.addStep("step-a", []string{"cap-a"}, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			checkCount[cfg.Name]++
			// Always return failed (check never passes).
			return step.StepResult{Status: step.StatusFailed, Reason: "not satisfied"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusApplied}, nil
		},
	)
	// step-b: requires cap-a — should be skipped.
	sc.addStep("step-b", nil, []string{"cap-a"},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			t.Fatal("step-b should not be called")
			return step.StepResult{}, nil
		},
		nil,
	)

	r := sc.build()
	result := r.Run(context.Background())

	if result.StepResults[0].Status != step.StatusVerifyFailed {
		t.Errorf("step-a: expected verify_failed, got %s", result.StepResults[0].Status)
	}
	if result.StepResults[1].Status != step.StatusSkipped {
		t.Errorf("step-b: expected skipped, got %s", result.StepResults[1].Status)
	}
}

// --- Summary formatting tests ---

func TestSummaryAllPass(t *testing.T) {
	result := &RunResult{
		StepResults: []StepRunResult{
			{Name: "step-a", Status: step.StatusSatisfied},
			{Name: "step-b", Status: step.StatusApplied},
		},
		ExitCode: 0,
	}

	summary := FormatSummary(result)

	if !strings.Contains(summary, "=== Run Summary ===") {
		t.Error("missing summary header")
	}
	if !strings.Contains(summary, "Steps: 2 total") {
		t.Error("missing total count")
	}
	if !strings.Contains(summary, "2 succeeded (1 applied, 1 already satisfied)") {
		t.Errorf("missing succeeded line, got:\n%s", summary)
	}
	if !strings.Contains(summary, "0 failed") {
		t.Error("missing failed count")
	}
	if !strings.Contains(summary, "0 skipped") {
		t.Error("missing skipped count")
	}
	// No "Failed:" section.
	if strings.Contains(summary, "\nFailed:\n") {
		t.Error("should not have Failed section when no failures")
	}
	// No "Re-run" line.
	if strings.Contains(summary, "Re-run:") {
		t.Error("should not have Re-run line when no failures")
	}
}

func TestSummaryMixed(t *testing.T) {
	result := &RunResult{
		StepResults: []StepRunResult{
			{Name: "step-a", Status: step.StatusApplied},
			{Name: "step-b", Status: step.StatusFailed, Reason: "install failed", LogPath: "/tmp/logs/step-b.log"},
			{Name: "step-c", Status: step.StatusSkipped, Reason: "skipped because step-b failed"},
		},
		ExitCode: 5,
	}

	summary := FormatSummary(result)

	if !strings.Contains(summary, "Steps: 3 total") {
		t.Errorf("wrong total, got:\n%s", summary)
	}
	if !strings.Contains(summary, "1 succeeded") {
		t.Errorf("wrong succeeded count, got:\n%s", summary)
	}
	if !strings.Contains(summary, "1 failed") {
		t.Errorf("wrong failed count, got:\n%s", summary)
	}
	if !strings.Contains(summary, "1 skipped") {
		t.Errorf("wrong skipped count, got:\n%s", summary)
	}
	if !strings.Contains(summary, "\nFailed:\n") {
		t.Errorf("missing Failed section, got:\n%s", summary)
	}
	if !strings.Contains(summary, "step-b") {
		t.Errorf("missing step-b in failed section, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Log: /tmp/logs/step-b.log") {
		t.Errorf("missing log path, got:\n%s", summary)
	}
	if !strings.Contains(summary, "\nSkipped:\n") {
		t.Errorf("missing Skipped section, got:\n%s", summary)
	}
	if !strings.Contains(summary, "step-c") {
		t.Errorf("missing step-c in skipped section, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Re-run: adze apply") {
		t.Errorf("missing Re-run line, got:\n%s", summary)
	}
}

func TestSummaryAllFailed(t *testing.T) {
	result := &RunResult{
		StepResults: []StepRunResult{
			{Name: "step-a", Status: step.StatusFailed, Reason: "install failed"},
			{Name: "step-b", Status: step.StatusFailed, Reason: "build failed"},
		},
		ExitCode: 4,
	}

	summary := FormatSummary(result)

	if !strings.Contains(summary, "0 succeeded") {
		t.Errorf("expected 0 succeeded, got:\n%s", summary)
	}
	if !strings.Contains(summary, "2 failed") {
		t.Errorf("expected 2 failed, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Re-run: adze apply") {
		t.Errorf("missing Re-run line, got:\n%s", summary)
	}
}

func TestSummaryPartialBatch(t *testing.T) {
	result := &RunResult{
		StepResults: []StepRunResult{
			{
				Name:   "brew-packages",
				Status: step.StatusPartial,
				Items: []step.ItemResult{
					{Item: step.StepItem{Name: "git"}, Status: step.StatusApplied},
					{Item: step.StepItem{Name: "curl"}, Status: step.StatusSatisfied},
					{Item: step.StepItem{Name: "broken-pkg"}, Status: step.StatusFailed, Reason: "formula not found"},
				},
				LogPath: "/tmp/logs/brew-packages-broken-pkg.log",
			},
		},
		ExitCode: 5,
	}

	summary := FormatSummary(result)

	if !strings.Contains(summary, "brew-packages (partial: 2/3 items)") {
		t.Errorf("expected partial line with counts, got:\n%s", summary)
	}
	if !strings.Contains(summary, "broken-pkg") {
		t.Errorf("expected failed item listed, got:\n%s", summary)
	}
	if !strings.Contains(summary, "formula not found") {
		t.Errorf("expected error message, got:\n%s", summary)
	}
}

// --- Log file tests ---

func TestLogFileCreation(t *testing.T) {
	logDir := filepath.Join(os.TempDir(), "adze-test-logs-create")
	defer os.RemoveAll(logDir)

	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "install failed: exit code 1"}, nil
		},
	)

	r := sc.build()
	r.logDir = logDir

	result := r.Run(context.Background())

	if result.StepResults[0].LogPath == "" {
		t.Fatal("expected log path to be set for failed step")
	}

	expectedPath := filepath.Join(logDir, "step-a.log")
	if result.StepResults[0].LogPath != expectedPath {
		t.Errorf("expected log path %q, got %q", expectedPath, result.StepResults[0].LogPath)
	}

	// Verify the log file exists and has content.
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(content) == 0 {
		t.Error("log file should not be empty")
	}
}

func TestLogFileOverwrite(t *testing.T) {
	logDir := filepath.Join(os.TempDir(), "adze-test-logs-overwrite")
	defer os.RemoveAll(logDir)

	sc := newScenario()
	failReason := "first failure"
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: failReason}, nil
		},
	)

	r := sc.build()
	r.logDir = logDir

	// First run.
	r.Run(context.Background())
	logPath := filepath.Join(logDir, "step-a.log")
	content1, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("first run: failed to read log: %v", err)
	}

	// Second run with different failure reason.
	failReason = "second failure"
	// Rebuild the step with new failure.
	sc2 := newScenario()
	sc2.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "second failure"}, nil
		},
	)
	r2 := sc2.build()
	r2.logDir = logDir
	r2.Run(context.Background())

	content2, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("second run: failed to read log: %v", err)
	}

	if string(content1) == string(content2) {
		t.Error("log file should have been overwritten with different content")
	}
	if !strings.Contains(string(content2), "second failure") {
		t.Errorf("expected second failure message, got: %s", string(content2))
	}
}

func TestLogFileNotCreatedForSuccess(t *testing.T) {
	logDir := filepath.Join(os.TempDir(), "adze-test-logs-success")
	defer os.RemoveAll(logDir)

	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		nil,
	)

	r := sc.build()
	r.logDir = logDir
	result := r.Run(context.Background())

	if result.StepResults[0].LogPath != "" {
		t.Error("log path should be empty for successful step")
	}

	// Verify no log file was created.
	logPath := filepath.Join(logDir, "step-a.log")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("log file should not exist for successful step")
	}
}

// --- Log utility tests ---

func TestLogFilePath(t *testing.T) {
	tests := []struct {
		name     string
		logDir   string
		step     string
		item     string
		expected string
	}{
		{"atomic", "/tmp/logs", "homebrew", "", "/tmp/logs/homebrew.log"},
		{"batch item", "/tmp/logs", "brew-packages", "git", "/tmp/logs/brew-packages-git.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LogFilePath(tt.logDir, tt.step, tt.item)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestWriteLog(t *testing.T) {
	logDir := filepath.Join(os.TempDir(), "adze-test-writelog")
	defer os.RemoveAll(logDir)

	path := WriteLog(logDir, "test-step", "", "log content here")
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written log: %v", err)
	}
	if string(content) != "log content here" {
		t.Errorf("expected 'log content here', got %q", string(content))
	}
}

// --- Exit code computation tests ---

func TestComputeExitCode(t *testing.T) {
	tests := []struct {
		name     string
		results  []StepRunResult
		expected int
	}{
		{
			name:     "empty results",
			results:  nil,
			expected: 0,
		},
		{
			name: "all satisfied",
			results: []StepRunResult{
				{Status: step.StatusSatisfied},
				{Status: step.StatusSatisfied},
			},
			expected: 0,
		},
		{
			name: "all applied",
			results: []StepRunResult{
				{Status: step.StatusApplied},
				{Status: step.StatusApplied},
			},
			expected: 0,
		},
		{
			name: "mixed satisfied and applied",
			results: []StepRunResult{
				{Status: step.StatusSatisfied},
				{Status: step.StatusApplied},
			},
			expected: 0,
		},
		{
			name: "all failed (exit 4)",
			results: []StepRunResult{
				{Status: step.StatusFailed},
				{Status: step.StatusFailed},
			},
			expected: 4,
		},
		{
			name: "one failed only step (exit 4)",
			results: []StepRunResult{
				{Status: step.StatusFailed},
			},
			expected: 4,
		},
		{
			name: "some applied some failed (exit 5)",
			results: []StepRunResult{
				{Status: step.StatusApplied},
				{Status: step.StatusFailed},
			},
			expected: 5,
		},
		{
			name: "partial counts as both applied and failed (exit 5)",
			results: []StepRunResult{
				{Status: step.StatusPartial},
			},
			expected: 5,
		},
		{
			name: "verify_failed is a failure",
			results: []StepRunResult{
				{Status: step.StatusVerifyFailed},
			},
			expected: 4,
		},
		{
			name: "applied + verify_failed (exit 5)",
			results: []StepRunResult{
				{Status: step.StatusApplied},
				{Status: step.StatusVerifyFailed},
			},
			expected: 5,
		},
		{
			name: "skipped only (exit 0)",
			results: []StepRunResult{
				{Status: step.StatusSkipped},
			},
			expected: 0,
		},
		{
			name: "satisfied + skipped (exit 0)",
			results: []StepRunResult{
				{Status: step.StatusSatisfied},
				{Status: step.StatusSkipped},
			},
			expected: 0,
		},
		{
			name: "failed + skipped (exit 4)",
			results: []StepRunResult{
				{Status: step.StatusFailed},
				{Status: step.StatusSkipped},
			},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeExitCode(tt.results)
			if got != tt.expected {
				t.Errorf("expected exit code %d, got %d", tt.expected, got)
			}
		})
	}
}

// --- Duration test ---

func TestRunRecordsDuration(t *testing.T) {
	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			time.Sleep(10 * time.Millisecond)
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		nil,
	)

	r := sc.build()
	result := r.Run(context.Background())

	if result.StepResults[0].Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

// --- makeEnvChecker test ---

func TestMakeEnvCheckerNilSecrets(t *testing.T) {
	checker := makeEnvChecker(nil)
	if checker != nil {
		t.Error("expected nil env checker for nil secrets manager")
	}
}

// --- buildStepConfig test ---

func TestBuildStepConfig(t *testing.T) {
	rs := dag.ResolvedStep{
		Name: "test-step",
		Config: dag.StepInput{
			Name:     "test-step",
			Provides: []string{"cap-a"},
			Requires: []string{"cap-b"},
		},
	}

	cfg := buildStepConfig(rs)

	if cfg.Name != "test-step" {
		t.Errorf("expected name 'test-step', got %q", cfg.Name)
	}
	if len(cfg.Provides) != 1 || cfg.Provides[0] != "cap-a" {
		t.Errorf("expected provides [cap-a], got %v", cfg.Provides)
	}
	if len(cfg.Requires) != 1 || cfg.Requires[0] != "cap-b" {
		t.Errorf("expected requires [cap-b], got %v", cfg.Requires)
	}
}

// --- Multiple independent steps all pass ---

func TestRunMultipleIndependentSteps(t *testing.T) {
	checkCount := map[string]int{}
	sc := newScenario()
	for _, name := range []string{"step-a", "step-b", "step-c"} {
		n := name // capture
		sc.addStep(n, nil, nil,
			func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
				checkCount[cfg.Name]++
				if checkCount[cfg.Name] == 1 {
					return step.StepResult{Status: step.StatusFailed, Reason: "not done"}, nil
				}
				return step.StepResult{Status: step.StatusSatisfied}, nil
			},
			func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
				return step.StepResult{Status: step.StatusApplied}, nil
			},
		)
	}

	r := sc.build()
	result := r.Run(context.Background())

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	for _, sr := range result.StepResults {
		if sr.Status != step.StatusApplied {
			t.Errorf("%s: expected applied, got %s", sr.Name, sr.Status)
		}
	}
}

// --- Callback interface tests ---

func TestCallbacksFireInOrder(t *testing.T) {
	checkCount := map[string]int{}
	sc := newScenario()
	sc.addStep("step-a", []string{"cap-a"}, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			checkCount[cfg.Name]++
			if checkCount[cfg.Name] == 1 {
				return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
			}
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusApplied}, nil
		},
	)
	sc.addStep("step-b", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		nil,
	)
	sc.addStep("step-c", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			checkCount[cfg.Name]++
			if checkCount[cfg.Name] == 1 {
				return step.StepResult{Status: step.StatusFailed, Reason: "not done"}, nil
			}
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusApplied}, nil
		},
	)

	r := sc.build()

	type callbackEvent struct {
		kind  string // "start" or "complete"
		name  string
		index int
		total int
	}
	var events []callbackEvent

	r.OnStepStart = func(stepName string, index int, total int) {
		events = append(events, callbackEvent{"start", stepName, index, total})
	}
	r.OnStepComplete = func(stepName string, index int, total int, result step.StepResult) {
		events = append(events, callbackEvent{"complete", stepName, index, total})
	}

	r.Run(context.Background())

	// Expect 6 events: start/complete for each of 3 steps, in order.
	if len(events) != 6 {
		t.Fatalf("expected 6 callback events, got %d", len(events))
	}

	expected := []callbackEvent{
		{"start", "step-a", 1, 3},
		{"complete", "step-a", 1, 3},
		{"start", "step-b", 2, 3},
		{"complete", "step-b", 2, 3},
		{"start", "step-c", 3, 3},
		{"complete", "step-c", 3, 3},
	}

	for i, ev := range events {
		if ev != expected[i] {
			t.Errorf("event %d: expected %+v, got %+v", i, expected[i], ev)
		}
	}
}

func TestCallbacksFireForSkippedSteps(t *testing.T) {
	sc := newScenario()
	// step-a fails, provides cap-a
	sc.addStep("step-a", []string{"cap-a"}, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "not installed"}, nil
		},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusFailed, Reason: "install failed"}, nil
		},
	)
	// step-b requires cap-a — should be skipped
	sc.addStep("step-b", nil, []string{"cap-a"},
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			t.Fatal("step-b check should not be called")
			return step.StepResult{}, nil
		},
		nil,
	)

	r := sc.build()

	type callbackEvent struct {
		kind   string
		name   string
		index  int
		total  int
		status step.StepStatus
	}
	var events []callbackEvent

	r.OnStepStart = func(stepName string, index int, total int) {
		events = append(events, callbackEvent{"start", stepName, index, total, ""})
	}
	r.OnStepComplete = func(stepName string, index int, total int, result step.StepResult) {
		events = append(events, callbackEvent{"complete", stepName, index, total, result.Status})
	}

	r.Run(context.Background())

	// Expect 4 events: start/complete for step-a (failed) and step-b (skipped).
	if len(events) != 4 {
		t.Fatalf("expected 4 callback events, got %d", len(events))
	}

	// Verify skipped step got both callbacks.
	if events[2].kind != "start" || events[2].name != "step-b" {
		t.Errorf("expected start callback for step-b, got %+v", events[2])
	}
	if events[3].kind != "complete" || events[3].name != "step-b" || events[3].status != step.StatusSkipped {
		t.Errorf("expected complete callback for step-b with skipped status, got %+v", events[3])
	}
}

func TestNilCallbacksDoNotPanic(t *testing.T) {
	sc := newScenario()
	sc.addStep("step-a", nil, nil,
		func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
			return step.StepResult{Status: step.StatusSatisfied}, nil
		},
		nil,
	)

	r := sc.build()
	// Ensure OnStepStart and OnStepComplete are nil (default).
	r.OnStepStart = nil
	r.OnStepComplete = nil

	// This should not panic.
	result := r.Run(context.Background())
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestCallbackIndexAndTotal(t *testing.T) {
	sc := newScenario()
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		sc.addStep(name, nil, nil,
			func(ctx context.Context, cfg step.StepConfig) (step.StepResult, error) {
				return step.StepResult{Status: step.StatusSatisfied}, nil
			},
			nil,
		)
	}

	r := sc.build()

	type indexRecord struct {
		name  string
		index int
		total int
	}
	var starts []indexRecord
	var completes []indexRecord

	r.OnStepStart = func(stepName string, index int, total int) {
		starts = append(starts, indexRecord{stepName, index, total})
	}
	r.OnStepComplete = func(stepName string, index int, total int, result step.StepResult) {
		completes = append(completes, indexRecord{stepName, index, total})
	}

	r.Run(context.Background())

	if len(starts) != 4 {
		t.Fatalf("expected 4 start callbacks, got %d", len(starts))
	}
	if len(completes) != 4 {
		t.Fatalf("expected 4 complete callbacks, got %d", len(completes))
	}

	names := []string{"alpha", "beta", "gamma", "delta"}
	for i, name := range names {
		expectedIndex := i + 1 // 1-based
		expectedTotal := 4

		if starts[i].name != name || starts[i].index != expectedIndex || starts[i].total != expectedTotal {
			t.Errorf("start[%d]: expected {%s %d %d}, got %+v", i, name, expectedIndex, expectedTotal, starts[i])
		}
		if completes[i].name != name || completes[i].index != expectedIndex || completes[i].total != expectedTotal {
			t.Errorf("complete[%d]: expected {%s %d %d}, got %+v", i, name, expectedIndex, expectedTotal, completes[i])
		}
	}
}

// --- Step not found in step list ---

func TestRunStepNotFound(t *testing.T) {
	graph := &dag.ResolvedGraph{
		Steps: []dag.ResolvedStep{
			{
				Name: "missing-step",
				Config: dag.StepInput{
					Name:     "missing-step",
					Provides: []string{"cap-m"},
				},
			},
		},
	}

	r := NewRunner(nil, graph, nil, "darwin")
	r.logDir = filepath.Join(os.TempDir(), "adze-test-logs-notfound")
	defer os.RemoveAll(r.logDir)

	result := r.Run(context.Background())

	if result.StepResults[0].Status != step.StatusFailed {
		t.Errorf("expected failed for missing step, got %s", result.StepResults[0].Status)
	}
	if !strings.Contains(result.StepResults[0].Reason, "not found") {
		t.Errorf("expected reason to mention 'not found', got %q", result.StepResults[0].Reason)
	}
}
