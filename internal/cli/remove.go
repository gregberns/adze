package cli

import (
	"fmt"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/spf13/cobra"
)

// removeDeps groups the dependencies needed by the remove command.
type removeDeps struct {
	adapter    adapter.Adapter
	loadConfig func(path string) (*config.Config, error)
}

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <pkg>",
		Short: "Uninstall a package and remove it from config",
		Long:  "Atomic operation: uninstall the package AND remove it from the config file.",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove(nil),
	}
	return cmd
}

// runRemove returns the RunE function for the remove command.
func runRemove(deps *removeDeps) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		gf := ResolveGlobalFlags(cmd)
		pkgName := args[0]

		if deps == nil {
			deps = &removeDeps{
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

		// Determine package properties from config (if present).
		entry, listType := findPackageInConfig(cfg, pkgName)
		pkg := adapter.Package{Name: pkgName}
		if entry != nil {
			pkg = packageEntryToAdapterPkg(entry, listType)
		}

		// Remove from machine.
		if err := deps.adapter.PackageRemove(pkg); err != nil {
			return &cmdExitError{
				Code: ExitExecFailure,
				Msg:  fmt.Sprintf("remove failed: %v", err),
			}
		}

		// Remove from config.
		if err := removePackageFromConfig(cfgPath, cfg, pkgName); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: %s removed but config update failed: %v\n", pkgName, err)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed %s and updated %s\n", pkgName, cfgPath)
		return nil
	}
}
