package cli

import (
	"fmt"
	"os"

	"github.com/gregberns/adze/internal/scan"
	"github.com/spf13/cobra"
)

// newInitCmd creates the "init" subcommand.
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scan the current machine and generate a config",
		Long:  "Scan the current machine and generate a config YAML. Detects installed packages, settings, and preferences.",
		RunE:  runInit,
	}
	cmd.Flags().String("config", "", "write output to specified file (default: stdout)")
	return cmd
}

// runInit implements the init command.
func runInit(cmd *cobra.Command, args []string) error {
	platform := scan.CurrentPlatform()

	result, err := scan.ScanMachine(platform)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	data, err := result.ToYAML()
	if err != nil {
		return fmt.Errorf("generating YAML: %w", err)
	}

	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Config written to %s\n", configPath)
		return nil
	}

	// Default: output to stdout.
	fmt.Fprint(cmd.OutOrStdout(), string(data))
	return nil
}
