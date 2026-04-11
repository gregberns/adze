package steps

import "github.com/gregberns/adze/internal/step"

// registerAll populates the registry with all built-in step definitions.
func registerAll(r *Registry) {
	// --- Core Infrastructure ---

	r.Register(StepDefinition{
		Name:        "xcode-cli-tools",
		Description: "Install Xcode Command Line Tools",
		Category:    "core",
		Type:        "atomic",
		Platforms:   []string{"darwin"},
		Provides:    []string{"xcode-cli-tools"},
		Requires:    nil,
		Constructor: func() step.Step { return NewXcodeCLIToolsStep() },
	})

	r.Register(StepDefinition{
		Name:        "homebrew",
		Description: "Install Homebrew package manager",
		Category:    "core",
		Type:        "atomic",
		Platforms:   []string{"darwin"},
		Provides:    []string{"homebrew"},
		Requires:    []string{"xcode-cli-tools"},
		Constructor: func() step.Step { return NewHomebrewStep() },
	})

	r.Register(StepDefinition{
		Name:        "apt-essentials",
		Description: "Install build-essential and common deps",
		Category:    "core",
		Type:        "atomic",
		Platforms:   []string{"ubuntu", "debian"},
		Provides:    []string{"apt-essentials"},
		Requires:    nil,
		Constructor: func() step.Step { return NewAptEssentialsStep() },
	})

	// --- Package Management ---

	r.Register(StepDefinition{
		Name:          "brew-packages",
		Description:   "Install Homebrew formulas from config",
		Category:      "packages",
		Type:          "batch",
		Platforms:     []string{"darwin"},
		Provides:      []string{"brew-packages"},
		Requires:      []string{"homebrew"},
		ConfigSection: "packages.brew",
		Constructor:   func() step.Step { return NewBrewPackagesStep() },
	})

	r.Register(StepDefinition{
		Name:          "brew-casks",
		Description:   "Install Homebrew casks from config",
		Category:      "packages",
		Type:          "batch",
		Platforms:     []string{"darwin"},
		Provides:      []string{"brew-casks"},
		Requires:      []string{"homebrew"},
		ConfigSection: "packages.cask",
		Constructor:   func() step.Step { return NewBrewCasksStep() },
	})

	r.Register(StepDefinition{
		Name:          "apt-packages",
		Description:   "Install APT packages from config",
		Category:      "packages",
		Type:          "batch",
		Platforms:     []string{"ubuntu", "debian"},
		Provides:      []string{"apt-packages"},
		Requires:      []string{"apt-essentials"},
		ConfigSection: "packages.apt",
		Constructor:   func() step.Step { return NewAptPackagesStep() },
	})

	// --- Languages ---

	r.Register(StepDefinition{
		Name:        "node-fnm",
		Description: "Install fnm (Fast Node Manager)",
		Category:    "languages",
		Type:        "atomic",
		Platforms:   []string{"darwin", "ubuntu"},
		Provides:    []string{"node", "fnm"},
		Requires:    []string{"homebrew"}, // ubuntu requires apt-essentials, but DAG resolves per platform
		Constructor: func() step.Step { return NewNodeFnmStep() },
	})

	r.Register(StepDefinition{
		Name:        "python",
		Description: "Install Python 3",
		Category:    "languages",
		Type:        "atomic",
		Platforms:   []string{"darwin", "ubuntu"},
		Provides:    []string{"python"},
		Requires:    []string{"homebrew"},
		Constructor: func() step.Step { return NewPythonStep() },
	})

	r.Register(StepDefinition{
		Name:        "go",
		Description: "Install Go programming language",
		Category:    "languages",
		Type:        "atomic",
		Platforms:   []string{"darwin", "ubuntu"},
		Provides:    []string{"go"},
		Requires:    []string{"homebrew"},
		Constructor: func() step.Step { return NewGoStep() },
	})

	r.Register(StepDefinition{
		Name:        "rust",
		Description: "Install Rust via rustup",
		Category:    "languages",
		Type:        "atomic",
		Platforms:   []string{"any"},
		Provides:    []string{"rust", "cargo"},
		Requires:    nil,
		Constructor: func() step.Step { return NewRustStep() },
	})

	// --- Shell ---

	r.Register(StepDefinition{
		Name:          "oh-my-zsh",
		Description:   "Install Oh My Zsh",
		Category:      "shell",
		Type:          "atomic",
		Platforms:     []string{"any"},
		Provides:      []string{"oh-my-zsh"},
		Requires:      nil,
		ConfigSection: "shell.oh_my_zsh",
		Constructor:   func() step.Step { return NewOhMyZshStep() },
	})

	r.Register(StepDefinition{
		Name:          "zsh-plugins",
		Description:   "Install Zsh plugins",
		Category:      "shell",
		Type:          "batch",
		Platforms:     []string{"any"},
		Provides:      []string{"zsh-plugins"},
		Requires:      []string{"oh-my-zsh"},
		ConfigSection: "shell.plugins",
		Constructor:   func() step.Step { return NewZshPluginsStep() },
	})

	r.Register(StepDefinition{
		Name:          "shell-default",
		Description:   "Set default shell",
		Category:      "shell",
		Type:          "atomic",
		Platforms:     []string{"any"},
		Provides:      []string{"shell-default"},
		Requires:      nil,
		ConfigSection: "shell.default",
		Constructor:   func() step.Step { return NewShellDefaultStep() },
	})

	// --- System (macOS) ---

	r.Register(StepDefinition{
		Name:          "macos-defaults",
		Description:   "Write macOS defaults preferences",
		Category:      "system",
		Type:          "batch",
		Platforms:     []string{"darwin"},
		Provides:      []string{"macos-defaults"},
		Requires:      nil,
		ConfigSection: "defaults",
		Constructor:   func() step.Step { return NewMacOSDefaultsStep() },
	})

	r.Register(StepDefinition{
		Name:          "dock-layout",
		Description:   "Configure macOS Dock layout",
		Category:      "system",
		Type:          "batch",
		Platforms:     []string{"darwin"},
		Provides:      []string{"dock-layout"},
		Requires:      []string{"homebrew"},
		ConfigSection: "dock",
		Constructor:   func() step.Step { return NewDockLayoutStep() },
	})

	r.Register(StepDefinition{
		Name:          "machine-name",
		Description:   "Set macOS machine name",
		Category:      "system",
		Type:          "atomic",
		Platforms:     []string{"darwin"},
		Provides:      []string{"machine-name"},
		Requires:      nil,
		ConfigSection: "machine.hostname",
		Constructor:   func() step.Step { return NewMachineNameStep() },
	})

	// --- System (Linux) ---

	r.Register(StepDefinition{
		Name:          "gsettings",
		Description:   "Write GNOME gsettings preferences",
		Category:      "system",
		Type:          "batch",
		Platforms:     []string{"ubuntu"},
		Provides:      []string{"gsettings"},
		Requires:      nil,
		ConfigSection: "defaults",
		Constructor:   func() step.Step { return NewGSettingsStep() },
	})

	// --- Generic ---

	r.Register(StepDefinition{
		Name:          "directories",
		Description:   "Create directories from config",
		Category:      "generic",
		Type:          "batch",
		Platforms:     []string{"any"},
		Provides:      []string{"directories"},
		Requires:      nil,
		ConfigSection: "directories",
		Constructor:   func() step.Step { return NewDirectoriesStep() },
	})

	r.Register(StepDefinition{
		Name:          "git-config",
		Description:   "Set global git configuration",
		Category:      "generic",
		Type:          "atomic",
		Platforms:     []string{"any"},
		Provides:      []string{"git-config"},
		Requires:      nil,
		ConfigSection: "identity",
		Constructor:   func() step.Step { return NewGitConfigStep() },
	})

	r.Register(StepDefinition{
		Name:        "ssh-keys",
		Description: "Generate SSH keys",
		Category:    "generic",
		Type:        "atomic",
		Platforms:   []string{"any"},
		Provides:    []string{"ssh-keys"},
		Requires:    nil,
		Constructor: func() step.Step { return NewSSHKeysStep() },
	})
}
