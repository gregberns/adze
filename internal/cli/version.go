package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd creates the "version" subcommand.
func newVersionCmd(version, commit, date string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print the adze version, commit hash, and build date.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "adze %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}
