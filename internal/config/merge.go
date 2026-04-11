package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// MergeWarning represents a warning emitted during the merge process.
type MergeWarning struct {
	FieldPath    string
	BaseFile     string
	BaseType     string
	OverrideFile string
	OverrideType string
}

// String returns the formatted warning message.
func (w MergeWarning) String() string {
	return fmt.Sprintf(
		"Warning: type mismatch at %q\n  %s defines it as %s\n  %s defines it as %s\n  Using %s's value",
		w.FieldPath, w.BaseFile, w.BaseType, w.OverrideFile, w.OverrideType, w.OverrideFile,
	)
}

// mergeNodes deep-merges two YAML nodes. base is the lower-priority node,
// override is the higher-priority node. The merge operates recursively.
// baseFile and overrideFile are used for warning messages.
// fieldPath tracks the current position in the document for warnings.
func mergeNodes(base, override *yaml.Node, baseFile, overrideFile, fieldPath string, warnings *[]MergeWarning) *yaml.Node {
	// If either is nil, return the other
	if base == nil {
		return cloneNode(override)
	}
	if override == nil {
		return cloneNode(base)
	}

	// Handle Document nodes: unwrap to content
	if base.Kind == yaml.DocumentNode && len(base.Content) > 0 {
		base = base.Content[0]
	}
	if override.Kind == yaml.DocumentNode && len(override.Content) > 0 {
		override = override.Content[0]
	}

	// Null override removes the key entirely
	if override.Tag == "!!null" || (override.Kind == yaml.ScalarNode && override.Value == "null" && override.Tag == "!!null") {
		return nil
	}

	// Check for type mismatch
	if base.Kind != override.Kind {
		if warnings != nil {
			*warnings = append(*warnings, MergeWarning{
				FieldPath:    fieldPath,
				BaseFile:     baseFile,
				BaseType:     yamlNodeTypeName(base),
				OverrideFile: overrideFile,
				OverrideType: yamlNodeTypeName(override),
			})
		}
		return cloneNode(override)
	}

	switch base.Kind {
	case yaml.MappingNode:
		return mergeMappings(base, override, baseFile, overrideFile, fieldPath, warnings)
	case yaml.SequenceNode:
		return mergeSequences(base, override, baseFile, overrideFile, fieldPath, warnings)
	default:
		// Scalars: override wins
		return cloneNode(override)
	}
}

// mergeMappings deep-merges two mapping nodes.
func mergeMappings(base, override *yaml.Node, baseFile, overrideFile, fieldPath string, warnings *[]MergeWarning) *yaml.Node {
	result := &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     "!!map",
		Style:   base.Style,
		Content: make([]*yaml.Node, 0),
	}

	// Build index of override keys
	overrideKeys := make(map[string]int) // key -> index of value in override.Content
	for i := 0; i < len(override.Content)-1; i += 2 {
		overrideKeys[override.Content[i].Value] = i + 1
	}

	// Track which override keys have been processed
	processedOverride := make(map[string]bool)

	// Process base keys first (preserves base ordering for keys only in base)
	for i := 0; i < len(base.Content)-1; i += 2 {
		keyNode := base.Content[i]
		baseVal := base.Content[i+1]
		key := keyNode.Value

		childPath := key
		if fieldPath != "" {
			childPath = fieldPath + "." + key
		}

		if overrideIdx, ok := overrideKeys[key]; ok {
			processedOverride[key] = true
			overrideVal := override.Content[overrideIdx]

			// Check if override is null -> remove the key
			if overrideVal.Tag == "!!null" {
				continue
			}

			merged := mergeNodes(baseVal, overrideVal, baseFile, overrideFile, childPath, warnings)
			if merged != nil {
				result.Content = append(result.Content, cloneNode(keyNode), merged)
			}
		} else {
			// Key only in base: keep it
			result.Content = append(result.Content, cloneNode(keyNode), cloneNode(baseVal))
		}
	}

	// Add override keys not in base
	for i := 0; i < len(override.Content)-1; i += 2 {
		keyNode := override.Content[i]
		key := keyNode.Value
		if processedOverride[key] {
			continue
		}

		overrideVal := override.Content[i+1]
		// Skip null values for keys not in base
		if overrideVal.Tag == "!!null" {
			continue
		}

		result.Content = append(result.Content, cloneNode(keyNode), cloneNode(overrideVal))
	}

	return result
}

// mergeSequences concatenates two sequences and deduplicates.
func mergeSequences(base, override *yaml.Node, baseFile, overrideFile, fieldPath string, warnings *[]MergeWarning) *yaml.Node {
	result := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Style:   base.Style,
		Content: make([]*yaml.Node, 0),
	}

	// Determine the dedup strategy by examining items
	allItems := make([]*yaml.Node, 0, len(base.Content)+len(override.Content))
	allItems = append(allItems, base.Content...)
	allItems = append(allItems, override.Content...)

	if len(allItems) == 0 {
		return result
	}

	// Classify: do any items have a "name" field (or could be interpreted as named)?
	hasNamedItems := false
	hasStringItems := false
	for _, item := range allItems {
		if item.Kind == yaml.MappingNode {
			if getNameField(item) != "" {
				hasNamedItems = true
			}
		} else if item.Kind == yaml.ScalarNode && item.Tag != "!!null" {
			hasStringItems = true
		}
	}

	if hasNamedItems || (hasStringItems && hasNamedItems) {
		// Mixed or named: dedup by name
		result.Content = deduplicateByName(base.Content, override.Content)
	} else if hasStringItems {
		// Pure string items: dedup by exact match
		result.Content = deduplicateStrings(base.Content, override.Content)
	} else {
		// Items without name field: concatenate without dedup
		for _, item := range allItems {
			result.Content = append(result.Content, cloneNode(item))
		}
	}

	return result
}

// deduplicateStrings concatenates base and override string items,
// keeping the first occurrence of each value.
func deduplicateStrings(base, override []*yaml.Node) []*yaml.Node {
	seen := make(map[string]bool)
	var result []*yaml.Node

	for _, item := range base {
		if item.Kind == yaml.ScalarNode {
			if !seen[item.Value] {
				seen[item.Value] = true
				result = append(result, cloneNode(item))
			}
		} else {
			result = append(result, cloneNode(item))
		}
	}

	for _, item := range override {
		if item.Kind == yaml.ScalarNode {
			if !seen[item.Value] {
				seen[item.Value] = true
				result = append(result, cloneNode(item))
			}
		} else {
			result = append(result, cloneNode(item))
		}
	}

	return result
}

// deduplicateByName deduplicates items by their "name" field.
// String items are normalized to objects. Later (override) replaces earlier (base).
func deduplicateByName(base, override []*yaml.Node) []*yaml.Node {
	type entry struct {
		name string
		node *yaml.Node
	}

	// Build ordered list from base, then override replaces
	var entries []entry
	nameIndex := make(map[string]int) // name -> index in entries

	processItems := func(items []*yaml.Node, isOverride bool) {
		for _, item := range items {
			name := ""
			node := item

			if item.Kind == yaml.ScalarNode && item.Tag != "!!null" {
				// String form: normalize to object
				name = item.Value
				node = stringToNamedObject(item.Value)
			} else if item.Kind == yaml.MappingNode {
				name = getNameField(item)
			}

			if name == "" {
				// No name field: just append
				entries = append(entries, entry{name: "", node: cloneNode(node)})
				continue
			}

			if idx, ok := nameIndex[name]; ok {
				// Replace existing entry
				entries[idx] = entry{name: name, node: cloneNode(node)}
			} else {
				nameIndex[name] = len(entries)
				entries = append(entries, entry{name: name, node: cloneNode(node)})
			}
		}
	}

	processItems(base, false)
	processItems(override, true)

	result := make([]*yaml.Node, 0, len(entries))
	for _, e := range entries {
		result = append(result, e.node)
	}
	return result
}

// getNameField returns the value of the "name" field from a mapping node, or "".
func getNameField(node *yaml.Node) string {
	if node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == "name" {
			return node.Content[i+1].Value
		}
	}
	return ""
}

// stringToNamedObject converts a scalar string to a mapping node with a "name" field.
func stringToNamedObject(name string) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
		},
	}
}

// cloneNode creates a deep copy of a YAML node.
func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	n := &yaml.Node{
		Kind:        node.Kind,
		Style:       node.Style,
		Tag:         node.Tag,
		Value:       node.Value,
		Anchor:      node.Anchor,
		Alias:       nil, // don't copy alias references
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
		Line:        node.Line,
		Column:      node.Column,
	}
	if len(node.Content) > 0 {
		n.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			n.Content[i] = cloneNode(child)
		}
	}
	return n
}

// yamlNodeTypeName returns a human-readable name for a YAML node kind.
func yamlNodeTypeName(node *yaml.Node) string {
	switch node.Kind {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.AliasNode:
		return "alias"
	case yaml.DocumentNode:
		return "document"
	default:
		return "unknown"
	}
}

// parseRawYAML parses YAML bytes into a raw yaml.Node (the document root).
func parseRawYAML(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		// Empty document
		return nil, nil
	}
	return doc.Content[0], nil
}

// parseNodeToConfig parses a raw YAML mapping node into a Config struct using the existing parser.
func parseNodeToConfig(root *yaml.Node) (*Config, []ValidationError, []ValidationWarning) {
	if root == nil {
		return nil, []ValidationError{
			newError(E001, "", "config: top-level document must be a YAML mapping"),
		}, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, []ValidationError{
			newError(E001, "", "config: top-level document must be a YAML mapping"),
		}, nil
	}
	return parseFromNode(root)
}

// getIncludeLineNumber returns the line number of an include entry in a YAML node tree.
// idx is the index into the include sequence.
func getIncludeLineNumber(root *yaml.Node, idx int) int {
	if root == nil || root.Kind != yaml.MappingNode {
		return 0
	}
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "include" {
			includeNode := root.Content[i+1]
			if includeNode.Kind == yaml.SequenceNode && idx < len(includeNode.Content) {
				return includeNode.Content[idx].Line
			}
			return includeNode.Line
		}
	}
	return 0
}

// getIncludePaths extracts the include paths from a raw YAML mapping node.
func getIncludePaths(root *yaml.Node) []string {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "include" {
			node := root.Content[i+1]
			if node.Tag == "!!null" || node.Kind != yaml.SequenceNode {
				return nil
			}
			var paths []string
			for _, item := range node.Content {
				if item.Kind == yaml.ScalarNode {
					paths = append(paths, item.Value)
				}
			}
			return paths
		}
	}
	return nil
}

// removeIncludeField returns a new mapping node with the "include" key removed.
func removeIncludeField(root *yaml.Node) *yaml.Node {
	if root == nil || root.Kind != yaml.MappingNode {
		return root
	}
	result := &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     root.Tag,
		Style:   root.Style,
		Content: make([]*yaml.Node, 0, len(root.Content)),
	}
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value != "include" {
			result.Content = append(result.Content, root.Content[i], root.Content[i+1])
		}
	}
	return result
}

// isHTTPURL checks if a path looks like an HTTP(S) URL.
func isHTTPURL(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
