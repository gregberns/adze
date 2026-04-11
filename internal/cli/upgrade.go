package cli

import (
	"encoding/json"
	"fmt"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/spf13/cobra"
)

// UpgradeResult holds the result of an upgrade operation.
type UpgradeResult struct {
	Upgraded []string `json:"upgraded"`
	Skipped  []string `json:"skipped"`
	Failed   []string `json:"failed"`
}

// upgradeDeps groups the dependencies needed by the upgrade command.
type upgradeDeps struct {
	adapter    adapter.Adapter
	loadConfig func(path string) (*config.Config, error)
}

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade non-pinned packages",
		Long:  "Upgrade non-pinned packages to latest versions. Packages with pinned: true are never upgraded.",
		RunE:  runUpgrade(nil),
	}
	cmd.Flags().Bool("all", false, "include casks with auto-updates")
	return cmd
}

// runUpgrade returns the RunE function for the upgrade command.
func runUpgrade(deps *upgradeDeps) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		gf := ResolveGlobalFlags(cmd)
		mode := DetectOutputMode(gf.JSON)
		includeAll, _ := cmd.Flags().GetBool("all")

		if deps == nil {
			deps = &upgradeDeps{
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

		result := &UpgradeResult{
			Upgraded: []string{},
			Skipped:  []string{},
			Failed:   []string{},
		}

		// Collect all packages to consider.
		allPkgs := allConfigPackages(cfg)

		for _, item := range allPkgs {
			entry := item.Entry
			listType := item.ListType

			// Skip casks unless --all is set.
			if listType == "cask" && !includeAll {
				result.Skipped = append(result.Skipped, entry.Name)
				continue
			}

			// Skip pinned packages.
			if entry.Pinned {
				result.Skipped = append(result.Skipped, entry.Name)
				continue
			}

			pkg := packageEntryToAdapterPkg(&entry, listType)
			if err := deps.adapter.PackageUpgrade(pkg); err != nil {
				result.Failed = append(result.Failed, entry.Name)
				if !gf.Quiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "upgrade %s: %v\n", entry.Name, err)
				}
			} else {
				result.Upgraded = append(result.Upgraded, entry.Name)
			}
		}

		if mode == OutputJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		printUpgradeHuman(cmd, result)

		// Determine exit code.
		total := len(result.Upgraded) + len(result.Failed)
		if total == 0 {
			return nil // nothing to upgrade
		}
		if len(result.Failed) == total {
			return &cmdExitError{Code: ExitExecFailure, Msg: "all upgrades failed"}
		}
		if len(result.Failed) > 0 {
			return &cmdExitError{Code: ExitPartialSuccess, Msg: "some upgrades failed"}
		}
		return nil
	}
}

// printUpgradeHuman prints the upgrade result in human-readable format.
func printUpgradeHuman(cmd *cobra.Command, result *UpgradeResult) {
	w := cmd.OutOrStdout()

	if len(result.Upgraded) > 0 {
		fmt.Fprintln(w, "Upgraded:")
		for _, name := range result.Upgraded {
			fmt.Fprintf(w, "  %s %s\n", "\u2713", name)
		}
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintln(w, "Skipped (pinned or cask):")
		for _, name := range result.Skipped {
			fmt.Fprintf(w, "  - %s\n", name)
		}
	}

	if len(result.Failed) > 0 {
		fmt.Fprintln(w, "Failed:")
		for _, name := range result.Failed {
			fmt.Fprintf(w, "  %s %s\n", "\u2717", name)
		}
	}

	if len(result.Upgraded) == 0 && len(result.Failed) == 0 {
		fmt.Fprintln(w, "Nothing to upgrade.")
	}
}
