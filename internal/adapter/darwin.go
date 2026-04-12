package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// processRestartRegistry maps macOS preference domains to processes that must
// be restarted after a defaults write.
var processRestartRegistry = map[string]string{
	"com.apple.dock":            "Dock",
	"com.apple.finder":          "Finder",
	"com.apple.SystemUIServer":  "SystemUIServer",
}

// DarwinAdapter implements the Adapter interface for macOS using Homebrew
// and macOS system utilities.
type DarwinAdapter struct {
	run commandRunner
}

// NewDarwinAdapter creates a new Darwin adapter. The runner function is used
// for all command execution, enabling dependency injection for tests.
func NewDarwinAdapter(runner commandRunner) *DarwinAdapter {
	if runner == nil {
		runner = defaultRunner
	}
	return &DarwinAdapter{run: runner}
}

// defaultRunner executes a command using exec.Command.
func defaultRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// checkBrewAvailable verifies that brew is in PATH and the process is not root.
func (d *DarwinAdapter) checkBrewAvailable() error {
	if os.Geteuid() == 0 {
		return ErrBrewCalledAsRoot
	}
	_, err := d.run("which", "brew")
	if err != nil {
		return ErrBrewNotFound
	}
	return nil
}

// PackageCheck checks whether a package is installed.
func (d *DarwinAdapter) PackageCheck(pkg Package) (bool, error) {
	if err := d.checkBrewAvailable(); err != nil {
		return false, err
	}

	if pkg.Cask {
		_, err := d.run("brew", "list", "--cask", pkg.Name)
		if err != nil {
			return false, nil
		}
		return true, nil
	}

	name, err := formulaName(pkg)
	if err != nil {
		return false, err
	}
	_, err = d.run("brew", "list", "--formula", name)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// PackageInstall installs a package via Homebrew.
func (d *DarwinAdapter) PackageInstall(pkg Package) error {
	if err := d.checkBrewAvailable(); err != nil {
		return err
	}

	if pkg.Cask && pkg.Pinned {
		return ErrCaskPinNotSupported
	}

	if pkg.Cask {
		_, err := d.run("brew", "install", "--cask", pkg.Name)
		if err != nil {
			return fmt.Errorf("brew install --cask %s failed: %w", pkg.Name, err)
		}
		return nil
	}

	name, err := formulaName(pkg)
	if err != nil {
		return err
	}
	_, err = d.run("brew", "install", name)
	if err != nil {
		return fmt.Errorf("brew install %s failed: %w", name, err)
	}

	// Check for keg_only status after install.
	d.checkKegOnly(name)

	// Pin the formula if requested.
	if pkg.Pinned {
		_, err := d.run("brew", "pin", name)
		if err != nil {
			return fmt.Errorf("brew pin %s failed: %w", name, err)
		}
	}

	return nil
}

// checkKegOnly checks if a formula is keg-only and logs a warning.
func (d *DarwinAdapter) checkKegOnly(name string) {
	out, err := d.run("brew", "info", "--json", name)
	if err != nil {
		return
	}

	var info []struct {
		KegOnly bool `json:"keg_only"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return
	}
	if len(info) > 0 && info[0].KegOnly {
		// In production this would be emitted via the logging system.
		// For now, we write to stderr.
		fmt.Fprintf(os.Stderr, "WARNING: %s is keg-only; run 'brew link --force %s' or add it to PATH\n", name, name)
	}
}

// PackageUpgrade upgrades a package via Homebrew.
func (d *DarwinAdapter) PackageUpgrade(pkg Package) error {
	if err := d.checkBrewAvailable(); err != nil {
		return err
	}

	if pkg.Pinned {
		return ErrPackagePinned
	}

	if pkg.Cask {
		_, err := d.run("brew", "upgrade", "--cask", pkg.Name)
		if err != nil {
			return fmt.Errorf("brew upgrade --cask %s failed: %w", pkg.Name, err)
		}
		return nil
	}

	name, err := formulaName(pkg)
	if err != nil {
		return err
	}
	_, err = d.run("brew", "upgrade", name)
	if err != nil {
		return fmt.Errorf("brew upgrade %s failed: %w", name, err)
	}
	return nil
}

// PackageRemove removes a package via Homebrew.
func (d *DarwinAdapter) PackageRemove(pkg Package) error {
	if err := d.checkBrewAvailable(); err != nil {
		return err
	}

	if pkg.Cask {
		_, err := d.run("brew", "uninstall", "--cask", pkg.Name)
		if err != nil {
			return fmt.Errorf("brew uninstall --cask %s failed: %w", pkg.Name, err)
		}
		return nil
	}

	name, err := formulaName(pkg)
	if err != nil {
		return err
	}

	// Unpin before removing if pinned.
	if pkg.Pinned {
		_, _ = d.run("brew", "unpin", name)
	}

	_, err = d.run("brew", "uninstall", name)
	if err != nil {
		return fmt.Errorf("brew uninstall %s failed: %w", name, err)
	}
	return nil
}

// PackageList returns the list of installed leaf packages.
func (d *DarwinAdapter) PackageList() ([]InstalledPackage, error) {
	if err := d.checkBrewAvailable(); err != nil {
		return nil, err
	}

	out, err := d.run("brew", "leaves")
	if err != nil {
		return nil, fmt.Errorf("brew leaves failed: %w", err)
	}

	var result []InstalledPackage
	lines := splitLines(string(out))
	for _, name := range lines {
		if name == "" {
			continue
		}
		result = append(result, InstalledPackage{Name: name})
	}
	return result, nil
}

// DefaultsRead reads a macOS defaults preference value.
func (d *DarwinAdapter) DefaultsRead(domain, key string) (DefaultsValue, error) {
	// Read the value (stderr suppressed in production via the runner).
	rawValue, err := d.run("defaults", "read", domain, key)
	if err != nil {
		return DefaultsValue{}, fmt.Errorf("defaults read %s %s: key does not exist", domain, key)
	}

	// Read the type.
	rawType, err := d.run("defaults", "read-type", domain, key)
	if err != nil {
		return DefaultsValue{}, fmt.Errorf("defaults read-type %s %s failed: %w", domain, key, err)
	}

	typeName := parseDefaultsType(string(rawType))
	value := strings.TrimSpace(string(rawValue))

	return DefaultsValue{Type: typeName, Raw: value}, nil
}

// DefaultsWrite writes a macOS defaults preference value with post-write verification.
func (d *DarwinAdapter) DefaultsWrite(domain, key string, value DefaultsValue) error {
	// Build the write command arguments.
	writeArgs := []string{"write", domain, key}
	switch value.Type {
	case "string":
		writeArgs = append(writeArgs, "-string", value.Raw)
	case "boolean":
		writeArgs = append(writeArgs, "-bool", value.Raw)
	case "integer":
		writeArgs = append(writeArgs, "-int", value.Raw)
	case "float":
		writeArgs = append(writeArgs, "-float", value.Raw)
	default:
		return fmt.Errorf("unsupported defaults type: %s", value.Type)
	}

	// Execute the write.
	_, err := d.run("defaults", writeArgs...)
	if err != nil {
		return fmt.Errorf("defaults write failed: %w", err)
	}

	// Post-write verification: read back the type.
	rawType, err := d.run("defaults", "read-type", domain, key)
	if err != nil {
		return fmt.Errorf("defaults read-type verification failed: %w", err)
	}
	actualType := parseDefaultsType(string(rawType))
	if actualType != value.Type {
		return fmt.Errorf("%w: domain=%s key=%s declared=%s actual=%s",
			ErrDefaultsTypeMismatch, domain, key, value.Type, actualType)
	}

	// Post-write verification: read back the value.
	rawValue, err := d.run("defaults", "read", domain, key)
	if err != nil {
		return fmt.Errorf("defaults read verification failed: %w", err)
	}
	actualValue := strings.TrimSpace(string(rawValue))

	if !defaultsValuesEqual(value.Type, value.Raw, actualValue) {
		return fmt.Errorf("%w: domain=%s key=%s declared=%s actual=%s",
			ErrDefaultsValueMismatch, domain, key, value.Raw, actualValue)
	}

	// Process restart if required.
	if process, ok := processRestartRegistry[domain]; ok {
		_, _ = d.run("killall", process)
	}

	return nil
}

// ServiceEnable enables a service (Darwin: not yet implemented for v1 beyond the interface).
func (d *DarwinAdapter) ServiceEnable(name string) error {
	return ErrNotSupported
}

// ServiceDisable disables a service.
func (d *DarwinAdapter) ServiceDisable(name string) error {
	return ErrNotSupported
}

// SetHostname sets the macOS hostname (HostName, LocalHostName, ComputerName).
func (d *DarwinAdapter) SetHostname(hostname string) error {
	for _, kind := range []string{"HostName", "LocalHostName", "ComputerName"} {
		_, err := d.run("sudo", "scutil", "--set", kind, hostname)
		if err != nil {
			return fmt.Errorf("scutil --set %s failed: %w", kind, err)
		}
	}
	return nil
}

// SetDefaultShell sets the user's default shell.
func (d *DarwinAdapter) SetDefaultShell(shell string) error {
	_, err := d.run("chsh", "-s", shell)
	if err != nil {
		return fmt.Errorf("chsh -s %s failed: %w", shell, err)
	}
	return nil
}

// ListLeaves returns the names of leaf formulas and casks.
func (d *DarwinAdapter) ListLeaves() ([]string, error) {
	if err := d.checkBrewAvailable(); err != nil {
		return nil, err
	}

	// Get leaf formulas.
	out, err := d.run("brew", "leaves")
	if err != nil {
		return nil, fmt.Errorf("brew leaves failed: %w", err)
	}
	leaves := splitNonEmpty(string(out))

	// Get casks.
	caskOut, err := d.run("brew", "list", "--cask")
	if err != nil {
		return nil, fmt.Errorf("brew list --cask failed: %w", err)
	}
	casks := splitNonEmpty(string(caskOut))

	return append(leaves, casks...), nil
}

// ListAllInstalled returns all installed packages (formulas + casks).
func (d *DarwinAdapter) ListAllInstalled() ([]InstalledPackage, error) {
	if err := d.checkBrewAvailable(); err != nil {
		return nil, err
	}

	// Get formulas with versions.
	out, err := d.run("brew", "list", "--formula", "--versions")
	if err != nil {
		return nil, fmt.Errorf("brew list --formula --versions failed: %w", err)
	}

	var result []InstalledPackage
	for _, line := range splitLines(string(out)) {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			result = append(result, InstalledPackage{
				Name:    parts[0],
				Version: parts[1],
			})
		} else if len(parts) == 1 {
			result = append(result, InstalledPackage{Name: parts[0]})
		}
	}

	// Get casks with versions.
	caskOut, err := d.run("brew", "list", "--cask", "--versions")
	if err != nil {
		return nil, fmt.Errorf("brew list --cask --versions failed: %w", err)
	}
	for _, line := range splitLines(string(caskOut)) {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			result = append(result, InstalledPackage{
				Name:    parts[0],
				Version: parts[1],
			})
		} else if len(parts) == 1 {
			result = append(result, InstalledPackage{Name: parts[0]})
		}
	}

	return result, nil
}

// formulaName returns the appropriate formula name for Homebrew.
// If a version is specified, it appends @<version> (major version only).
// Returns ErrNoVersionedFormula if the version contains dots or is not
// a simple integer/major version string.
func formulaName(pkg Package) (string, error) {
	if pkg.Version == "" {
		return pkg.Name, nil
	}
	if strings.Contains(pkg.Version, ".") {
		return "", fmt.Errorf("%w: %s version %q", ErrNoVersionedFormula, pkg.Name, pkg.Version)
	}
	return pkg.Name + "@" + pkg.Version, nil
}

// parseDefaultsType extracts the type name from `defaults read-type` output.
// The output format is "Type is <type>", and we take the last word.
func parseDefaultsType(output string) string {
	output = strings.TrimSpace(output)
	parts := strings.Fields(output)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// defaultsValuesEqual compares two defaults values, applying boolean normalization.
func defaultsValuesEqual(typ, declared, actual string) bool {
	if typ == "boolean" {
		return normalizeBool(declared) == normalizeBool(actual)
	}
	return declared == actual
}

// normalizeBool normalizes boolean values: "true"/"1" -> "1", "false"/"0" -> "0".
func normalizeBool(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "true", "1":
		return "1"
	case "false", "0":
		return "0"
	}
	return v
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	return strings.Split(strings.TrimSpace(s), "\n")
}

// splitNonEmpty splits a string into lines and filters out empty ones.
func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
