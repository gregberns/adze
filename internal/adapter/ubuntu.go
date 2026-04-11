package adapter

import (
	"fmt"
	"strings"
)

// UbuntuAdapter implements the Adapter interface for Ubuntu/Debian systems
// using apt, dpkg, systemctl, and hostnamectl.
type UbuntuAdapter struct {
	run commandRunner
}

// NewUbuntuAdapter creates a new Ubuntu/Debian adapter. The runner function is
// used for all command execution, enabling dependency injection for tests.
func NewUbuntuAdapter(runner commandRunner) *UbuntuAdapter {
	if runner == nil {
		runner = defaultRunner
	}
	return &UbuntuAdapter{run: runner}
}

// checkAptAvailable verifies that apt is in PATH.
func (u *UbuntuAdapter) checkAptAvailable() error {
	_, err := u.run("which", "apt")
	if err != nil {
		return ErrAptNotFound
	}
	return nil
}

// PackageCheck checks whether a package is installed via dpkg-query.
func (u *UbuntuAdapter) PackageCheck(pkg Package) (bool, error) {
	out, err := u.run("dpkg-query", "-W", "-f=${Status}", pkg.Name)
	if err != nil {
		// Exit 1 means package unknown to dpkg.
		return false, nil
	}
	return strings.Contains(string(out), "install ok installed"), nil
}

// PackageInstall installs a package via apt.
func (u *UbuntuAdapter) PackageInstall(pkg Package) error {
	if err := u.checkAptAvailable(); err != nil {
		return err
	}

	installArg := pkg.Name
	if pkg.Version != "" {
		// Validate the version is available before attempting install.
		if err := u.validateVersion(pkg.Name, pkg.Version); err != nil {
			return err
		}
		installArg = pkg.Name + "=" + pkg.Version
	}

	_, err := u.run("sudo", "apt", "install", "-y", installArg)
	if err != nil {
		return fmt.Errorf("apt install %s failed: %w", installArg, err)
	}

	// Hold the package if pinned.
	if pkg.Pinned {
		_, err := u.run("sudo", "apt-mark", "hold", pkg.Name)
		if err != nil {
			return fmt.Errorf("apt-mark hold %s failed: %w", pkg.Name, err)
		}
	}

	return nil
}

// PackageUpgrade upgrades a package via apt.
func (u *UbuntuAdapter) PackageUpgrade(pkg Package) error {
	if err := u.checkAptAvailable(); err != nil {
		return err
	}

	if pkg.Pinned {
		return ErrPackagePinned
	}

	_, err := u.run("sudo", "apt", "install", "-y", "--only-upgrade", pkg.Name)
	if err != nil {
		return fmt.Errorf("apt upgrade %s failed: %w", pkg.Name, err)
	}
	return nil
}

// PackageRemove removes a package via apt.
func (u *UbuntuAdapter) PackageRemove(pkg Package) error {
	if err := u.checkAptAvailable(); err != nil {
		return err
	}

	// Unhold before removing if the package is held.
	if pkg.Pinned {
		_, _ = u.run("sudo", "apt-mark", "unhold", pkg.Name)
	}

	_, err := u.run("sudo", "apt", "remove", "-y", pkg.Name)
	if err != nil {
		return fmt.Errorf("apt remove %s failed: %w", pkg.Name, err)
	}
	return nil
}

// PackageList returns the list of manually installed packages.
func (u *UbuntuAdapter) PackageList() ([]InstalledPackage, error) {
	out, err := u.run("apt-mark", "showmanual")
	if err != nil {
		return nil, fmt.Errorf("apt-mark showmanual failed: %w", err)
	}

	var result []InstalledPackage
	for _, name := range splitNonEmpty(string(out)) {
		result = append(result, InstalledPackage{Name: name})
	}
	return result, nil
}

// DefaultsRead is not supported on Ubuntu/Debian.
func (u *UbuntuAdapter) DefaultsRead(domain, key string) (DefaultsValue, error) {
	return DefaultsValue{}, ErrNotSupported
}

// DefaultsWrite is not supported on Ubuntu/Debian.
func (u *UbuntuAdapter) DefaultsWrite(domain, key string, value DefaultsValue) error {
	return ErrNotSupported
}

// ServiceEnable enables a service via systemctl.
func (u *UbuntuAdapter) ServiceEnable(name string) error {
	_, err := u.run("sudo", "systemctl", "enable", name)
	if err != nil {
		return fmt.Errorf("systemctl enable %s failed: %w", name, err)
	}
	return nil
}

// ServiceDisable disables a service via systemctl.
func (u *UbuntuAdapter) ServiceDisable(name string) error {
	_, err := u.run("sudo", "systemctl", "disable", name)
	if err != nil {
		return fmt.Errorf("systemctl disable %s failed: %w", name, err)
	}
	return nil
}

// SetHostname sets the hostname via hostnamectl.
func (u *UbuntuAdapter) SetHostname(hostname string) error {
	_, err := u.run("sudo", "hostnamectl", "set-hostname", hostname)
	if err != nil {
		return fmt.Errorf("hostnamectl set-hostname failed: %w", err)
	}
	return nil
}

// SetDefaultShell sets the user's default shell via chsh.
func (u *UbuntuAdapter) SetDefaultShell(shell string) error {
	_, err := u.run("chsh", "-s", shell)
	if err != nil {
		return fmt.Errorf("chsh -s %s failed: %w", shell, err)
	}
	return nil
}

// ListLeaves returns the names of manually installed packages.
func (u *UbuntuAdapter) ListLeaves() ([]string, error) {
	out, err := u.run("apt-mark", "showmanual")
	if err != nil {
		return nil, fmt.Errorf("apt-mark showmanual failed: %w", err)
	}
	return splitNonEmpty(string(out)), nil
}

// ListAllInstalled returns all installed packages with version and hold status.
func (u *UbuntuAdapter) ListAllInstalled() ([]InstalledPackage, error) {
	out, err := u.run("dpkg-query", "-W", "-f=${Package}\t${Version}\t${db:Status-Abbrev}\n")
	if err != nil {
		return nil, fmt.Errorf("dpkg-query failed: %w", err)
	}

	var result []InstalledPackage
	for _, line := range splitLines(string(out)) {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		status := parts[2]
		// Filter for installed packages: status starts with 'i' or 'h'.
		if len(status) > 0 && (status[0] == 'i' || status[0] == 'h') {
			result = append(result, InstalledPackage{
				Name:    parts[0],
				Version: parts[1],
				Held:    status[0] == 'h',
			})
		}
	}

	return result, nil
}

// validateVersion checks that the specified version is available in apt-cache madison output.
func (u *UbuntuAdapter) validateVersion(name, version string) error {
	out, err := u.run("apt-cache", "madison", name)
	if err != nil {
		return fmt.Errorf("apt-cache madison %s failed: %w", name, err)
	}

	for _, line := range splitLines(string(out)) {
		// Format: <name> | <version> | <source>
		parts := strings.Split(line, "|")
		if len(parts) >= 2 {
			available := strings.TrimSpace(parts[1])
			if available == version {
				return nil
			}
		}
	}

	return fmt.Errorf("%w: %s version %s", ErrVersionNotAvailable, name, version)
}
