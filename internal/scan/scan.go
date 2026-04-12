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
	"time"

	"gopkg.in/yaml.v3"
)

// ScanResult holds all detected machine state.
type ScanResult struct {
	Platform    string
	Hostname    string
	Packages    ScanPackages
	Defaults    map[string]map[string]interface{}
	Dock        []string // dock app names from dockutil
	DockError   string   // non-empty if dockutil missing or failed
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

	// Dock (macOS only, requires dockutil)
	if platform == "darwin" {
		result.Dock, result.DockError = detectDock(run)
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

	// com.apple.finder
	finderPrefs := detectDefaultsDomain(run, "com.apple.finder", []string{
		"AppleShowAllFiles",
		"ShowPathbar",
		"ShowStatusBar",
		"FXDefaultSearchScope",
		"FXPreferredViewStyle",
	})
	if len(finderPrefs) > 0 {
		result["com.apple.finder"] = finderPrefs
	}

	// com.apple.screencapture
	screencapPrefs := detectDefaultsDomain(run, "com.apple.screencapture", []string{
		"location",
		"type",
		"disable-shadow",
	})
	if len(screencapPrefs) > 0 {
		result["com.apple.screencapture"] = screencapPrefs
	}

	return result
}

// detectDock detects dock apps using dockutil.
func detectDock(run CommandRunner) ([]string, string) {
	out, err := run("dockutil", "--list")
	if err != nil {
		return nil, "dockutil not installed"
	}
	// dockutil --list outputs tab-separated: name\tpath\n
	lines := strings.Split(out, "\n")
	var apps []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		apps = append(apps, strings.TrimSpace(parts[0]))
	}
	return apps, ""
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
		"~/github",
		"~/gitlab",
		"~/code",
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

// ToYAML converts the scan result to valid adze config YAML with helpful comments.
func (r *ScanResult) ToYAML() ([]byte, error) {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	root := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = append(doc.Content, root)

	// Header comment
	root.HeadComment = fmt.Sprintf("Generated by adze init\nPlatform: %s\nDate: %s",
		r.Platform, currentDate())

	// name + platform (always present)
	addScalar(root, "name", machineName(r), "")
	addScalar(root, "platform", r.Platform, "")

	// machine
	if r.Hostname != "" {
		machineKey := scalarNode("machine", "")
		machineMap := &yaml.Node{Kind: yaml.MappingNode}
		addScalar(machineMap, "hostname", r.Hostname, "")
		root.Content = append(root.Content, machineKey, machineMap)
	}

	// identity
	if r.Identity.GitName != "" || r.Identity.GitEmail != "" || r.Identity.GithubUser != "" {
		idKey := scalarNode("identity", "Detected from: git config --global")
		idMap := &yaml.Node{Kind: yaml.MappingNode}
		if r.Identity.GitName != "" {
			addScalar(idMap, "git_name", r.Identity.GitName, "")
		}
		if r.Identity.GitEmail != "" {
			addScalar(idMap, "git_email", r.Identity.GitEmail, "")
		}
		if r.Identity.GithubUser != "" {
			addScalar(idMap, "github_user", r.Identity.GithubUser, "")
		}
		root.Content = append(root.Content, idKey, idMap)
	}

	// packages
	hasBrew := len(r.Packages.Brew) > 0
	hasCask := len(r.Packages.Cask) > 0
	hasApt := len(r.Packages.Apt) > 0
	if hasBrew || hasCask || hasApt {
		pkgKey := scalarNode("packages", "")
		pkgMap := &yaml.Node{Kind: yaml.MappingNode}
		if hasBrew {
			brewKey := scalarNode("brew", fmt.Sprintf("Detected from: brew leaves (%d packages)", len(r.Packages.Brew)))
			brewSeq := sequenceNode(sortedCopy(r.Packages.Brew))
			pkgMap.Content = append(pkgMap.Content, brewKey, brewSeq)
		}
		if hasCask {
			caskKey := scalarNode("cask", fmt.Sprintf("Detected from: brew list --cask (%d packages)", len(r.Packages.Cask)))
			caskSeq := sequenceNode(sortedCopy(r.Packages.Cask))
			pkgMap.Content = append(pkgMap.Content, caskKey, caskSeq)
		}
		if hasApt {
			aptKey := scalarNode("apt", fmt.Sprintf("Detected from: apt-mark showmanual (%d packages)", len(r.Packages.Apt)))
			aptSeq := sequenceNode(sortedCopy(r.Packages.Apt))
			pkgMap.Content = append(pkgMap.Content, aptKey, aptSeq)
		}
		root.Content = append(root.Content, pkgKey, pkgMap)
	}

	// defaults
	if len(r.Defaults) > 0 {
		defKey := scalarNode("defaults", "Detected from: defaults read <domain>")
		defMap := &yaml.Node{Kind: yaml.MappingNode}
		// Sort domain names for deterministic output
		domainNames := make([]string, 0, len(r.Defaults))
		for d := range r.Defaults {
			domainNames = append(domainNames, d)
		}
		sort.Strings(domainNames)
		for _, domain := range domainNames {
			prefs := r.Defaults[domain]
			domKey := scalarNode(domain, "")
			domMap := &yaml.Node{Kind: yaml.MappingNode}
			keys := make([]string, 0, len(prefs))
			for k := range prefs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				addScalar(domMap, k, fmt.Sprintf("%v", prefs[k]), "")
			}
			defMap.Content = append(defMap.Content, domKey, domMap)
		}
		root.Content = append(root.Content, defKey, defMap)
	}

	// dock
	if len(r.Dock) > 0 {
		dockKey := scalarNode("dock", fmt.Sprintf("Detected from: dockutil --list (%d apps)", len(r.Dock)))
		dockSeq := sequenceNode(r.Dock)
		root.Content = append(root.Content, dockKey, dockSeq)
	} else if r.DockError != "" {
		// Add a comment-only placeholder so the user knows it was skipped
		dockKey := scalarNode("dock", fmt.Sprintf("dock: (%s)", r.DockError))
		dockSeq := &yaml.Node{Kind: yaml.SequenceNode}
		root.Content = append(root.Content, dockKey, dockSeq)
	}

	// shell
	if r.Shell.Default != "" || r.Shell.OhMyZsh || len(r.Shell.Plugins) > 0 {
		shellKey := scalarNode("shell", "Detected from: $SHELL and ~/.oh-my-zsh")
		shellMap := &yaml.Node{Kind: yaml.MappingNode}
		if r.Shell.Default != "" {
			addScalar(shellMap, "default", r.Shell.Default, "")
		}
		if r.Shell.OhMyZsh {
			addScalar(shellMap, "oh_my_zsh", "true", "")
		}
		if len(r.Shell.Plugins) > 0 {
			plugKey := scalarNode("plugins", "")
			plugSeq := sequenceNode(r.Shell.Plugins)
			shellMap.Content = append(shellMap.Content, plugKey, plugSeq)
		}
		root.Content = append(root.Content, shellKey, shellMap)
	}

	// directories
	if len(r.Directories) > 0 {
		dirKey := scalarNode("directories", fmt.Sprintf("Detected from: scanning ~/ (%d directories)", len(r.Directories)))
		dirSeq := sequenceNode(r.Directories)
		root.Content = append(root.Content, dirKey, dirSeq)
	}

	return yaml.Marshal(doc)
}

// currentDate returns today's date as YYYY-MM-DD. Extracted for testability.
var currentDate = func() string {
	return fmt.Sprintf("%d-%02d-%02d",
		timeNow().Year(), timeNow().Month(), timeNow().Day())
}

// timeNow returns the current time. Can be overridden in tests.
var timeNow = time.Now

// scalarNode creates a yaml scalar node with an optional head comment.
func scalarNode(value, comment string) *yaml.Node {
	n := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: value,
	}
	if comment != "" {
		n.HeadComment = comment
	}
	return n
}

// addScalar appends a key-value scalar pair to a mapping node.
func addScalar(mapping *yaml.Node, key, value, comment string) {
	k := scalarNode(key, comment)
	v := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	mapping.Content = append(mapping.Content, k, v)
}

// sequenceNode creates a yaml sequence node from a string slice.
func sequenceNode(items []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, item := range items {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: item})
	}
	return seq
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
