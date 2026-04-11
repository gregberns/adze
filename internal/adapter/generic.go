package adapter

import (
	"fmt"
	"os/exec"
	"strings"
)

// GenericAdapter implements platform-independent operations such as directory
// creation, git config, shell setup, and file operations.
type GenericAdapter struct {
	run commandRunner
}

// NewGenericAdapter creates a new generic adapter. The runner function is used
// for all command execution, enabling dependency injection for tests.
func NewGenericAdapter(runner commandRunner) *GenericAdapter {
	if runner == nil {
		runner = defaultRunner
	}
	return &GenericAdapter{run: runner}
}

// PackageInstall is not supported on the generic adapter.
func (g *GenericAdapter) PackageInstall(pkg Package) error {
	return ErrNotSupported
}

// PackageCheck is not supported on the generic adapter.
func (g *GenericAdapter) PackageCheck(pkg Package) (bool, error) {
	return false, ErrNotSupported
}

// PackageList is not supported on the generic adapter.
func (g *GenericAdapter) PackageList() ([]InstalledPackage, error) {
	return nil, ErrNotSupported
}

// PackageUpgrade is not supported on the generic adapter.
func (g *GenericAdapter) PackageUpgrade(pkg Package) error {
	return ErrNotSupported
}

// PackageRemove is not supported on the generic adapter.
func (g *GenericAdapter) PackageRemove(pkg Package) error {
	return ErrNotSupported
}

// DefaultsRead is not supported on the generic adapter.
func (g *GenericAdapter) DefaultsRead(domain, key string) (DefaultsValue, error) {
	return DefaultsValue{}, ErrNotSupported
}

// DefaultsWrite is not supported on the generic adapter.
func (g *GenericAdapter) DefaultsWrite(domain, key string, value DefaultsValue) error {
	return ErrNotSupported
}

// ServiceEnable is not supported on the generic adapter.
func (g *GenericAdapter) ServiceEnable(name string) error {
	return ErrNotSupported
}

// ServiceDisable is not supported on the generic adapter.
func (g *GenericAdapter) ServiceDisable(name string) error {
	return ErrNotSupported
}

// SetHostname is not supported on the generic adapter.
func (g *GenericAdapter) SetHostname(hostname string) error {
	return ErrNotSupported
}

// SetDefaultShell sets the user's default login shell.
// The shell must be an absolute path.
func (g *GenericAdapter) SetDefaultShell(shell string) error {
	_, err := g.run("chsh", "-s", shell)
	if err != nil {
		return fmt.Errorf("chsh -s %s failed: %w", shell, err)
	}
	return nil
}

// ListLeaves is not supported on the generic adapter.
func (g *GenericAdapter) ListLeaves() ([]string, error) {
	return nil, ErrNotSupported
}

// ListAllInstalled is not supported on the generic adapter.
func (g *GenericAdapter) ListAllInstalled() ([]InstalledPackage, error) {
	return nil, ErrNotSupported
}

// MakeDir creates a directory and all parent directories.
func (g *GenericAdapter) MakeDir(path string) error {
	_, err := g.run("mkdir", "-p", path)
	if err != nil {
		return fmt.Errorf("mkdir -p %s failed: %w", path, err)
	}
	return nil
}

// GitConfigSet sets a global git configuration value.
func (g *GenericAdapter) GitConfigSet(key, value string) error {
	_, err := g.run("git", "config", "--global", key, value)
	if err != nil {
		return fmt.Errorf("git config --global %s %s failed: %w", key, value, err)
	}
	return nil
}

// GitConfigGet reads a global git configuration value.
func (g *GenericAdapter) GitConfigGet(key string) (string, error) {
	out, err := g.run("git", "config", "--global", "--get", key)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", fmt.Errorf("git config key %s is not set", key)
		}
		return "", fmt.Errorf("git config --global --get %s failed: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Symlink creates or updates a symbolic link.
func (g *GenericAdapter) Symlink(target, link string) error {
	_, err := g.run("ln", "-sf", target, link)
	if err != nil {
		return fmt.Errorf("ln -sf %s %s failed: %w", target, link, err)
	}
	return nil
}

// CopyFile copies a file from src to dst.
func (g *GenericAdapter) CopyFile(src, dst string) error {
	_, err := g.run("cp", src, dst)
	if err != nil {
		return fmt.Errorf("cp %s %s failed: %w", src, dst, err)
	}
	return nil
}
