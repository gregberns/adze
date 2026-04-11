package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/spf13/cobra"
)

// DriftKind indicates the type of drift detected.
type DriftKind string

const (
	DriftAdded   DriftKind = "+"
	DriftMissing DriftKind = "-"
	DriftChanged DriftKind = "~"
)

// DriftEntry represents a single drift item for status output.
type DriftEntry struct {
	Kind     DriftKind `json:"kind"`
	Category string    `json:"category"`
	Name     string    `json:"name"`
	Detail   string    `json:"detail"`
	Config   string    `json:"config,omitempty"`
	Actual   string    `json:"actual,omitempty"`
}

// StatusResult holds the full status comparison result.
type StatusResult struct {
	ConfigFile string       `json:"config_file"`
	InSync     bool         `json:"in_sync"`
	Drift      []DriftEntry `json:"drift"`
}

// statusDeps groups the dependencies needed by the status command, enabling test injection.
type statusDeps struct {
	adapter    adapter.Adapter
	loadConfig func(path string) (*config.Config, error)
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Compare config against current machine state",
		Long:  "Compare the config against the current machine state and show drift. Items on the machine but not in config, in config but not installed, or with differing values are reported.",
		RunE:  runStatus(nil),
	}
	return cmd
}

// runStatus returns the RunE function for the status command.
// If deps is nil, real dependencies are used.
func runStatus(deps *statusDeps) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		gf := ResolveGlobalFlags(cmd)
		mode := DetectOutputMode(gf.JSON)

		if deps == nil {
			deps = &statusDeps{
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

		result, err := computeStatus(cfg, cfgPath, deps.adapter)
		if err != nil {
			return err
		}

		if mode == OutputJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				return err
			}
		} else {
			printStatusHuman(cmd, result)
		}

		if !result.InSync {
			return &cmdExitError{Code: ExitDriftDetected}
		}
		return nil
	}
}

// computeStatus compares config against the adapter's view of the machine.
func computeStatus(cfg *config.Config, cfgPath string, adp adapter.Adapter) (*StatusResult, error) {
	result := &StatusResult{
		ConfigFile: cfgPath,
		InSync:     true,
		Drift:      []DriftEntry{},
	}

	// --- Package drift ---
	installed, err := adp.ListLeaves()
	if err != nil {
		return nil, fmt.Errorf("listing installed packages: %w", err)
	}

	installedSet := make(map[string]bool, len(installed))
	for _, name := range installed {
		installedSet[name] = true
	}

	// Collect all config package names.
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

	// Packages on machine but not in config (+).
	var extraNames []string
	for _, name := range installed {
		if !configPkgs[name] {
			extraNames = append(extraNames, name)
		}
	}
	sort.Strings(extraNames)
	for _, name := range extraNames {
		result.Drift = append(result.Drift, DriftEntry{
			Kind:     DriftAdded,
			Category: "packages",
			Name:     name,
			Detail:   "installed, not in config",
		})
		result.InSync = false
	}

	// Packages in config but not on machine (-).
	var missingNames []string
	allConfigEntries := make(map[string]bool)
	for _, p := range cfg.Packages.Brew {
		allConfigEntries[p.Name] = true
	}
	for _, p := range cfg.Packages.Cask {
		allConfigEntries[p.Name] = true
	}
	for _, p := range cfg.Packages.Apt {
		allConfigEntries[p.Name] = true
	}
	for name := range allConfigEntries {
		if !installedSet[name] {
			missingNames = append(missingNames, name)
		}
	}
	sort.Strings(missingNames)
	for _, name := range missingNames {
		result.Drift = append(result.Drift, DriftEntry{
			Kind:     DriftMissing,
			Category: "packages",
			Name:     name,
			Detail:   "in config, not installed",
		})
		result.InSync = false
	}

	// --- Defaults drift ---
	if cfg.Defaults != nil {
		var domains []string
		for domain := range cfg.Defaults {
			domains = append(domains, domain)
		}
		sort.Strings(domains)

		for _, domain := range domains {
			prefs := cfg.Defaults[domain]
			var keys []string
			for key := range prefs {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				dv := prefs[key]
				actual, err := adp.DefaultsRead(domain, key)
				if err != nil {
					// Key doesn't exist on machine -- treat as missing.
					result.Drift = append(result.Drift, DriftEntry{
						Kind:     DriftMissing,
						Category: "defaults",
						Name:     domain + "." + key,
						Detail:   "in config, not on machine",
						Config:   fmt.Sprintf("%v", dv.Value),
					})
					result.InSync = false
					continue
				}

				configStr := fmt.Sprintf("%v", dv.Value)
				if actual.Raw != configStr {
					result.Drift = append(result.Drift, DriftEntry{
						Kind:     DriftChanged,
						Category: "defaults",
						Name:     domain + "." + key,
						Detail:   "value differs",
						Config:   configStr,
						Actual:   actual.Raw,
					})
					result.InSync = false
				}
			}
		}
	}

	return result, nil
}

// printStatusHuman prints the status result in human-readable format.
func printStatusHuman(cmd *cobra.Command, result *StatusResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Comparing %s against current machine state...\n", result.ConfigFile)

	// Group drift by category.
	pkgDrift := []DriftEntry{}
	defaultsDrift := []DriftEntry{}
	for _, d := range result.Drift {
		switch d.Category {
		case "packages":
			pkgDrift = append(pkgDrift, d)
		case "defaults":
			defaultsDrift = append(defaultsDrift, d)
		}
	}

	if len(pkgDrift) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Packages:")
		for _, d := range pkgDrift {
			// Find max name length for alignment.
			fmt.Fprintf(w, "  %s %-16s %s\n", string(d.Kind), d.Name, d.Detail)
		}
	}

	if len(defaultsDrift) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Defaults:")
		for _, d := range defaultsDrift {
			if d.Kind == DriftChanged {
				fmt.Fprintf(w, "  %s %-32s config: %s    actual: %s\n",
					string(d.Kind), d.Name, d.Config, d.Actual)
			} else {
				fmt.Fprintf(w, "  %s %-32s %s\n", string(d.Kind), d.Name, d.Detail)
			}
		}
	}

	if result.InSync {
		fmt.Fprintf(w, "\nEverything: %s in sync\n", "\u2713")
	} else if len(pkgDrift) == 0 || len(defaultsDrift) == 0 {
		// Print "in sync" note for categories that have no drift.
		synced := []string{}
		if len(pkgDrift) == 0 {
			synced = append(synced, "packages")
		}
		if len(defaultsDrift) == 0 {
			synced = append(synced, "defaults")
		}
		if len(synced) > 0 {
			fmt.Fprintf(w, "\nEverything else: %s in sync\n", "\u2713")
		}
	}
}

// cmdExitError is an error type that carries an exit code for the process.
type cmdExitError struct {
	Code int
	Msg  string
}

func (e *cmdExitError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

// loadConfigFromFile reads and parses a config file from disk.
func loadConfigFromFile(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg, errs, _, parseErr := config.Parse(data)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, parseErr)
	}
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return nil, fmt.Errorf("config validation errors in %s:\n  %s", path, strings.Join(msgs, "\n  "))
	}
	if cfg == nil {
		return nil, fmt.Errorf("config %s parsed to nil", path)
	}

	return cfg, nil
}

// createAdapter creates the appropriate platform adapter.
func createAdapter(platform string) adapter.Adapter {
	switch platform {
	case "darwin":
		return adapter.NewDarwinAdapter(nil)
	case "ubuntu":
		return adapter.NewUbuntuAdapter(nil)
	default:
		return adapter.NewGenericAdapter(nil)
	}
}
