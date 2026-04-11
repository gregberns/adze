package cli

import (
	"fmt"

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
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newGraphCmd())
	rootCmd.AddCommand(newRenderCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newStepCmd())

	return rootCmd
}

// stubRun returns a RunE function that prints "not implemented" and exits.
func stubRun(name string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("%s: not yet implemented", name)
	}
}

// --- Subcommand constructors (stubs) ---

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scan the current machine and generate a config",
		Long:  "Scan the current machine and generate a config YAML. Detects installed packages, settings, and preferences.",
		RunE:  stubRun("init"),
	}
	cmd.Flags().String("config", "", "write output to specified file (default: stdout)")
	return cmd
}

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what changes would be made",
		Long:  "Resolve the dependency graph, check current state against config, and show what would change. No mutations are performed.",
		RunE:  stubRun("plan"),
	}
	return cmd
}

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the configuration to the machine",
		Long:  "Execute the plan. Pre-flight validation runs first, then each step is applied in dependency order.",
		RunE:  stubRun("apply"),
	}
	cmd.Flags().Bool("yes", false, "non-interactive mode (skip prompts, assume yes)")
	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Compare config against current machine state",
		Long:  "Compare the config against the current machine state and show drift. Items on the machine but not in config, in config but not installed, or with differing values are reported.",
		RunE:  stubRun("status"),
	}
	return cmd
}

func newCaptureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Detect packages not in the config",
		Long:  "Detect packages on the machine that are not in the config. With --all, write all detected packages to the config file.",
		RunE:  stubRun("capture"),
	}
	cmd.Flags().Bool("all", false, "write all detected packages to the config file")
	return cmd
}

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <pkg>",
		Short: "Install a package and add it to config",
		Long:  "Atomic operation: install the package on the machine AND add it to the config file.",
		Args:  cobra.ExactArgs(1),
		RunE:  stubRun("install"),
	}
	cmd.Flags().Bool("cask", false, "install as a Homebrew cask (macOS only)")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <pkg>",
		Short: "Uninstall a package and remove it from config",
		Long:  "Atomic operation: uninstall the package AND remove it from the config file.",
		Args:  cobra.ExactArgs(1),
		RunE:  stubRun("remove"),
	}
	return cmd
}

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade non-pinned packages",
		Long:  "Upgrade non-pinned packages to latest versions. Packages with pinned: true are never upgraded.",
		RunE:  stubRun("upgrade"),
	}
	cmd.Flags().Bool("all", false, "include casks with auto-updates")
	return cmd
}

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the config without execution",
		Long:  "Deep validation of the config: YAML syntax, schema compliance, include resolution, dependency graph, secret declarations, and platform compatibility.",
		RunE:  stubRun("validate"),
	}
	return cmd
}

func newGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Visualize the dependency graph",
		Long:  "Display the dependency graph in text tree format (default) or DOT format for Graphviz.",
		RunE:  stubRun("graph"),
	}
	cmd.Flags().String("format", "text", "output format: text or dot")
	return cmd
}

func newRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Generate a standalone bash script from config",
		Long:  "Generate a standalone bash script from the resolved config. The script can be run independently without adze.",
		RunE:  stubRun("render"),
	}
	cmd.Flags().StringP("output", "o", "", "write to file (default: stdout)")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Dump full context for AI agent review",
		Long:  "Dump full context about the machine and config for AI agent review. Output is always to stdout.",
		RunE:  stubRun("doctor"),
	}
	return cmd
}

func newStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "step",
		Short: "Manage built-in and custom steps",
		Long:  "List, inspect, and scaffold steps.",
	}

	cmd.AddCommand(newStepListCmd())
	cmd.AddCommand(newStepInfoCmd())
	cmd.AddCommand(newStepAddCmd())

	return cmd
}

func newStepListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all built-in steps",
		Long:  "List all built-in steps, optionally filtered by platform.",
		RunE:  stubRun("step list"),
	}
	cmd.Flags().String("platform", "", "filter by platform: darwin, ubuntu, or any")
	return cmd
}

func newStepInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show details for a built-in step",
		Long:  "Show full details for a built-in step including its provides, requires, platform, and commands.",
		Args:  cobra.ExactArgs(1),
		RunE:  stubRun("step info"),
	}
	return cmd
}

func newStepAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Scaffold a custom step in the config",
		Long:  "Scaffold a custom step definition in the config's custom_steps section.",
		Args:  cobra.ExactArgs(1),
		RunE:  stubRun("step add"),
	}
	return cmd
}
