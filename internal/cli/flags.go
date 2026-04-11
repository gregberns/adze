package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// GlobalFlags holds the values of all global (persistent) flags.
type GlobalFlags struct {
	Config  string
	JSON    bool
	Verbose bool
	Quiet   bool
	NoColor bool
}

// RegisterGlobalFlags adds the global persistent flags to the root command.
// Flag precedence: explicit flag > environment variable > default.
func RegisterGlobalFlags(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()

	pf.StringP("config", "c", "", "path to config file (env: ADZE_CONFIG)")
	pf.Bool("json", false, "machine-readable JSON output")
	pf.BoolP("verbose", "v", false, "show command output (env: ADZE_LOG_LEVEL=debug)")
	pf.BoolP("quiet", "q", false, "errors only (env: ADZE_LOG_LEVEL=error)")
	pf.Bool("no-color", false, "disable color output (env: NO_COLOR)")
}

// ResolveGlobalFlags reads the flag values from the command, applying
// environment variable fallbacks where applicable.
func ResolveGlobalFlags(cmd *cobra.Command) GlobalFlags {
	var gf GlobalFlags

	gf.Config, _ = cmd.Flags().GetString("config")
	if gf.Config == "" {
		gf.Config = os.Getenv("ADZE_CONFIG")
	}

	gf.JSON, _ = cmd.Flags().GetBool("json")

	gf.Verbose, _ = cmd.Flags().GetBool("verbose")
	gf.Quiet, _ = cmd.Flags().GetBool("quiet")

	// Environment variable fallback for verbose/quiet via ADZE_LOG_LEVEL.
	if !cmd.Flags().Changed("verbose") && !cmd.Flags().Changed("quiet") {
		switch os.Getenv("ADZE_LOG_LEVEL") {
		case "debug":
			gf.Verbose = true
		case "error":
			gf.Quiet = true
		}
	}

	gf.NoColor, _ = cmd.Flags().GetBool("no-color")
	if !cmd.Flags().Changed("no-color") {
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			gf.NoColor = true
		}
	}

	return gf
}
