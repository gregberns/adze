// Package adapter provides platform-specific implementations for system operations.
// At runtime, exactly one platform adapter (Darwin or Ubuntu) is active alongside
// the always-active generic adapter.
package adapter

import "errors"

// Adapter is the interface that all platform adapters must implement.
type Adapter interface {
	// Package operations
	PackageInstall(pkg Package) error
	PackageCheck(pkg Package) (bool, error) // true = already satisfied
	PackageList() ([]InstalledPackage, error)
	PackageUpgrade(pkg Package) error
	PackageRemove(pkg Package) error

	// Preference/defaults operations
	DefaultsRead(domain, key string) (DefaultsValue, error)
	DefaultsWrite(domain, key string, value DefaultsValue) error

	// System operations
	ServiceEnable(name string) error
	ServiceDisable(name string) error
	SetHostname(hostname string) error

	// Shell operations
	SetDefaultShell(shell string) error

	// Detection (for status, capture, init)
	ListLeaves() ([]string, error)
	ListAllInstalled() ([]InstalledPackage, error)
}

// Package represents a package to be managed by an adapter.
type Package struct {
	Name    string
	Version string // optional; semantics differ by platform
	Pinned  bool
	Cask    bool // Darwin only
}

// DefaultsValue represents a macOS defaults preference value.
type DefaultsValue struct {
	Type string // one of: "string", "boolean", "integer", "float"
	Raw  string // string representation of the value
}

// InstalledPackage represents a package installed on the system.
type InstalledPackage struct {
	Name    string
	Version string
	Held    bool
}

// commandRunner is a function type for executing external commands.
// Adapters accept this for dependency injection in tests.
type commandRunner func(name string, args ...string) ([]byte, error)

// Error constants for adapter operations.
var (
	ErrBrewNotFound          = errors.New("brew not found in PATH")
	ErrAptNotFound           = errors.New("apt not found in PATH")
	ErrBrewCalledAsRoot      = errors.New("brew must not be called as root")
	ErrPackagePinned         = errors.New("package is pinned")
	ErrCaskPinNotSupported   = errors.New("cask pinning is not supported")
	ErrVersionNotAvailable   = errors.New("version not available")
	ErrDefaultsTypeMismatch  = errors.New("defaults type mismatch")
	ErrDefaultsValueMismatch = errors.New("defaults value mismatch")
	ErrPrivilegeRequired     = errors.New("privilege escalation failed")
	ErrUnsupportedPlatform   = errors.New("unsupported platform")
	ErrNotSupported          = errors.New("operation not supported on this platform")
)
