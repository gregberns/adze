package config

// Config is the top-level configuration structure for an adze machine config file.
type Config struct {
	Name        string                              `yaml:"name"`
	Platform    string                              `yaml:"platform"`
	Tags        []string                            `yaml:"tags"`
	Include     []string                            `yaml:"include"`
	Machine     MachineConfig                       `yaml:"machine"`
	Identity    IdentityConfig                      `yaml:"identity"`
	Secrets     []SecretEntry                       `yaml:"secrets"`
	Packages    PackagesConfig                      `yaml:"packages"`
	Defaults    map[string]map[string]DefaultValue  `yaml:"defaults"`
	Dock        DockConfig                          `yaml:"dock"`
	Shell       ShellConfig                         `yaml:"shell"`
	Directories []string                            `yaml:"directories"`
	CustomSteps map[string]CustomStep               `yaml:"custom_steps"`
}

// MachineConfig holds machine identity settings.
type MachineConfig struct {
	Hostname string `yaml:"hostname"`
}

// IdentityConfig holds git and GitHub identity settings.
type IdentityConfig struct {
	GitName    string `yaml:"git_name"`
	GitEmail   string `yaml:"git_email"`
	GithubUser string `yaml:"github_user"`
}

// SecretEntry declares a required environment variable.
type SecretEntry struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Sensitive   bool   `yaml:"sensitive"`
	Validate    string `yaml:"validate"`
	Prompt      bool   `yaml:"prompt"`
}

// PackagesConfig holds package installation lists.
type PackagesConfig struct {
	Brew []PackageEntry `yaml:"brew"`
	Cask []PackageEntry `yaml:"cask"`
	Apt  []PackageEntry `yaml:"apt"`
}

// PackageEntry represents a package to install, supporting both short form (string)
// and object form (mapping with name, version, pinned).
type PackageEntry struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Pinned  bool   `yaml:"pinned"`
}

// DefaultValue holds a macOS defaults value. It can be bool, int, float64, or string.
type DefaultValue struct {
	Value interface{} // bool, int, float64, or string
}

// DockConfig holds Dock layout settings.
type DockConfig struct {
	Apps []string `yaml:"apps"`
}

// ShellConfig holds shell configuration.
type ShellConfig struct {
	Default  string   `yaml:"default"`
	OhMyZsh  bool     `yaml:"oh_my_zsh"`
	Theme    string   `yaml:"theme"`
	Plugins  []string `yaml:"plugins"`
}

// CustomStep defines a user-defined step with platform-specific commands.
type CustomStep struct {
	Description string            `yaml:"description"`
	Provides    []string          `yaml:"provides"`
	Requires    []string          `yaml:"requires"`
	Platform    []string          `yaml:"platform"`
	Check       string            `yaml:"check"`
	Apply       map[string]string `yaml:"apply"`
	Rollback    map[string]string `yaml:"rollback"`
	Env         []string          `yaml:"env"`
	Tags        []string          `yaml:"tags"`
}

// validTopLevelFields lists all valid top-level field names.
var validTopLevelFields = []string{
	"name", "platform", "tags", "include", "machine", "identity",
	"secrets", "packages", "defaults", "dock", "shell", "directories", "custom_steps",
}

// validPlatforms lists all valid platform values.
var validPlatforms = []string{"darwin", "ubuntu", "debian", "any"}

// validShells lists all valid shell values.
var validShells = []string{"zsh", "bash", "fish"}
