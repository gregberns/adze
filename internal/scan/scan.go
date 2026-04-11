// Package scan provides machine scanning for the adze init command.
// It detects installed packages, settings, and preferences on the current machine
// and produces a ScanResult that can be rendered as YAML config.
package scan

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScanResult holds all detected machine state.
type ScanResult struct {
	Platform    string
	Hostname    string
	Packages    ScanPackages
	Defaults    map[string]map[string]interface{}
	Shell       ScanShell
	Identity    ScanIdentity
	Directories []string
}

// ScanPackages holds detected package lists.
type ScanPackages struct {
	Brew []string
	Cask []string
	Apt  []string
}

// ScanShell holds detected shell configuration.
type ScanShell struct {
	Default string
	OhMyZsh bool
	Plugins []string
}

// ScanIdentity holds detected git/GitHub identity.
type ScanIdentity struct {
	GitName    string
	GitEmail   string
	GithubUser string
}

// CommandRunner is a function that runs a command and returns its output.
// This allows dependency injection for testing.
type CommandRunner func(name string, args ...string) (string, error)

// defaultRunner runs commands using os/exec.
func defaultRunner(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ScanMachine detects the current machine state for the given platform.
func ScanMachine(platform string) (*ScanResult, error) {
	return scanMachineWith(platform, defaultRunner)
}

// scanMachineWith is the testable core that accepts an injected command runner.
func scanMachineWith(platform string, run CommandRunner) (*ScanResult, error) {
	result := &ScanResult{
		Platform: platform,
		Defaults: make(map[string]map[string]interface{}),
	}

	// Hostname
	result.Hostname = detectHostname(platform, run)

	// Packages
	result.Packages = detectPackages(platform, run)

	// Defaults (macOS only)
	if platform == "darwin" {
		result.Defaults = detectDefaults(run)
	}

	// Shell
	result.Shell = detectShell(run)

	// Identity
	result.Identity = detectIdentity(run)

	// Directories
	result.Directories = detectDirectories()

	return result, nil
}

// detectHostname detects the machine hostname.
func detectHostname(platform string, run CommandRunner) string {
	switch platform {
	case "darwin":
		if name, err := run("scutil", "--get", "ComputerName"); err == nil && name != "" {
			return name
		}
	case "ubuntu":
		if name, err := run("hostnamectl", "status", "--static"); err == nil && name != "" {
			return name
		}
	}
	// Fallback: os.Hostname
	if name, err := os.Hostname(); err == nil {
		return name
	}
	return ""
}

// detectPackages detects installed packages per platform.
func detectPackages(platform string, run CommandRunner) ScanPackages {
	var pkgs ScanPackages

	switch platform {
	case "darwin":
		pkgs.Brew = detectBrewPackages(run)
		pkgs.Cask = detectBrewCasks(run)
	case "ubuntu":
		pkgs.Apt = detectAptPackages(run)
	}

	return pkgs
}

// detectBrewPackages detects installed Homebrew formulas.
func detectBrewPackages(run CommandRunner) []string {
	out, err := run("brew", "leaves")
	if err != nil {
		return nil
	}
	return parseLines(out)
}

// detectBrewCasks detects installed Homebrew casks.
func detectBrewCasks(run CommandRunner) []string {
	out, err := run("brew", "list", "--cask")
	if err != nil {
		return nil
	}
	return parseLines(out)
}

// detectAptPackages detects manually installed APT packages.
func detectAptPackages(run CommandRunner) []string {
	out, err := run("apt-mark", "showmanual")
	if err != nil {
		return nil
	}
	return parseLines(out)
}

// detectDefaults detects macOS defaults for common domains.
func detectDefaults(run CommandRunner) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})

	// NSGlobalDomain — detect a few well-known preferences
	globalPrefs := detectDefaultsDomain(run, "NSGlobalDomain", []string{
		"AppleShowAllExtensions",
		"NSAutomaticSpellingCorrectionEnabled",
		"NSAutomaticCapitalizationEnabled",
	})
	if len(globalPrefs) > 0 {
		result["NSGlobalDomain"] = globalPrefs
	}

	// com.apple.dock
	dockPrefs := detectDefaultsDomain(run, "com.apple.dock", []string{
		"autohide",
		"tilesize",
		"magnification",
	})
	if len(dockPrefs) > 0 {
		result["com.apple.dock"] = dockPrefs
	}

	return result
}

// detectDefaultsDomain reads specific keys from a macOS defaults domain.
func detectDefaultsDomain(run CommandRunner, domain string, keys []string) map[string]interface{} {
	prefs := make(map[string]interface{})
	for _, key := range keys {
		val, err := run("defaults", "read", domain, key)
		if err != nil {
			continue
		}
		prefs[key] = parseDefaultsValue(val)
	}
	return prefs
}

// parseDefaultsValue attempts to parse a defaults output value into a typed value.
func parseDefaultsValue(s string) interface{} {
	s = strings.TrimSpace(s)
	switch s {
	case "1", "true":
		return true
	case "0", "false":
		return false
	}
	// Try integer
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return i
	}
	return s
}

// detectShell detects the current shell configuration.
func detectShell(run CommandRunner) ScanShell {
	var shell ScanShell

	// Default shell from $SHELL
	shellEnv := os.Getenv("SHELL")
	if shellEnv != "" {
		shell.Default = filepath.Base(shellEnv)
	}

	// Oh My Zsh detection
	home, err := os.UserHomeDir()
	if err == nil {
		omzDir := filepath.Join(home, ".oh-my-zsh")
		if info, err := os.Stat(omzDir); err == nil && info.IsDir() {
			shell.OhMyZsh = true
			shell.Plugins = detectOhMyZshPlugins(home)
		}
	}

	return shell
}

// detectOhMyZshPlugins reads the plugins from .zshrc.
func detectOhMyZshPlugins(home string) []string {
	zshrc := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(zshrc)
	if err != nil {
		return nil
	}

	content := string(data)
	return parseZshPlugins(content)
}

// parseZshPlugins extracts plugin names from a .zshrc plugins=(...) line.
func parseZshPlugins(content string) []string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "plugins=(") {
			// Extract plugin list. May span multiple lines but we handle single-line case.
			inner := strings.TrimPrefix(trimmed, "plugins=(")
			inner = strings.TrimSuffix(inner, ")")
			fields := strings.Fields(inner)
			if len(fields) > 0 {
				return fields
			}
		}
	}
	return nil
}

// detectIdentity detects git and GitHub identity.
func detectIdentity(run CommandRunner) ScanIdentity {
	var id ScanIdentity

	if name, err := run("git", "config", "--global", "user.name"); err == nil {
		id.GitName = name
	}
	if email, err := run("git", "config", "--global", "user.email"); err == nil {
		id.GitEmail = email
	}
	if user, err := run("git", "config", "--global", "github.user"); err == nil {
		id.GithubUser = user
	}

	return id
}

// detectDirectories checks for common development directories.
func detectDirectories() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	candidates := []string{
		"~/Projects",
		"~/projects",
		"~/Development",
		"~/dev",
		"~/workspace",
		"~/go",
		"~/src",
		"~/.config",
	}

	var found []string
	for _, d := range candidates {
		expanded := strings.Replace(d, "~", home, 1)
		if info, err := os.Stat(expanded); err == nil && info.IsDir() {
			found = append(found, d)
		}
	}

	return found
}

// yamlConfig is the intermediate struct for YAML serialization.
type yamlConfig struct {
	Name        string                            `yaml:"name"`
	Platform    string                            `yaml:"platform"`
	Machine     *yamlMachine                      `yaml:"machine,omitempty"`
	Identity    *yamlIdentity                     `yaml:"identity,omitempty"`
	Packages    *yamlPackages                     `yaml:"packages,omitempty"`
	Defaults    map[string]map[string]interface{} `yaml:"defaults,omitempty"`
	Shell       *yamlShell                        `yaml:"shell,omitempty"`
	Directories []string                          `yaml:"directories,omitempty"`
}

type yamlMachine struct {
	Hostname string `yaml:"hostname"`
}

type yamlIdentity struct {
	GitName    string `yaml:"git_name,omitempty"`
	GitEmail   string `yaml:"git_email,omitempty"`
	GithubUser string `yaml:"github_user,omitempty"`
}

type yamlPackages struct {
	Brew []string `yaml:"brew,omitempty"`
	Cask []string `yaml:"cask,omitempty"`
	Apt  []string `yaml:"apt,omitempty"`
}

type yamlShell struct {
	Default string   `yaml:"default,omitempty"`
	OhMyZsh bool     `yaml:"oh_my_zsh,omitempty"`
	Plugins []string `yaml:"plugins,omitempty"`
}

// ToYAML converts the scan result to valid adze config YAML.
func (r *ScanResult) ToYAML() ([]byte, error) {
	cfg := yamlConfig{
		Name:     machineName(r),
		Platform: r.Platform,
	}

	if r.Hostname != "" {
		cfg.Machine = &yamlMachine{Hostname: r.Hostname}
	}

	if r.Identity.GitName != "" || r.Identity.GitEmail != "" || r.Identity.GithubUser != "" {
		cfg.Identity = &yamlIdentity{
			GitName:    r.Identity.GitName,
			GitEmail:   r.Identity.GitEmail,
			GithubUser: r.Identity.GithubUser,
		}
	}

	if len(r.Packages.Brew) > 0 || len(r.Packages.Cask) > 0 || len(r.Packages.Apt) > 0 {
		cfg.Packages = &yamlPackages{
			Brew: sortedCopy(r.Packages.Brew),
			Cask: sortedCopy(r.Packages.Cask),
			Apt:  sortedCopy(r.Packages.Apt),
		}
	}

	if len(r.Defaults) > 0 {
		cfg.Defaults = r.Defaults
	}

	if r.Shell.Default != "" || r.Shell.OhMyZsh || len(r.Shell.Plugins) > 0 {
		cfg.Shell = &yamlShell{
			Default: r.Shell.Default,
			OhMyZsh: r.Shell.OhMyZsh,
			Plugins: r.Shell.Plugins,
		}
	}

	if len(r.Directories) > 0 {
		cfg.Directories = r.Directories
	}

	return yaml.Marshal(cfg)
}

// machineName generates a config name from the scan result.
func machineName(r *ScanResult) string {
	if r.Hostname != "" {
		return strings.ToLower(strings.ReplaceAll(r.Hostname, " ", "-"))
	}
	return fmt.Sprintf("my-%s-machine", r.Platform)
}

// parseLines splits output into non-empty trimmed lines.
func parseLines(s string) []string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// sortedCopy returns a sorted copy of a string slice (nil if empty).
func sortedCopy(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	cp := make([]string, len(ss))
	copy(cp, ss)
	sort.Strings(cp)
	return cp
}

// CurrentPlatform returns the platform string for the running system.
func CurrentPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		return "ubuntu"
	default:
		return runtime.GOOS
	}
}
