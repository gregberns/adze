package adapter

import (
	"testing"
)

func TestNewSudoManager(t *testing.T) {
	m := NewSudoManager()
	if m == nil {
		t.Fatal("expected non-nil SudoManager")
	}
	if m.IsAcquired() {
		t.Fatal("expected IsAcquired to be false initially")
	}
}

func TestSudoManagerNeedsPrivilege(t *testing.T) {
	m := NewSudoManager()

	privilegedOps := []string{
		"apt-install", "apt-remove", "apt-mark-hold", "apt-mark-unhold",
		"systemctl-enable", "systemctl-disable", "systemctl-start", "systemctl-stop",
		"hostnamectl", "scutil-set",
	}
	for _, op := range privilegedOps {
		if !m.NeedsPrivilege(op) {
			t.Errorf("expected %q to need privilege", op)
		}
	}

	unprivilegedOps := []string{
		"brew-install", "defaults-write", "mkdir", "git-config",
		"chsh", "ln", "cp", "",
	}
	for _, op := range unprivilegedOps {
		if m.NeedsPrivilege(op) {
			t.Errorf("expected %q to not need privilege", op)
		}
	}
}

func TestSudoManagerRelease_NoOp(t *testing.T) {
	m := NewSudoManager()
	// Release without acquire should not panic.
	m.Release()
	if m.IsAcquired() {
		t.Fatal("expected IsAcquired to be false after Release without Acquire")
	}
}
