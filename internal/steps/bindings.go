package steps

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gregberns/adze/internal/config"
	"github.com/gregberns/adze/internal/step"
)

// BuildStepConfigs takes a parsed Config and produces StepConfigs per the
// Step Inclusion Rule (specs/step-library.md): a built-in step is included
// iff (a) its ConfigSection predicate is satisfied, or (b) some other
// included step transitively Requires one of its Provides capabilities.
//
// Phase 1: built-in steps with a non-empty ConfigSection are evaluated;
// those whose section predicate is satisfied become seeds.
// Phase 2: custom steps are added as seeds.
// Phase 3: transitive-only candidates (built-in steps with empty
// ConfigSection) are pulled in iff something in the seed set requires
// one of their capabilities. See expandCandidates for the algorithm.
func BuildStepConfigs(cfg *config.Config, platform string, reg *Registry) []step.StepConfig {
	defs := reg.ForPlatform(platform)

	var seeds []step.StepConfig
	var candidates []StepDefinition

	// A step is a seed iff its config-section predicate is satisfied
	// (buildStepConfig returns non-nil). All other defs are eligible
	// candidates for transitive inclusion.
	for _, def := range defs {
		if !isCandidate(def) {
			sc := buildStepConfig(cfg, platform, def)
			if sc != nil {
				seeds = append(seeds, *sc)
				continue
			}
		}
		candidates = append(candidates, def)
	}

	for name, cs := range cfg.CustomSteps {
		sc := buildCustomStepConfig(name, cs, platform)
		if sc != nil {
			seeds = append(seeds, *sc)
		}
	}

	return expandCandidates(seeds, candidates, platform, cfg)
}

// isCandidate reports whether a built-in step is "transitive-only" —
// eligible for inclusion only when another included step requires one
// of its Provides capabilities. Per specs/step-library.md, this is
// exactly the set of steps with no ConfigSection.
func isCandidate(def StepDefinition) bool {
	return def.ConfigSection == ""
}

// expandCandidates performs fixed-point transitive expansion of candidate
// steps over the given seed set. See specs/dag-resolver.md "Pre-Resolution:
// Candidate Expansion" for the formal algorithm.
//
// Determinism: candidates are iterated in sorted name order; Requires
// lists are iterated in input order. Re-running on identical inputs
// produces identical output.
func expandCandidates(seeds []step.StepConfig, candidates []StepDefinition, platform string, cfg *config.Config) []step.StepConfig {
	sortedCands := make([]StepDefinition, len(candidates))
	copy(sortedCands, candidates)
	sort.Slice(sortedCands, func(i, j int) bool {
		return sortedCands[i].Name < sortedCands[j].Name
	})

	included := make([]step.StepConfig, len(seeds))
	copy(included, seeds)

	providedBy := make(map[string]string)
	addedNames := make(map[string]bool)
	for _, sc := range included {
		addedNames[sc.Name] = true
		for _, cap := range sc.Provides {
			providedBy[cap] = sc.Name
		}
	}

	for {
		changed := false
		snapshot := make([]step.StepConfig, len(included))
		copy(snapshot, included)

		for _, sc := range snapshot {
			for _, req := range sc.Requires {
				if _, satisfied := providedBy[req]; satisfied {
					continue
				}
				for _, cand := range sortedCands {
					if addedNames[cand.Name] {
						continue
					}
					if !containsCapability(cand.Provides, req) {
						continue
					}
					newSC := buildCandidateStepConfig(cfg, platform, cand)
					if newSC == nil {
						continue
					}
					included = append(included, *newSC)
					addedNames[cand.Name] = true
					for _, cap := range cand.Provides {
						providedBy[cap] = cand.Name
					}
					changed = true
					break
				}
			}
		}

		if !changed {
			break
		}
	}

	return included
}

func containsCapability(provides []string, cap string) bool {
	for _, p := range provides {
		if p == cap {
			return true
		}
	}
	return false
}

// buildCandidateStepConfig constructs a StepConfig for a transitive-only
// step. Mirrors buildStepConfig's default shape but always routes through
// populateBuiltinCommands since candidates have no config section.
func buildCandidateStepConfig(cfg *config.Config, platform string, def StepDefinition) *step.StepConfig {
	sc := &step.StepConfig{
		Name:         def.Name,
		Description:  def.Description,
		Provides:     def.Provides,
		Requires:     def.RequiresForPlatform(platform),
		Platforms:    def.Platforms,
		CheckTimeout: 5 * time.Minute,
		ApplyTimeout: 15 * time.Minute,
	}
	populateBuiltinCommands(sc, def, platform, cfg)
	return sc
}

// buildStepConfig creates a StepConfig for a built-in step definition,
// populating items from the config where applicable. Returns nil if the step
// should be skipped (e.g., batch step with no items).
func buildStepConfig(cfg *config.Config, platform string, def StepDefinition) *step.StepConfig {
	sc := &step.StepConfig{
		Name:         def.Name,
		Description:  def.Description,
		Provides:     def.Provides,
		Requires:     def.RequiresForPlatform(platform),
		Platforms:    def.Platforms,
		CheckTimeout: 5 * time.Minute,
		ApplyTimeout: 15 * time.Minute,
	}

	// Populate items and commands based on config section.
	switch def.ConfigSection {
	case "packages.brew":
		if len(cfg.Packages.Brew) == 0 {
			return nil
		}
		sc.Items = packageEntriesToItems(cfg.Packages.Brew)

	case "packages.cask":
		if len(cfg.Packages.Cask) == 0 {
			return nil
		}
		sc.Items = packageEntriesToItems(cfg.Packages.Cask)

	case "packages.apt":
		if len(cfg.Packages.Apt) == 0 {
			return nil
		}
		sc.Items = packageEntriesToItems(cfg.Packages.Apt)

	case "defaults":
		items := defaultsToItems(cfg.Defaults, platform)
		if len(items) == 0 {
			return nil
		}
		sc.Items = items

	case "dock":
		if len(cfg.Dock.Apps) == 0 {
			return nil
		}
		sc.Items = dockAppsToItems(cfg.Dock.Apps)

	case "shell.plugins":
		if len(cfg.Shell.Plugins) == 0 {
			return nil
		}
		sc.Items = pluginsToItems(cfg.Shell.Plugins)

	case "shell.default":
		if cfg.Shell.Default == "" {
			return nil
		}
		shellPath := resolveShellPath(cfg.Shell.Default)
		sc.Check = shellCmd(fmt.Sprintf(`[ "$SHELL" = "%s" ]`, shellPath))
		sc.Apply = shellCmd(fmt.Sprintf("chsh -s %s", shellPath))

	case "shell.oh_my_zsh":
		if !cfg.Shell.OhMyZsh {
			return nil
		}
		// oh-my-zsh check/apply are built into the step itself.

	case "machine.hostname":
		if cfg.Machine.Hostname == "" {
			return nil
		}
		hostname := cfg.Machine.Hostname
		sc.Check = shellCmd(fmt.Sprintf(`[ "$(scutil --get ComputerName)" = "%s" ]`, hostname))
		sc.Apply = shellCmd(fmt.Sprintf(
			`sudo scutil --set ComputerName "%s" && sudo scutil --set LocalHostName "%s" && sudo scutil --set HostName "%s"`,
			hostname, hostname, hostname))

	case "identity":
		if cfg.Identity.GitName == "" && cfg.Identity.GitEmail == "" && cfg.Identity.GithubUser == "" {
			return nil
		}
		sc.Check = buildGitConfigCheck(cfg.Identity)
		sc.Apply = buildGitConfigApply(cfg.Identity)

	case "identity.generate_ssh_key":
		if !cfg.Identity.GenerateSSHKey {
			return nil
		}
		populateBuiltinCommands(sc, def, platform, cfg)

	case "directories":
		if len(cfg.Directories) == 0 {
			return nil
		}
		sc.Items = directoriesToItems(cfg.Directories)
	}

	return sc
}

// populateBuiltinCommands sets default check/apply commands for steps that
// don't read from config sections.
func populateBuiltinCommands(sc *step.StepConfig, def StepDefinition, platform string, cfg *config.Config) {
	switch def.Name {
	case "xcode-cli-tools":
		sc.Check = shellCmd("xcode-select -p")
		sc.Apply = shellCmd("xcode-select --install")

	case "homebrew":
		sc.Check = shellCmd("command -v brew")
		sc.Apply = shellCmd(`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)

	case "apt-essentials":
		sc.Check = shellCmd("dpkg-query -W build-essential")
		sc.Apply = shellCmd("sudo apt-get update && sudo apt-get install -y build-essential curl wget git")

	case "node-fnm":
		sc.Check = shellCmd("command -v fnm")
		sc.PlatformApply = map[string]*step.ShellCommand{
			"darwin": shellCmd("brew install fnm"),
			"ubuntu": shellCmd(`curl -fsSL https://fnm.vercel.app/install | bash`),
		}

	case "python":
		sc.Check = shellCmd("command -v python3")
		sc.PlatformApply = map[string]*step.ShellCommand{
			"darwin": shellCmd("brew install python"),
			"ubuntu": shellCmd("sudo apt-get install -y python3 python3-pip python3-venv"),
		}

	case "go":
		sc.Check = shellCmd("command -v go")
		sc.PlatformApply = map[string]*step.ShellCommand{
			"darwin": shellCmd("brew install go"),
			"ubuntu": shellCmd("sudo apt-get install -y golang"),
		}

	case "rust":
		sc.Check = shellCmd("command -v rustc")
		sc.Apply = shellCmd(`curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y`)

	case "ssh-keys":
		sc.Check = shellCmd(`[ -f "$HOME/.ssh/id_ed25519" ]`)
		if cfg.Identity.GitEmail != "" {
			sc.Apply = shellCmd(fmt.Sprintf(`ssh-keygen -t ed25519 -f "$HOME/.ssh/id_ed25519" -N "" -C "%s"`, cfg.Identity.GitEmail))
		} else {
			sc.Apply = shellCmd(`ssh-keygen -t ed25519 -f "$HOME/.ssh/id_ed25519" -N ""`)
		}
	}
}

// buildCustomStepConfig converts a config.CustomStep to a step.StepConfig.
func buildCustomStepConfig(name string, cs config.CustomStep, platform string) *step.StepConfig {
	// Filter by platform.
	if len(cs.Platform) > 0 && !platformMatches(cs.Platform, platform) {
		return nil
	}

	sc := &step.StepConfig{
		Name:         name,
		Description:  cs.Description,
		Tags:         cs.Tags,
		Provides:     cs.Provides,
		Requires:     cs.Requires,
		Platforms:    cs.Platform,
		Env:          cs.Env,
		CheckTimeout: 5 * time.Minute,
		ApplyTimeout: 15 * time.Minute,
	}

	if cs.Check != "" {
		sc.Check = parseCommand(cs.Check)
	}

	// Apply is platform-dispatched for custom steps.
	if len(cs.Apply) > 0 {
		// Check for a platform-specific apply.
		if cmd, ok := cs.Apply[platform]; ok {
			sc.Apply = parseCommand(cmd)
		} else if cmd, ok := cs.Apply["any"]; ok {
			sc.Apply = parseCommand(cmd)
		}

		// Also set PlatformApply for the executor.
		sc.PlatformApply = make(map[string]*step.ShellCommand)
		for p, cmd := range cs.Apply {
			sc.PlatformApply[p] = parseCommand(cmd)
		}
	}

	// Rollback (not executed in v1).
	if len(cs.Rollback) > 0 {
		if cmd, ok := cs.Rollback[platform]; ok {
			sc.Rollback = parseCommand(cmd)
		} else if cmd, ok := cs.Rollback["any"]; ok {
			sc.Rollback = parseCommand(cmd)
		}
	}

	return sc
}

// parseCommand converts a shell command string to a ShellCommand.
// Commands are run via sh -c.
func parseCommand(cmd string) *step.ShellCommand {
	return shellCmd(cmd)
}

// packageEntriesToItems converts PackageEntry slices to StepItem slices.
func packageEntriesToItems(entries []config.PackageEntry) []step.StepItem {
	items := make([]step.StepItem, len(entries))
	for i, e := range entries {
		items[i] = step.StepItem{
			Name:    e.Name,
			Version: e.Version,
			Pinned:  e.Pinned,
		}
	}
	return items
}

// dockAppsToItems converts dock app names to StepItems.
func dockAppsToItems(apps []string) []step.StepItem {
	items := make([]step.StepItem, len(apps))
	for i, app := range apps {
		items[i] = step.StepItem{Name: app}
	}
	return items
}

// pluginsToItems converts plugin names to StepItems.
func pluginsToItems(plugins []string) []step.StepItem {
	items := make([]step.StepItem, len(plugins))
	for i, p := range plugins {
		items[i] = step.StepItem{Name: p}
	}
	return items
}

// directoriesToItems converts directory paths to StepItems.
func directoriesToItems(dirs []string) []step.StepItem {
	items := make([]step.StepItem, len(dirs))
	for i, d := range dirs {
		items[i] = step.StepItem{Name: d}
	}
	return items
}

// defaultsToItems converts the defaults config section to StepItems.
// Each item encodes "domain key" in Name and the type+value in Version.
func defaultsToItems(defaults map[string]map[string]config.DefaultValue, platform string) []step.StepItem {
	if defaults == nil {
		return nil
	}

	var items []step.StepItem
	for domain, keys := range defaults {
		for key, dv := range keys {
			itemName := fmt.Sprintf("%s %s", domain, key)
			var typeValue string
			switch v := dv.Value.(type) {
			case bool:
				typeValue = fmt.Sprintf("-bool %t", v)
			case int:
				typeValue = fmt.Sprintf("-int %d", v)
			case float64:
				// Check if it's actually an integer.
				if v == float64(int(v)) {
					typeValue = fmt.Sprintf("-int %d", int(v))
				} else {
					typeValue = fmt.Sprintf("-float %g", v)
				}
			case string:
				typeValue = fmt.Sprintf("-string %s", v)
			default:
				typeValue = fmt.Sprintf("-string %v", v)
			}

			// For gsettings (ubuntu), just store the value without type prefix.
			if platform == "ubuntu" {
				switch v := dv.Value.(type) {
				case bool:
					typeValue = fmt.Sprintf("%t", v)
				case int:
					typeValue = fmt.Sprintf("%d", v)
				case float64:
					if v == float64(int(v)) {
						typeValue = fmt.Sprintf("%d", int(v))
					} else {
						typeValue = fmt.Sprintf("%g", v)
					}
				case string:
					typeValue = fmt.Sprintf("'%s'", v)
				default:
					typeValue = fmt.Sprintf("%v", v)
				}
			}

			items = append(items, step.StepItem{
				Name:    itemName,
				Version: typeValue,
			})
		}
	}
	return items
}

// resolveShellPath converts a shell name to its binary path.
func resolveShellPath(shell string) string {
	if path, ok := shellPaths[shell]; ok {
		return path
	}
	// If the shell is already a path, use it directly.
	if strings.HasPrefix(shell, "/") {
		return shell
	}
	// Default: assume it's in /bin/
	return "/bin/" + shell
}

// buildGitConfigCheck creates a check command for git config.
func buildGitConfigCheck(id config.IdentityConfig) *step.ShellCommand {
	var checks []string
	if id.GitName != "" {
		checks = append(checks, fmt.Sprintf(`[ "$(git config --global user.name)" = "%s" ]`, id.GitName))
	}
	if id.GitEmail != "" {
		checks = append(checks, fmt.Sprintf(`[ "$(git config --global user.email)" = "%s" ]`, id.GitEmail))
	}
	if id.GithubUser != "" {
		checks = append(checks, fmt.Sprintf(`[ "$(git config --global github.user)" = "%s" ]`, id.GithubUser))
	}
	if len(checks) == 0 {
		return nil
	}
	return shellCmd(strings.Join(checks, " && "))
}

// buildGitConfigApply creates an apply command for git config.
func buildGitConfigApply(id config.IdentityConfig) *step.ShellCommand {
	var cmds []string
	if id.GitName != "" {
		cmds = append(cmds, fmt.Sprintf(`git config --global user.name "%s"`, id.GitName))
	}
	if id.GitEmail != "" {
		cmds = append(cmds, fmt.Sprintf(`git config --global user.email "%s"`, id.GitEmail))
	}
	if id.GithubUser != "" {
		cmds = append(cmds, fmt.Sprintf(`git config --global github.user "%s"`, id.GithubUser))
	}
	if len(cmds) == 0 {
		return nil
	}
	return shellCmd(strings.Join(cmds, " && "))
}
