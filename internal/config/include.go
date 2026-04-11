package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxIncludeDepth = 10

// LoadConfig loads a config file, resolving all includes and merging.
// configPath is the path to the main config file.
// isURL indicates if the config was loaded from a URL (disables includes).
func LoadConfig(configPath string, isURL bool) (*Config, []ValidationError, []ValidationWarning, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading config: %w", err)
	}

	root, err := parseRawYAML(data)
	if err != nil {
		return nil, nil, nil, err
	}

	if root == nil {
		return nil, []ValidationError{
			newError(E001, "", "config: top-level document must be a YAML mapping"),
		}, nil, nil
	}

	includePaths := getIncludePaths(root)

	// Check: URL-sourced configs cannot use includes
	if isURL && len(includePaths) > 0 {
		return nil, nil, nil, fmt.Errorf(
			"Error: URL-sourced configs cannot use includes\n  Config loaded from: %s\n  Includes require local file paths",
			configPath,
		)
	}

	if len(includePaths) == 0 {
		// No includes: parse directly
		cfg, errs, warns := parseNodeToConfig(root)
		return cfg, errs, warns, nil
	}

	// Resolve includes
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolving config path: %w", err)
	}

	visited := map[string]bool{absPath: true}
	chain := []string{absPath}
	var mergeWarnings []MergeWarning

	mergedInclude, err := resolveIncludes(root, absPath, visited, chain, 1, &mergeWarnings)
	if err != nil {
		return nil, nil, nil, err
	}

	// Remove include field from main config before merging
	mainWithoutInclude := removeIncludeField(root)

	// Merge: includes form the base, main config overrides
	var finalNode *yaml.Node
	if mergedInclude != nil {
		finalNode = mergeNodes(mergedInclude, mainWithoutInclude, "<includes>", configPath, "", &mergeWarnings)
	} else {
		finalNode = mainWithoutInclude
	}

	if finalNode == nil {
		return nil, []ValidationError{
			newError(E001, "", "config: top-level document must be a YAML mapping"),
		}, nil, nil
	}

	// Parse the final merged node into Config
	cfg, errs, warns := parseNodeToConfig(finalNode)

	// Convert merge warnings to validation warnings
	for _, mw := range mergeWarnings {
		warns = append(warns, ValidationWarning{
			Code:    W003,
			Field:   mw.FieldPath,
			Message: mw.String(),
		})
	}

	return cfg, errs, warns, nil
}

// resolveIncludes processes the include directives of a YAML node, recursively
// loading and merging included files. It returns the merged result of all includes
// (without the including file's own content, except what comes from its includes).
func resolveIncludes(root *yaml.Node, currentFile string, visited map[string]bool, chain []string, depth int, warnings *[]MergeWarning) (*yaml.Node, error) {
	includePaths := getIncludePaths(root)
	if len(includePaths) == 0 {
		return nil, nil
	}

	currentDir := filepath.Dir(currentFile)
	var merged *yaml.Node

	for idx, relPath := range includePaths {
		// Check for remote URLs
		if isHTTPURL(relPath) {
			return nil, fmt.Errorf(
				"Error: remote includes are not supported\n  Path: %s\n  Use a local file path instead",
				relPath,
			)
		}

		// Resolve relative path
		absPath := filepath.Join(currentDir, relPath)
		absPath, err := filepath.Abs(absPath)
		if err != nil {
			return nil, fmt.Errorf("resolving include path: %w", err)
		}

		// Check depth limit
		if depth >= maxIncludeDepth {
			newChain := append(chain, absPath)
			return nil, fmt.Errorf(
				"Error: include depth limit (%d) exceeded\n  Chain: %s",
				maxIncludeDepth,
				strings.Join(newChain, " → "),
			)
		}

		// Check for circular includes
		if visited[absPath] {
			return nil, formatCircularError(chain, absPath)
		}

		// Check file exists
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			lineNum := getIncludeLineNumber(root, idx)
			return nil, fmt.Errorf(
				"Error: include file not found\n  Referenced in: %s (line %d)\n  Path: %s\n  Resolved to: %s",
				currentFile, lineNum, relPath, absPath,
			)
		}

		// Read and parse the included file
		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading include file %s: %w", absPath, err)
		}

		includeRoot, err := parseRawYAML(data)
		if err != nil {
			return nil, fmt.Errorf("parsing include file %s: %w", absPath, err)
		}

		if includeRoot == nil {
			// Empty include file, skip
			continue
		}

		// Mark as visited
		visited[absPath] = true
		newChain := append(append([]string{}, chain...), absPath)

		// Recursively resolve this file's own includes
		subMerged, err := resolveIncludes(includeRoot, absPath, visited, newChain, depth+1, warnings)
		if err != nil {
			return nil, err
		}

		// Build the complete content for this include: sub-includes merged with this file's content
		includeContent := removeIncludeField(includeRoot)
		var resolvedInclude *yaml.Node
		if subMerged != nil {
			resolvedInclude = mergeNodes(subMerged, includeContent, "<sub-includes>", absPath, "", warnings)
		} else {
			resolvedInclude = includeContent
		}

		// Merge into accumulated result
		if merged == nil {
			merged = resolvedInclude
		} else if resolvedInclude != nil {
			merged = mergeNodes(merged, resolvedInclude, "<earlier includes>", absPath, "", warnings)
		}
	}

	return merged, nil
}

// formatCircularError formats a circular include error message.
func formatCircularError(chain []string, cyclePath string) error {
	var lines []string
	lines = append(lines, "Error: circular include detected")

	// Find the start of the cycle in the chain
	cycleStart := -1
	for i, p := range chain {
		if p == cyclePath {
			cycleStart = i
			break
		}
	}

	if cycleStart >= 0 {
		// Show the chain from the cycle start
		for i := cycleStart; i < len(chain); i++ {
			next := ""
			if i+1 < len(chain) {
				next = chain[i+1]
			} else {
				next = cyclePath
			}
			suffix := ""
			if i == len(chain)-1 {
				suffix = "  \u2190 cycle"
			}
			lines = append(lines, fmt.Sprintf("  %s includes %s%s", chain[i], next, suffix))
		}
	} else {
		// Fallback: show entire chain
		for i := 0; i < len(chain); i++ {
			next := ""
			if i+1 < len(chain) {
				next = chain[i+1]
			} else {
				next = cyclePath
			}
			suffix := ""
			if i == len(chain)-1 {
				suffix = "  \u2190 cycle"
			}
			lines = append(lines, fmt.Sprintf("  %s includes %s%s", chain[i], next, suffix))
		}
	}

	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}
