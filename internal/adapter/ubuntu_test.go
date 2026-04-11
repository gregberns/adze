package adapter

import (
	"errors"
	"fmt"
	"testing"
)

// --- Ubuntu Package Check Tests ---

func TestUbuntuPackageCheck_Installed(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "dpkg-query", out: "install ok installed"},
	})
	u := NewUbuntuAdapter(runner)
	installed, err := u.PackageCheck(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Fatal("expected package to be installed")
	}
}

func TestUbuntuPackageCheck_NotInstalled(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "dpkg-query", err: fmt.Errorf("exit 1")},
	})
	u := NewUbuntuAdapter(runner)
	installed, err := u.PackageCheck(Package{Name: "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatal("expected package to not be installed")
	}
}

func TestUbuntuPackageCheck_RemovedNotPurged(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "dpkg-query", out: "deinstall ok config-files"},
	})
	u := NewUbuntuAdapter(runner)
	installed, err := u.PackageCheck(Package{Name: "removed-pkg"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatal("expected package with deinstall status to not be installed")
	}
}

// --- Ubuntu Package Install Tests ---

func TestUbuntuPackageInstall(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
		{name: "sudo", out: ""},
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageInstall(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUbuntuPackageInstall_WithVersion(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
		{name: "apt-cache", out: "git | 1:2.34.1-1ubuntu1 | http://archive.ubuntu.com/ubuntu jammy/main amd64 Packages\n"},
		{name: "sudo", out: ""},
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageInstall(Package{Name: "git", Version: "1:2.34.1-1ubuntu1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUbuntuPackageInstall_VersionNotAvailable(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
		{name: "apt-cache", out: "git | 1:2.34.1-1ubuntu1 | http://archive.ubuntu.com/ubuntu jammy/main amd64 Packages\n"},
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageInstall(Package{Name: "git", Version: "9.9.9"})
	if !errors.Is(err, ErrVersionNotAvailable) {
		t.Fatalf("expected ErrVersionNotAvailable, got %v", err)
	}
}

func TestUbuntuPackageInstall_WithHold(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
		{name: "sudo", out: ""},  // apt install
		{name: "sudo", out: ""},  // apt-mark hold
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageInstall(Package{Name: "git", Pinned: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUbuntuPackageInstall_AptNotFound(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", err: fmt.Errorf("not found")},
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageInstall(Package{Name: "git"})
	if !errors.Is(err, ErrAptNotFound) {
		t.Fatalf("expected ErrAptNotFound, got %v", err)
	}
}

// --- Ubuntu Package Upgrade Tests ---

func TestUbuntuPackageUpgrade(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
		{name: "sudo", out: ""},
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageUpgrade(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUbuntuPackageUpgrade_Pinned(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageUpgrade(Package{Name: "git", Pinned: true})
	if !errors.Is(err, ErrPackagePinned) {
		t.Fatalf("expected ErrPackagePinned, got %v", err)
	}
}

// --- Ubuntu Package Remove Tests ---

func TestUbuntuPackageRemove(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
		{name: "sudo", out: ""},
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageRemove(Package{Name: "git"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUbuntuPackageRemove_Held(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "which", out: "/usr/bin/apt"},
		{name: "sudo", out: ""},  // apt-mark unhold
		{name: "sudo", out: ""},  // apt remove
	})
	u := NewUbuntuAdapter(runner)
	err := u.PackageRemove(Package{Name: "git", Pinned: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Ubuntu PackageList Tests ---

func TestUbuntuPackageList(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "apt-mark", out: "git\ncurl\nwget\n"},
	})
	u := NewUbuntuAdapter(runner)
	pkgs, err := u.PackageList()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(pkgs))
	}
}

// --- Ubuntu Defaults Tests ---

func TestUbuntuDefaultsRead_NotSupported(t *testing.T) {
	u := NewUbuntuAdapter(nil)
	_, err := u.DefaultsRead("domain", "key")
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestUbuntuDefaultsWrite_NotSupported(t *testing.T) {
	u := NewUbuntuAdapter(nil)
	err := u.DefaultsWrite("domain", "key", DefaultsValue{Type: "string", Raw: "val"})
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

// --- Ubuntu Service Tests ---

func TestUbuntuServiceEnable(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "sudo", out: ""},
	})
	u := NewUbuntuAdapter(runner)
	err := u.ServiceEnable("nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUbuntuServiceDisable(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "sudo", out: ""},
	})
	u := NewUbuntuAdapter(runner)
	err := u.ServiceDisable("nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Ubuntu Hostname Tests ---

func TestUbuntuSetHostname(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "sudo", out: ""},
	})
	u := NewUbuntuAdapter(runner)
	err := u.SetHostname("myhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Ubuntu ListLeaves Tests ---

func TestUbuntuListLeaves(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "apt-mark", out: "git\ncurl\nwget\n"},
	})
	u := NewUbuntuAdapter(runner)
	leaves, err := u.ListLeaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leaves) != 3 {
		t.Fatalf("expected 3 leaves, got %d", len(leaves))
	}
}

// --- Ubuntu ListAllInstalled Tests ---

func TestUbuntuListAllInstalled(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "dpkg-query", out: "git\t1:2.34.1\tii \ncurl\t7.81.0\tii \nheld-pkg\t1.0\thi \nremoved\t2.0\trc \n"},
	})
	u := NewUbuntuAdapter(runner)
	pkgs, err := u.ListAllInstalled()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 installed packages, got %d", len(pkgs))
	}
	// Verify held status.
	var foundHeld bool
	for _, p := range pkgs {
		if p.Name == "held-pkg" && p.Held {
			foundHeld = true
		}
	}
	if !foundHeld {
		t.Error("expected held-pkg to be marked as held")
	}
}

// --- Ubuntu Version Validation ---

func TestUbuntuValidateVersion(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "apt-cache", out: " git | 1:2.34.1-1ubuntu1 | http://archive.ubuntu.com jammy/main\n git | 1:2.30.2-1 | http://archive.ubuntu.com focal/main\n"},
	})
	u := NewUbuntuAdapter(runner)
	err := u.validateVersion("git", "1:2.34.1-1ubuntu1")
	if err != nil {
		t.Fatalf("expected version to be valid, got %v", err)
	}
}

func TestUbuntuValidateVersion_NotFound(t *testing.T) {
	runner := newMockRunner([]mockCall{
		{name: "apt-cache", out: " git | 1:2.34.1-1ubuntu1 | http://archive.ubuntu.com jammy/main\n"},
	})
	u := NewUbuntuAdapter(runner)
	err := u.validateVersion("git", "9.9.9")
	if !errors.Is(err, ErrVersionNotAvailable) {
		t.Fatalf("expected ErrVersionNotAvailable, got %v", err)
	}
}
