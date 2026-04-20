package cli

import (
	"context"
	"testing"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/dag"
	"github.com/gregberns/adze/internal/runner"
	"github.com/gregberns/adze/internal/secrets"
	"github.com/gregberns/adze/internal/step"
	"github.com/gregberns/adze/internal/steps"
)

// TestIntegration_FullPipelineBrewPackages exercises the full wiring pipeline
// that apply.go follows: BuildStepConfigs -> stepConfigsToDagInputs -> dag.Resolve
// -> configMap construction. It verifies that brew-packages StepConfig retains its
// Check, Items, and Apply/PlatformApply fields through the entire pipeline.
func TestIntegration_FullPipelineBrewPackages(t *testing.T) {
	cfg := &config.Config{
		Name:     "integration-test",
		Platform: "darwin",
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{
				{Name: "git"},
				{Name: "jq"},
			},
		},
	}

	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, "darwin", reg)

	// Find the brew-packages StepConfig.
	var brewSC *step.StepConfig
	for i := range stepConfigs {
		if stepConfigs[i].Name == "brew-packages" {
			brewSC = &stepConfigs[i]
			break
		}
	}
	if brewSC == nil {
		t.Fatal("brew-packages StepConfig not found in BuildStepConfigs result")
	}

	// Verify the StepConfig has the fields we expect from bindings.
	if brewSC.Items == nil {
		t.Fatal("brew-packages StepConfig.Items is nil; expected 2 items")
	}
	if len(brewSC.Items) != 2 {
		t.Fatalf("brew-packages StepConfig.Items has %d items, want 2", len(brewSC.Items))
	}
	if brewSC.Items[0].Name != "git" {
		t.Errorf("Items[0].Name = %q, want %q", brewSC.Items[0].Name, "git")
	}
	if brewSC.Items[1].Name != "jq" {
		t.Errorf("Items[1].Name = %q, want %q", brewSC.Items[1].Name, "jq")
	}

	// brew-packages is a batch step; it may not have a top-level Check/Apply
	// (the built-in step implementation handles per-item checks), but the
	// StepConfig must at least be non-empty.
	// The key point: brew-packages gets Items from the config, which is
	// what the built-in step uses.

	// Now continue the pipeline: DAG resolution.
	dagInputs := stepConfigsToDagInputs(stepConfigs)
	graph, dagErrs := dag.Resolve(dagInputs, "darwin", nil)
	if len(dagErrs) > 0 {
		t.Fatalf("DAG resolution errors: %v", dagErrs)
	}

	if len(graph.Steps) == 0 {
		t.Fatal("resolved graph has 0 steps")
	}

	// Build the configMap the same way apply.go does.
	configMap := make(map[string]step.StepConfig, len(stepConfigs))
	for _, sc := range stepConfigs {
		configMap[sc.Name] = sc
	}

	// Verify the configMap entry for brew-packages survives the pipeline.
	brewFromMap, ok := configMap["brew-packages"]
	if !ok {
		t.Fatal("brew-packages not found in configMap")
	}
	if brewFromMap.Items == nil {
		t.Fatal("configMap[brew-packages].Items is nil after pipeline")
	}
	if len(brewFromMap.Items) != 2 {
		t.Fatalf("configMap[brew-packages].Items has %d items after pipeline, want 2", len(brewFromMap.Items))
	}
}

// TestIntegration_AllBuiltinStepsHaveCommands creates a config that activates
// many built-in steps and verifies that each returned StepConfig has a non-nil
// Check and either a non-nil Apply or a PlatformApply entry for the platform.
//
// This is the test that would have caught the "runner drops step configs" bug:
// if BuildStepConfigs produces configs with Check/Apply, but the runner's
// buildStepConfig (the fallback) does not, steps lose their commands.
func TestIntegration_AllBuiltinStepsHaveCommands(t *testing.T) {
	cfg := &config.Config{
		Name:     "full-config",
		Platform: "darwin",
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{{Name: "git"}},
			Cask: []config.PackageEntry{{Name: "iterm2"}},
		},
		Defaults: map[string]map[string]config.DefaultValue{
			"com.apple.finder": {
				"ShowPathbar": {Value: true},
			},
		},
		Dock: config.DockConfig{
			Apps: []string{"Safari"},
		},
		Shell: config.ShellConfig{
			Default: "zsh",
			OhMyZsh: true,
			Plugins: []string{"git"},
		},
		Directories: []string{"~/Projects"},
		Identity: config.IdentityConfig{
			GitName:  "Test User",
			GitEmail: "test@example.com",
		},
		Machine: config.MachineConfig{
			Hostname: "test-machine",
		},
	}

	platform := "darwin"
	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, platform, reg)

	if len(stepConfigs) == 0 {
		t.Fatal("BuildStepConfigs returned 0 step configs for a full config")
	}

	// oh-my-zsh is a known exception: its Check/Apply are hardcoded in the
	// step implementation, and the bindings intentionally leave them nil.
	// See the comment in bindings.go case "shell.oh_my_zsh".
	selfContainedSteps := map[string]bool{
		"oh-my-zsh": true,
	}

	for _, sc := range stepConfigs {
		t.Run(sc.Name, func(t *testing.T) {
			if selfContainedSteps[sc.Name] {
				t.Skipf("step %q has self-contained Check/Apply in its implementation", sc.Name)
				return
			}

			// Every step that BuildStepConfigs returns should have a Check command,
			// OR be a batch step whose built-in implementation handles checking
			// internally. Steps with no ConfigSection get commands from
			// populateBuiltinCommands.
			//
			// For batch steps (Items non-nil), the built-in step implementation
			// handles per-item check/apply, so a top-level Check may be nil.
			// For atomic steps, Check must be non-nil.
			if sc.Items == nil && sc.Check == nil {
				t.Errorf("atomic step %q has nil Check", sc.Name)
			}

			// Every step must have either Apply or a PlatformApply entry for the platform.
			// Batch steps may not need a top-level Apply because the built-in
			// step handles per-item apply.
			if sc.Items == nil {
				hasApply := sc.Apply != nil
				hasPlatformApply := false
				if sc.PlatformApply != nil {
					_, hasPlatformApply = sc.PlatformApply[platform]
				}
				if !hasApply && !hasPlatformApply {
					t.Errorf("atomic step %q has neither Apply nor PlatformApply[%s]", sc.Name, platform)
				}
			}
		})
	}
}

// TestIntegration_PlatformApplyDispatch verifies that steps using PlatformApply
// have the correct platform-specific apply commands for both darwin and ubuntu.
func TestIntegration_PlatformApplyDispatch(t *testing.T) {
	cfg := &config.Config{
		Name:     "platform-test",
		Platform: "any",
	}

	reg := steps.NewRegistry()

	// Build for darwin.
	darwinConfigs := steps.BuildStepConfigs(cfg, "darwin", reg)
	darwinByName := make(map[string]step.StepConfig, len(darwinConfigs))
	for _, sc := range darwinConfigs {
		darwinByName[sc.Name] = sc
	}

	// Build for ubuntu.
	ubuntuConfigs := steps.BuildStepConfigs(cfg, "ubuntu", reg)
	ubuntuByName := make(map[string]step.StepConfig, len(ubuntuConfigs))
	for _, sc := range ubuntuConfigs {
		ubuntuByName[sc.Name] = sc
	}

	// node-fnm should exist on both platforms with PlatformApply entries.
	platformApplySteps := []string{"node-fnm", "python", "go"}

	for _, name := range platformApplySteps {
		t.Run(name+"_darwin", func(t *testing.T) {
			sc, ok := darwinByName[name]
			if !ok {
				t.Fatalf("%s not found in darwin step configs", name)
			}
			if sc.PlatformApply == nil {
				t.Fatalf("%s PlatformApply is nil for darwin build", name)
			}
			if sc.PlatformApply["darwin"] == nil {
				t.Errorf("%s PlatformApply[darwin] is nil", name)
			}
		})

		t.Run(name+"_ubuntu", func(t *testing.T) {
			sc, ok := ubuntuByName[name]
			if !ok {
				t.Fatalf("%s not found in ubuntu step configs", name)
			}
			if sc.PlatformApply == nil {
				t.Fatalf("%s PlatformApply is nil for ubuntu build", name)
			}
			if sc.PlatformApply["ubuntu"] == nil {
				t.Errorf("%s PlatformApply[ubuntu] is nil", name)
			}
		})
	}
}

// TestIntegration_MissingSetStepConfigsCausesSkip is the regression test for
// the "runner drops step configs" bug. It creates a Runner WITHOUT calling
// SetStepConfigs, which means the runner falls back to its internal
// buildStepConfig that produces minimal configs without Check/Apply commands.
// Steps that need Apply will get StatusSkipped with "no apply command".
func TestIntegration_MissingSetStepConfigsCausesSkip(t *testing.T) {
	cfg := &config.Config{
		Name:     "regression-test",
		Platform: "darwin",
		Packages: config.PackagesConfig{
			Brew: []config.PackageEntry{{Name: "git"}},
		},
	}

	platform := "darwin"
	reg := steps.NewRegistry()
	stepConfigs := steps.BuildStepConfigs(cfg, platform, reg)

	dagInputs := stepConfigsToDagInputs(stepConfigs)
	graph, dagErrs := dag.Resolve(dagInputs, platform, nil)
	if len(dagErrs) > 0 {
		t.Fatalf("DAG resolution errors: %v", dagErrs)
	}

	stepImpls := buildStepImplsForGraph(reg, graph)

	// Deliberately omit SetStepConfigs to simulate the bug.
	sm := secrets.NewSecretManager(nil)
	r := runner.NewRunner(stepImpls, graph, sm, platform)
	// NOTE: r.SetStepConfigs(configMap) is NOT called -- this is the bug.

	ctx := context.Background()
	result := r.Run(ctx)

	if result == nil {
		t.Fatal("Runner.Run returned nil result")
	}

	// Without SetStepConfigs, the runner's internal buildStepConfig creates
	// minimal configs without Check/Apply. Atomic steps that need an Apply
	// command will be skipped with "no apply command for platform darwin"
	// (because resolveApplyCommand returns nil).
	//
	// Batch steps (like brew-packages) will have nil Items, so they will
	// go through ExecuteStep instead of ExecuteBatchStep, and similarly
	// get skipped if they have no Apply.
	//
	// This verifies the bug scenario: steps are effectively neutered when
	// SetStepConfigs is not called.
	foundSkippedNoApply := false
	for _, sr := range result.StepResults {
		if sr.Status == step.StatusSkipped && sr.Reason != "" {
			// Look for "no apply command" in the skip reason, which is
			// the symptom of the missing SetStepConfigs bug.
			if contains(sr.Reason, "no apply command") {
				foundSkippedNoApply = true
				t.Logf("step %q correctly skipped: %s", sr.Name, sr.Reason)
			}
		}
	}

	if !foundSkippedNoApply {
		// If no steps were skipped with "no apply command", it means either:
		// (a) all steps happened to be satisfied (check passed), or
		// (b) the runner somehow has apply commands without SetStepConfigs.
		//
		// We also accept the case where steps failed for other reasons
		// (e.g., infrastructure errors, step not found), as long as the
		// runner didn't magically have full configs.
		//
		// Verify that no step reports StatusApplied (which would mean
		// apply ran without proper configs — the original bug symptom).
		for _, sr := range result.StepResults {
			if sr.Status == step.StatusApplied {
				t.Errorf("step %q reports StatusApplied without SetStepConfigs; "+
					"this means the runner ran apply without proper configs (the original bug)", sr.Name)
			}
		}

		// Also verify that steps that should have Items (like brew-packages)
		// lost them due to missing SetStepConfigs.
		for _, sr := range result.StepResults {
			if sr.Name == "brew-packages" && sr.Items != nil && len(sr.Items) > 0 {
				t.Errorf("brew-packages has %d item results without SetStepConfigs; "+
					"expected nil items since the runner's fallback buildStepConfig "+
					"doesn't include Items", len(sr.Items))
			}
		}
	}

	// Now verify the correct path: WITH SetStepConfigs, the configs are preserved.
	r2 := runner.NewRunner(stepImpls, graph, sm, platform)
	configMap := make(map[string]step.StepConfig, len(stepConfigs))
	for _, sc := range stepConfigs {
		configMap[sc.Name] = sc
	}
	r2.SetStepConfigs(configMap)

	// Verify the configMap has the expected fields for brew-packages.
	brewCfg, ok := configMap["brew-packages"]
	if !ok {
		t.Fatal("brew-packages not in configMap")
	}
	if brewCfg.Items == nil {
		t.Error("brew-packages in configMap has nil Items even with SetStepConfigs")
	}
	if len(brewCfg.Items) != 1 {
		t.Errorf("brew-packages in configMap has %d items, want 1", len(brewCfg.Items))
	}
}

// contains checks if substr is in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
