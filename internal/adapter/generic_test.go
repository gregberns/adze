package adapter

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// --- Generic Adapter Unsupported Operations ---

func TestGenericPackageInstall_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	err := g.PackageInstall(Package{Name: "git"})
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericPackageCheck_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	_, err := g.PackageCheck(Package{Name: "git"})
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericPackageList_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	_, err := g.PackageList()
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericPackageUpgrade_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	err := g.PackageUpgrade(Package{Name: "git"})
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericPackageRemove_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	err := g.PackageRemove(Package{Name: "git"})
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericDefaultsRead_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	_, err := g.DefaultsRead("domain", "key")
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericDefaultsWrite_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	err := g.DefaultsWrite("domain", "key", DefaultsValue{})
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericServiceEnable_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	err := g.ServiceEnable("svc")
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericServiceDisable_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	err := g.ServiceDisable("svc")
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericSetHostname_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	err := g.SetHostname("myhost")
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericListLeaves_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	_, err := g.ListLeaves()
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestGenericListAllInstalled_NotSupported(t *testing.T) {
	g := NewGenericAdapter(nil)
	_, err := g.ListAllInstalled()
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

// --- Generic Adapter Real Operations ---
// These use real commands since they're safe (tmp dirs, git config).

func TestGenericMakeDir(t *testing.T) {
	g := NewGenericAdapter(defaultRunner)
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	err := g.MakeDir(dir)
	if err != nil {
		t.Fatalf("MakeDir failed: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

func TestGenericMakeDir_Idempotent(t *testing.T) {
	g := NewGenericAdapter(defaultRunner)
	dir := filepath.Join(t.TempDir(), "existing")
	// Create once.
	if err := g.MakeDir(dir); err != nil {
		t.Fatalf("first MakeDir failed: %v", err)
	}
	// Create again (idempotent).
	if err := g.MakeDir(dir); err != nil {
		t.Fatalf("second MakeDir failed: %v", err)
	}
}

func TestGenericSymlink(t *testing.T) {
	g := NewGenericAdapter(defaultRunner)
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}

	link := filepath.Join(tmpDir, "link.txt")
	err := g.Symlink(target, link)
	if err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	resolved, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if resolved != target {
		t.Fatalf("expected link to point to %s, got %s", target, resolved)
	}
}

func TestGenericSymlink_Overwrite(t *testing.T) {
	g := NewGenericAdapter(defaultRunner)
	tmpDir := t.TempDir()

	target1 := filepath.Join(tmpDir, "target1.txt")
	target2 := filepath.Join(tmpDir, "target2.txt")
	if err := os.WriteFile(target1, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target2, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(tmpDir, "link.txt")
	// Create initial link.
	if err := g.Symlink(target1, link); err != nil {
		t.Fatal(err)
	}
	// Overwrite with new target.
	if err := g.Symlink(target2, link); err != nil {
		t.Fatal(err)
	}

	resolved, _ := os.Readlink(link)
	if resolved != target2 {
		t.Fatalf("expected link to point to %s, got %s", target2, resolved)
	}
}

func TestGenericCopyFile(t *testing.T) {
	g := NewGenericAdapter(defaultRunner)
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")
	content := []byte("file content")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	err := g.CopyFile(src, dst)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("expected %q, got %q", content, got)
	}
}

func TestGenericGitConfig(t *testing.T) {
	g := NewGenericAdapter(defaultRunner)

	// Use a test-specific key to avoid messing with real git config.
	key := "adze-test.adapter-test-key"
	value := "adapter-test-value"

	err := g.GitConfigSet(key, value)
	if err != nil {
		t.Fatalf("GitConfigSet failed: %v", err)
	}

	got, err := g.GitConfigGet(key)
	if err != nil {
		t.Fatalf("GitConfigGet failed: %v", err)
	}
	if got != value {
		t.Fatalf("expected %q, got %q", value, got)
	}

	// Clean up: unset the test key.
	_, _ = defaultRunner("git", "config", "--global", "--unset", key)
}

func TestGenericGitConfigGet_NotSet(t *testing.T) {
	g := NewGenericAdapter(defaultRunner)
	_, err := g.GitConfigGet("adze-test.nonexistent-key-that-should-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent git config key")
	}
}

// --- Interface Compliance ---

func TestDarwinAdapterImplementsAdapter(t *testing.T) {
	var _ Adapter = (*DarwinAdapter)(nil)
}

func TestUbuntuAdapterImplementsAdapter(t *testing.T) {
	var _ Adapter = (*UbuntuAdapter)(nil)
}

func TestGenericAdapterImplementsAdapter(t *testing.T) {
	var _ Adapter = (*GenericAdapter)(nil)
}
