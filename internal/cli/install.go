package cli

import (
	"fmt"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/spf13/cobra"
)

// installDeps groups the dependencies needed by the install command.
type installDeps struct {
	adapter    adapter.Adapter
	loadConfig func(path string) (*config.Config, error)
}

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <pkg>",
		Short: "Install a package and add it to config",
		Long:  "Atomic operation: install the package on the machine AND add it to the config file.",
		Args:  cobra.ExactArgs(1),
		RunE:  runInstall(nil),
	}
	cmd.Flags().Bool("cask", false, "install as a Homebrew cask (macOS only)")
	return cmd
}

// runInstall returns the RunE function for the install command.
func runInstall(deps *installDeps) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		gf := ResolveGlobalFlags(cmd)
		isCask, _ := cmd.Flags().GetBool("cask")
		pkgName := args[0]

		if deps == nil {
			deps = &installDeps{
				loadConfig: loadConfigFromFile,
			}
			plat, err := adapter.DetectPlatform()
			if err != nil {
				return fmt.Errorf("detecting platform: %w", err)
			}
			deps.adapter = createAdapter(plat)
		}

		cfgPath, err := ResolveConfig(gf.Config)
		if err != nil {
			return err
		}

		cfg, err := deps.loadConfig(cfgPath)
		if err != nil {
			return err
		}

		// Install the package on the machine.
		pkg := adapter.Package{
			Name: pkgName,
			Cask: isCask,
		}
		if err := deps.adapter.PackageInstall(pkg); err != nil {
			return &cmdExitError{
				Code: ExitExecFailure,
				Msg:  fmt.Sprintf("install failed: %v", err),
			}
		}

		// Add package to config.
		if err := addPackageToConfig(cfgPath, cfg, pkgName, isCask); err != nil {
			// Package was installed but config update failed.
			// Report success with warning.
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: %s installed but config update failed: %v\n", pkgName, err)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Installed %s and added to %s\n", pkgName, cfgPath)
		return nil
	}
}

// addPackageToConfig adds a package to the appropriate list in the config file.
func addPackageToConfig(cfgPath string, cfg *config.Config, pkgName string, isCask bool) error {
	// Check if already in config.
	if isCask {
		for _, p := range cfg.Packages.Cask {
			if p.Name == pkgName {
				return nil // already present
			}
		}
		cfg.Packages.Cask = append(cfg.Packages.Cask, config.PackageEntry{Name: pkgName})
	} else if cfg.Platform == "ubuntu" || cfg.Platform == "debian" {
		for _, p := range cfg.Packages.Apt {
			if p.Name == pkgName {
				return nil
			}
		}
		cfg.Packages.Apt = append(cfg.Packages.Apt, config.PackageEntry{Name: pkgName})
	} else {
		for _, p := range cfg.Packages.Brew {
			if p.Name == pkgName {
				return nil
			}
		}
		cfg.Packages.Brew = append(cfg.Packages.Brew, config.PackageEntry{Name: pkgName})
	}

	return writeConfigToFile(cfgPath, cfg)
}

// removePackageFromConfig removes a package from the config file.
func removePackageFromConfig(cfgPath string, cfg *config.Config, pkgName string) error {
	modified := false

	// Check brew list.
	for i, p := range cfg.Packages.Brew {
		if p.Name == pkgName {
			cfg.Packages.Brew = append(cfg.Packages.Brew[:i], cfg.Packages.Brew[i+1:]...)
			modified = true
			break
		}
	}

	// Check cask list.
	if !modified {
		for i, p := range cfg.Packages.Cask {
			if p.Name == pkgName {
				cfg.Packages.Cask = append(cfg.Packages.Cask[:i], cfg.Packages.Cask[i+1:]...)
				modified = true
				break
			}
		}
	}

	// Check apt list.
	if !modified {
		for i, p := range cfg.Packages.Apt {
			if p.Name == pkgName {
				cfg.Packages.Apt = append(cfg.Packages.Apt[:i], cfg.Packages.Apt[i+1:]...)
				modified = true
				break
			}
		}
	}

	if !modified {
		return nil // not in config, nothing to do
	}

	return writeConfigToFile(cfgPath, cfg)
}

// findPackageInConfig checks if a package exists in any config package list
// and returns its PackageEntry and which list it's in.
func findPackageInConfig(cfg *config.Config, pkgName string) (*config.PackageEntry, string) {
	for i := range cfg.Packages.Brew {
		if cfg.Packages.Brew[i].Name == pkgName {
			return &cfg.Packages.Brew[i], "brew"
		}
	}
	for i := range cfg.Packages.Cask {
		if cfg.Packages.Cask[i].Name == pkgName {
			return &cfg.Packages.Cask[i], "cask"
		}
	}
	for i := range cfg.Packages.Apt {
		if cfg.Packages.Apt[i].Name == pkgName {
			return &cfg.Packages.Apt[i], "apt"
		}
	}
	return nil, ""
}

// packageEntryToAdapterPkg converts a config.PackageEntry to an adapter.Package.
func packageEntryToAdapterPkg(entry *config.PackageEntry, listType string) adapter.Package {
	return adapter.Package{
		Name:    entry.Name,
		Version: entry.Version,
		Pinned:  entry.Pinned,
		Cask:    listType == "cask",
	}
}

// allConfigPackages returns all packages from all lists with their list type.
func allConfigPackages(cfg *config.Config) []struct {
	Entry    config.PackageEntry
	ListType string
} {
	var all []struct {
		Entry    config.PackageEntry
		ListType string
	}
	for _, p := range cfg.Packages.Brew {
		all = append(all, struct {
			Entry    config.PackageEntry
			ListType string
		}{p, "brew"})
	}
	for _, p := range cfg.Packages.Cask {
		all = append(all, struct {
			Entry    config.PackageEntry
			ListType string
		}{p, "cask"})
	}
	for _, p := range cfg.Packages.Apt {
		all = append(all, struct {
			Entry    config.PackageEntry
			ListType string
		}{p, "apt"})
	}
	return all
}

