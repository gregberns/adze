package step

import (
	"testing"
	"time"
)

func TestDefaultTimeouts(t *testing.T) {
	if DefaultCheckTimeout != 5*time.Minute {
		t.Errorf("expected DefaultCheckTimeout 5m, got %s", DefaultCheckTimeout)
	}
	if DefaultApplyTimeout != 15*time.Minute {
		t.Errorf("expected DefaultApplyTimeout 15m, got %s", DefaultApplyTimeout)
	}
	if PostSIGTERMGrace != 5*time.Second {
		t.Errorf("expected PostSIGTERMGrace 5s, got %s", PostSIGTERMGrace)
	}
}

func TestStepConfigDefaults(t *testing.T) {
	cfg := StepConfig{
		Name:        "test-step",
		Description: "A test step",
		Tags:        []string{"test"},
		Provides:    []string{"test-capability"},
		Requires:    []string{"other"},
		Platforms:   []string{"darwin"},
		Check:       &ShellCommand{Args: []string{"true"}},
		Apply:       &ShellCommand{Args: []string{"echo", "applied"}},
		Env:         []string{"FOO=bar"},
	}

	if cfg.Name != "test-step" {
		t.Errorf("expected name test-step, got %s", cfg.Name)
	}
	if cfg.Check == nil {
		t.Fatal("expected non-nil Check")
	}
	if cfg.Check.Args[0] != "true" {
		t.Errorf("expected 'true', got %s", cfg.Check.Args[0])
	}

	// CheckTimeout/ApplyTimeout default to 0 (executor applies defaults).
	if cfg.CheckTimeout != 0 {
		t.Errorf("expected zero CheckTimeout, got %s", cfg.CheckTimeout)
	}
	if cfg.ApplyTimeout != 0 {
		t.Errorf("expected zero ApplyTimeout, got %s", cfg.ApplyTimeout)
	}
}

func TestShellCommand(t *testing.T) {
	cmd := ShellCommand{
		Args: []string{"brew", "install", "git"},
		Env:  map[string]string{"HOMEBREW_NO_AUTO_UPDATE": "1"},
	}

	if cmd.Args[0] != "brew" {
		t.Errorf("expected executable 'brew', got %s", cmd.Args[0])
	}
	if len(cmd.Args) != 3 {
		t.Errorf("expected 3 args, got %d", len(cmd.Args))
	}
	if cmd.Env["HOMEBREW_NO_AUTO_UPDATE"] != "1" {
		t.Errorf("expected env var, got %v", cmd.Env)
	}
}

func TestStepItem(t *testing.T) {
	item := StepItem{
		Name:    "git",
		Version: "2.40.0",
		Pinned:  true,
	}

	if item.Name != "git" {
		t.Errorf("expected name 'git', got %s", item.Name)
	}
	if item.Version != "2.40.0" {
		t.Errorf("expected version '2.40.0', got %s", item.Version)
	}
	if !item.Pinned {
		t.Error("expected pinned to be true")
	}
}

func TestRollbackComment(t *testing.T) {
	// Verify rollback field exists but is documented as not executed in v1.
	cfg := StepConfig{
		Rollback: &ShellCommand{Args: []string{"rollback-cmd"}},
	}
	if cfg.Rollback == nil {
		t.Fatal("expected non-nil Rollback")
	}
	// Rollback is persisted but never executed — this is by design.
}

func TestPlatformApply(t *testing.T) {
	cfg := StepConfig{
		Apply: &ShellCommand{Args: []string{"generic-apply"}},
		PlatformApply: map[string]*ShellCommand{
			"darwin": {Args: []string{"darwin-apply"}},
			"ubuntu": {Args: []string{"ubuntu-apply"}},
		},
	}

	if cfg.PlatformApply["darwin"].Args[0] != "darwin-apply" {
		t.Errorf("expected darwin-apply, got %s", cfg.PlatformApply["darwin"].Args[0])
	}
	if cfg.PlatformApply["ubuntu"].Args[0] != "ubuntu-apply" {
		t.Errorf("expected ubuntu-apply, got %s", cfg.PlatformApply["ubuntu"].Args[0])
	}
}
