package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	cfgpkg "github.com/gregberns/adze/internal/config"
)

// ResolveConfig determines which config file to use.
//
// If flagValue is non-empty it is returned directly. Otherwise auto-detection
// scans the current working directory for YAML files that parse as valid adze
// configs whose platform field matches the runtime OS (or "any").
//
// Returns the resolved path or an error with guidance.
func ResolveConfig(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return autoDetectConfig(".")
}

// autoDetectConfig scans dir for *.yaml and *.yml files, parses each, and
// returns the single file whose platform matches the current OS.
func autoDetectConfig(dir string) (string, error) {
	return autoDetectConfigWithPlatform(dir, currentPlatform())
}

// autoDetectConfigWithPlatform is the testable core of auto-detection.
func autoDetectConfigWithPlatform(dir string, platform string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("scanning directory: %w", err)
	}

	var candidates []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // skip unreadable files
		}

		cfg, _, _, parseErr := cfgpkg.Parse(data)
		if parseErr != nil {
			continue // skip files with YAML syntax errors
		}
		if cfg == nil {
			continue // skip files that didn't parse into a config
		}

		// Must have a platform field to be considered an adze config.
		if cfg.Platform == "" {
			continue
		}

		if cfg.Platform == platform || cfg.Platform == "any" {
			candidates = append(candidates, path)
		}
	}

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("no adze config file found in current directory; run 'adze init' to create one or specify --config")
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("multiple adze config files found: %s; specify one with --config", strings.Join(candidates, ", "))
	}
}

// currentPlatform maps runtime.GOOS to the platform values used in config files.
func currentPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		// Could be ubuntu, debian, etc. For now we check both.
		return runtime.GOOS
	default:
		return runtime.GOOS
	}
}
