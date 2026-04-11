package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// CaptureResult holds the result of a capture operation.
type CaptureResult struct {
	ConfigFile string   `json:"config_file"`
	Extras     []string `json:"extras"`
	Written    bool     `json:"written"`
}

// captureDeps groups the dependencies needed by the capture command.
type captureDeps struct {
	adapter    adapter.Adapter
	loadConfig func(path string) (*config.Config, error)
}

func newCaptureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Detect packages not in the config",
		Long:  "Detect packages on the machine that are not in the config. With --all, write all detected packages to the config file.",
		RunE:  runCapture(nil),
	}
	cmd.Flags().Bool("all", false, "write all detected packages to the config file")
	return cmd
}

// runCapture returns the RunE function for the capture command.
func runCapture(deps *captureDeps) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		gf := ResolveGlobalFlags(cmd)
		mode := DetectOutputMode(gf.JSON)
		writeAll, _ := cmd.Flags().GetBool("all")

		if deps == nil {
			deps = &captureDeps{
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

		result, err := computeCapture(cfg, cfgPath, deps.adapter)
		if err != nil {
			return err
		}

		if writeAll && len(result.Extras) > 0 {
			if err := writeCapturedPackages(cfgPath, result.Extras); err != nil {
				return fmt.Errorf("writing captured packages: %w", err)
			}
			result.Written = true
		}

		if mode == OutputJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		printCaptureHuman(cmd, result)
		return nil
	}
}

// computeCapture finds packages on the machine not present in the config.
func computeCapture(cfg *config.Config, cfgPath string, adp adapter.Adapter) (*CaptureResult, error) {
	installed, err := adp.ListLeaves()
	if err != nil {
		return nil, fmt.Errorf("listing installed packages: %w", err)
	}

	// Build set of all config package names.
	configPkgs := make(map[string]bool)
	for _, p := range cfg.Packages.Brew {
		configPkgs[p.Name] = true
	}
	for _, p := range cfg.Packages.Cask {
		configPkgs[p.Name] = true
	}
	for _, p := range cfg.Packages.Apt {
		configPkgs[p.Name] = true
	}

	var extras []string
	for _, name := range installed {
		if !configPkgs[name] {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)

	return &CaptureResult{
		ConfigFile: cfgPath,
		Extras:     extras,
	}, nil
}

// writeCapturedPackages reads the config YAML, adds the new packages to the
// brew list, and writes back.
func writeCapturedPackages(cfgPath string, extras []string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	// Parse existing YAML into a generic map to preserve structure.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing yaml: %w", err)
	}

	// Re-parse to get the config struct (for platform detection).
	cfg, _, _, parseErr := config.Parse(data)
	if parseErr != nil {
		return fmt.Errorf("parsing config: %w", parseErr)
	}

	// Determine which package list to append to based on platform.
	listKey := "brew"
	if cfg.Platform == "ubuntu" || cfg.Platform == "debian" {
		listKey = "apt"
	}

	// Build existing package set to avoid duplicates.
	existing := make(map[string]bool)
	switch listKey {
	case "brew":
		for _, p := range cfg.Packages.Brew {
			existing[p.Name] = true
		}
	case "apt":
		for _, p := range cfg.Packages.Apt {
			existing[p.Name] = true
		}
	}

	// Filter extras to only truly new packages.
	var toAdd []string
	for _, name := range extras {
		if !existing[name] {
			toAdd = append(toAdd, name)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	// Append packages to the config struct and re-serialize.
	switch listKey {
	case "brew":
		for _, name := range toAdd {
			cfg.Packages.Brew = append(cfg.Packages.Brew, config.PackageEntry{Name: name})
		}
	case "apt":
		for _, name := range toAdd {
			cfg.Packages.Apt = append(cfg.Packages.Apt, config.PackageEntry{Name: name})
		}
	}

	return writeConfigToFile(cfgPath, cfg)
}

// printCaptureHuman prints the capture result in human-readable format.
func printCaptureHuman(cmd *cobra.Command, result *CaptureResult) {
	w := cmd.OutOrStdout()

	if len(result.Extras) == 0 {
		fmt.Fprintln(w, "No extra packages detected. Config is up to date.")
		return
	}

	fmt.Fprintf(w, "Packages on machine but not in %s:\n", result.ConfigFile)
	for _, name := range result.Extras {
		fmt.Fprintf(w, "  + %s\n", name)
	}

	if result.Written {
		fmt.Fprintf(w, "\nAdded %d package(s) to %s\n", len(result.Extras), result.ConfigFile)
	} else {
		fmt.Fprintf(w, "\nRun with --all to add these to the config.\n")
	}
}

// writeConfigToFile serializes a config and writes it to the specified path.
func writeConfigToFile(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
