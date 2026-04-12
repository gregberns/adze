package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root cobra command with all subcommands registered.
func NewRootCmd(version, commit, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "adze",
		Short: "adze - machine configuration tool",
		Long: `adze shapes a raw machine into a configured, ready-to-use state.

Declarative YAML config, dependency-aware execution, bidirectional sync
between config and machine state.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	RegisterGlobalFlags(rootCmd)

	// Register all subcommands.
	rootCmd.AddCommand(newVersionCmd(version, commit, date))
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newPlanCmd())
	rootCmd.AddCommand(newApplyCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newCaptureCmd())
	rootCmd.AddCommand(newInstallCmd())
	rootCmd.AddCommand(newRemoveCmd())
	rootCmd.AddCommand(newUpgradeCmd())
	rootCmd.AddCommand(newValidateCmdImpl())
	rootCmd.AddCommand(newGraphCmdImpl())
	rootCmd.AddCommand(newRenderCmdImpl())
	rootCmd.AddCommand(newDoctorCmd(version))
	rootCmd.AddCommand(newStepCmd())

	return rootCmd
}
