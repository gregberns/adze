package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse parses YAML input into a Config struct, returning validation errors and warnings.
// If the YAML is syntactically invalid, a non-nil error is returned and no validation
// errors/warnings are produced. Otherwise, error is nil and all validation problems
// appear in the ValidationError/ValidationWarning slices.
func Parse(data []byte) (*Config, []ValidationError, []ValidationWarning, error) {
	// Step 1: Parse YAML into a raw node tree
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, nil, fmt.Errorf("yaml: %w", err)
	}

	// Handle empty documents (null)
	if doc.Kind == 0 {
		// Empty document -- treat as null (not a mapping)
		return nil, []ValidationError{
			newError(E001, "", "config: top-level document must be a YAML mapping"),
		}, nil, nil
	}

	// doc is the Document node; its Content[0] is the actual value
	if len(doc.Content) == 0 {
		return nil, []ValidationError{
			newError(E001, "", "config: top-level document must be a YAML mapping"),
		}, nil, nil
	}

	root := doc.Content[0]

	// Step 2: Check that the top-level is a mapping
	if root.Kind != yaml.MappingNode {
		return nil, []ValidationError{
			newError(E001, "", "config: top-level document must be a YAML mapping"),
		}, nil, nil
	}

	// Step 3: Parse from node tree, collecting errors
	cfg, errs, warns := parseFromNode(root)

	return cfg, errs, warns, nil
}

// validTopLevelFieldSet is a set for fast lookup.
var validTopLevelFieldSet = func() map[string]bool {
	m := make(map[string]bool, len(validTopLevelFields))
	for _, f := range validTopLevelFields {
		m[f] = true
	}
	return m
}()

// parseFromNode extracts a Config from a YAML mapping node, collecting all validation
// errors and warnings.
func parseFromNode(root *yaml.Node) (*Config, []ValidationError, []ValidationWarning) {
	var errs []ValidationError
	var warns []ValidationWarning

	cfg := &Config{
		Tags:        []string{},
		Include:     []string{},
		Secrets:     []SecretEntry{},
		Packages:    PackagesConfig{Brew: []PackageEntry{}, Cask: []PackageEntry{}, Apt: []PackageEntry{}},
		Defaults:    map[string]map[string]DefaultValue{},
		Dock:        DockConfig{Apps: []string{}},
		Shell:       ShellConfig{Plugins: []string{}},
		Directories: []string{},
		CustomSteps: map[string]CustomStep{},
	}

	// Track which fields we've seen, and check for unknown top-level keys
	seenFields := make(map[string]bool)
	fieldNodes := make(map[string]*yaml.Node)

	for i := 0; i < len(root.Content)-1; i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]
		key := keyNode.Value

		if !validTopLevelFieldSet[key] {
			errs = append(errs, newError(E002, key,
				fmt.Sprintf("%s: unknown field; valid top-level fields are: %s",
					key, strings.Join(validTopLevelFields, ", "))))
			continue
		}

		seenFields[key] = true
		fieldNodes[key] = valNode
	}

	// Parse each known field from its node
	if node, ok := fieldNodes["name"]; ok {
		cfg.Name = node.Value
	}
	if node, ok := fieldNodes["platform"]; ok {
		cfg.Platform = node.Value
	}
	if node, ok := fieldNodes["tags"]; ok {
		parseTags(node, cfg, &errs)
	}
	if node, ok := fieldNodes["include"]; ok {
		parseInclude(node, cfg, &errs)
	}
	if node, ok := fieldNodes["machine"]; ok {
		parseMachine(node, cfg, &errs)
	}
	if node, ok := fieldNodes["identity"]; ok {
		parseIdentity(node, cfg, &errs, &warns)
	}
	if node, ok := fieldNodes["secrets"]; ok {
		parseSecrets(node, cfg, &errs)
	}
	if node, ok := fieldNodes["packages"]; ok {
		parsePackages(node, cfg, &errs)
	}
	if node, ok := fieldNodes["defaults"]; ok {
		parseDefaults(node, cfg, &errs)
	}
	if node, ok := fieldNodes["dock"]; ok {
		parseDock(node, cfg, &errs)
	}
	if node, ok := fieldNodes["shell"]; ok {
		parseShell(node, cfg, &errs)
	}
	if node, ok := fieldNodes["directories"]; ok {
		parseDirectories(node, cfg, &errs, &warns)
	}
	if node, ok := fieldNodes["custom_steps"]; ok {
		parseCustomSteps(node, cfg, &errs)
	}

	// Now run validation on the parsed config
	ve, vw := validate(cfg, seenFields, fieldNodes)
	errs = append(errs, ve...)
	warns = append(warns, vw...)

	return cfg, errs, warns
}

func parseTags(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.Tags = []string{}
		return
	}
	if node.Kind != yaml.SequenceNode {
		*errs = append(*errs, newError(E042, "tags", "tags: expected sequence, got mapping or scalar"))
		return
	}
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag == "!!null" {
			cfg.Tags = append(cfg.Tags, "")
		} else {
			cfg.Tags = append(cfg.Tags, item.Value)
		}
	}
}

func parseInclude(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.Include = []string{}
		return
	}
	if node.Kind != yaml.SequenceNode {
		*errs = append(*errs, newError(E042, "include", "include: expected sequence, got mapping or scalar"))
		return
	}
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag == "!!null" {
			cfg.Include = append(cfg.Include, "")
		} else {
			cfg.Include = append(cfg.Include, item.Value)
		}
	}
}

func parseMachine(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		return
	}
	if node.Kind != yaml.MappingNode {
		*errs = append(*errs, newError(E042, "machine", "machine: expected mapping, got sequence or scalar"))
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "hostname":
			if val.Tag != "!!null" {
				cfg.Machine.Hostname = val.Value
			}
		default:
			*errs = append(*errs, newError(E043, "machine."+key,
				fmt.Sprintf("machine.%s: unknown field; valid fields are: hostname", key)))
		}
	}
}

func parseIdentity(node *yaml.Node, cfg *Config, errs *[]ValidationError, warns *[]ValidationWarning) {
	if node.Tag == "!!null" {
		return
	}
	if node.Kind != yaml.MappingNode {
		*errs = append(*errs, newError(E042, "identity", "identity: expected mapping, got sequence or scalar"))
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "git_name":
			if val.Tag != "!!null" {
				cfg.Identity.GitName = val.Value
			}
		case "git_email":
			if val.Tag != "!!null" {
				cfg.Identity.GitEmail = val.Value
			}
		case "github_user":
			if val.Tag != "!!null" {
				cfg.Identity.GithubUser = val.Value
			}
		default:
			*errs = append(*errs, newError(E043, "identity."+key,
				fmt.Sprintf("identity.%s: unknown field; valid fields are: git_name, git_email, github_user", key)))
		}
	}
}

func parseSecrets(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.Secrets = []SecretEntry{}
		return
	}
	if node.Kind != yaml.SequenceNode {
		*errs = append(*errs, newError(E042, "secrets", "secrets: expected sequence, got mapping or scalar"))
		return
	}
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		entry := SecretEntry{Required: true} // default required=true
		for j := 0; j < len(item.Content)-1; j += 2 {
			key := item.Content[j].Value
			val := item.Content[j+1]
			switch key {
			case "name":
				entry.Name = val.Value
			case "description":
				entry.Description = val.Value
			case "required":
				entry.Required = val.Value == "true"
			case "sensitive":
				entry.Sensitive = val.Value == "true"
			case "validate":
				entry.Validate = val.Value
			case "prompt":
				entry.Prompt = val.Value == "true"
			default:
				*errs = append(*errs, newError(E043, "secrets[]."+key,
					fmt.Sprintf("secrets[].%s: unknown field; valid fields are: name, description, required, sensitive, validate, prompt", key)))
			}
		}
		cfg.Secrets = append(cfg.Secrets, entry)
	}
}

func parsePackages(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.Packages = PackagesConfig{Brew: []PackageEntry{}, Cask: []PackageEntry{}, Apt: []PackageEntry{}}
		return
	}
	if node.Kind != yaml.MappingNode {
		*errs = append(*errs, newError(E042, "packages", "packages: expected mapping, got sequence or scalar"))
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "brew":
			cfg.Packages.Brew = parsePackageList(val, "packages.brew", errs)
		case "cask":
			cfg.Packages.Cask = parsePackageList(val, "packages.cask", errs)
		case "apt":
			cfg.Packages.Apt = parsePackageList(val, "packages.apt", errs)
		default:
			*errs = append(*errs, newError(E043, "packages."+key,
				fmt.Sprintf("packages.%s: unknown field; valid fields are: brew, cask, apt", key)))
		}
	}
}

func parsePackageList(node *yaml.Node, path string, errs *[]ValidationError) []PackageEntry {
	if node.Tag == "!!null" || node.Kind != yaml.SequenceNode {
		return []PackageEntry{}
	}
	var entries []PackageEntry
	for idx, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			// Short form: just a string name
			entries = append(entries, PackageEntry{Name: item.Value})
		case yaml.MappingNode:
			// Object form: parse name, version, pinned from node
			entry := PackageEntry{}
			for j := 0; j < len(item.Content)-1; j += 2 {
				keyNode := item.Content[j]
				valNode := item.Content[j+1]
				switch keyNode.Value {
				case "name":
					entry.Name = valNode.Value
				case "version":
					// Check for unquoted numeric: inspect the YAML tag
					if valNode.Tag == "!!float" || valNode.Tag == "!!int" {
						*errs = append(*errs, newError(E020,
							fmt.Sprintf("%s[%d].version", path, idx),
							fmt.Sprintf("%s[%d].version: version values must be quoted strings (e.g., version: \"3.11\" not version: 3.11); unquoted numeric versions lose precision", path, idx)))
						// Still store the raw value for the struct
						entry.Version = valNode.Value
					} else if valNode.Tag == "!!null" {
						entry.Version = ""
					} else {
						entry.Version = valNode.Value
					}
				case "pinned":
					entry.Pinned = valNode.Value == "true"
				default:
					*errs = append(*errs, newError(E043,
						fmt.Sprintf("%s[%d].%s", path, idx, keyNode.Value),
						fmt.Sprintf("%s[%d].%s: unknown field; valid fields are: name, version, pinned", path, idx, keyNode.Value)))
				}
			}
			entries = append(entries, entry)
		default:
			// Invalid node type in package list
		}
	}
	return entries
}

func parseDefaults(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.Defaults = map[string]map[string]DefaultValue{}
		return
	}
	if node.Kind != yaml.MappingNode {
		*errs = append(*errs, newError(E042, "defaults", "defaults: expected mapping, got sequence or scalar"))
		return
	}
	cfg.Defaults = make(map[string]map[string]DefaultValue)
	for i := 0; i < len(node.Content)-1; i += 2 {
		domainKey := node.Content[i].Value
		domainNode := node.Content[i+1]

		if domainKey == "" {
			*errs = append(*errs, newError(E023, "defaults.",
				"defaults.: domain key must not be empty"))
			continue
		}

		if domainNode.Kind != yaml.MappingNode {
			continue
		}

		prefs := make(map[string]DefaultValue)
		for j := 0; j < len(domainNode.Content)-1; j += 2 {
			prefKey := domainNode.Content[j].Value
			prefVal := domainNode.Content[j+1]

			fieldPath := fmt.Sprintf("defaults.%s.%s", domainKey, prefKey)

			if prefKey == "" {
				*errs = append(*errs, newError(E024, fieldPath,
					fmt.Sprintf("defaults.%s.: preference key must not be empty", domainKey)))
				continue
			}

			if prefVal.Tag == "!!null" {
				*errs = append(*errs, newError(E025, fieldPath,
					fmt.Sprintf("%s: null values are not permitted", fieldPath)))
				continue
			}

			dv, err := parseDefaultValue(prefVal, fieldPath)
			if err != nil {
				*errs = append(*errs, *err)
				continue
			}
			prefs[prefKey] = dv
		}
		cfg.Defaults[domainKey] = prefs
	}
}

func parseDefaultValue(node *yaml.Node, fieldPath string) (DefaultValue, *ValidationError) {
	switch node.Tag {
	case "!!bool":
		return DefaultValue{Value: node.Value == "true"}, nil
	case "!!int":
		// Parse as int
		var v int
		if err := node.Decode(&v); err != nil {
			ve := newError(E026, fieldPath,
				fmt.Sprintf("%s: unsupported value type %s; must be bool, int, float, or string", fieldPath, "int"))
			return DefaultValue{}, &ve
		}
		return DefaultValue{Value: v}, nil
	case "!!float":
		var v float64
		if err := node.Decode(&v); err != nil {
			ve := newError(E026, fieldPath,
				fmt.Sprintf("%s: unsupported value type %s; must be bool, int, float, or string", fieldPath, "float"))
			return DefaultValue{}, &ve
		}
		return DefaultValue{Value: v}, nil
	case "!!str":
		return DefaultValue{Value: node.Value}, nil
	default:
		// Try to determine what the tag is for error messaging
		tagName := node.Tag
		if tagName == "" {
			tagName = "unknown"
		}
		ve := newError(E026, fieldPath,
			fmt.Sprintf("%s: unsupported value type %s; must be bool, int, float, or string", fieldPath, tagName))
		return DefaultValue{}, &ve
	}
}

func parseDock(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.Dock = DockConfig{Apps: []string{}}
		return
	}
	if node.Kind != yaml.MappingNode {
		*errs = append(*errs, newError(E042, "dock", "dock: expected mapping, got sequence or scalar"))
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "apps":
			if val.Tag == "!!null" || val.Kind != yaml.SequenceNode {
				cfg.Dock.Apps = []string{}
				continue
			}
			for _, item := range val.Content {
				if item.Kind == yaml.ScalarNode && item.Tag != "!!null" {
					cfg.Dock.Apps = append(cfg.Dock.Apps, item.Value)
				} else {
					cfg.Dock.Apps = append(cfg.Dock.Apps, "")
				}
			}
		default:
			*errs = append(*errs, newError(E043, "dock."+key,
				fmt.Sprintf("dock.%s: unknown field; valid fields are: apps", key)))
		}
	}
}

func parseShell(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.Shell = ShellConfig{Plugins: []string{}}
		return
	}
	if node.Kind != yaml.MappingNode {
		*errs = append(*errs, newError(E042, "shell", "shell: expected mapping, got sequence or scalar"))
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "default":
			if val.Tag != "!!null" {
				cfg.Shell.Default = val.Value
			}
		case "oh_my_zsh":
			cfg.Shell.OhMyZsh = val.Value == "true"
		case "theme":
			if val.Tag != "!!null" {
				cfg.Shell.Theme = val.Value
			}
		case "plugins":
			if val.Tag == "!!null" || val.Kind != yaml.SequenceNode {
				cfg.Shell.Plugins = []string{}
				continue
			}
			for _, item := range val.Content {
				if item.Kind == yaml.ScalarNode && item.Tag != "!!null" {
					cfg.Shell.Plugins = append(cfg.Shell.Plugins, item.Value)
				} else {
					cfg.Shell.Plugins = append(cfg.Shell.Plugins, "")
				}
			}
		default:
			*errs = append(*errs, newError(E043, "shell."+key,
				fmt.Sprintf("shell.%s: unknown field; valid fields are: default, oh_my_zsh, theme, plugins", key)))
		}
	}
}

func parseDirectories(node *yaml.Node, cfg *Config, errs *[]ValidationError, warns *[]ValidationWarning) {
	if node.Tag == "!!null" {
		cfg.Directories = []string{}
		return
	}
	if node.Kind != yaml.SequenceNode {
		*errs = append(*errs, newError(E042, "directories", "directories: expected sequence, got mapping or scalar"))
		return
	}
	for _, item := range node.Content {
		if item.Kind == yaml.ScalarNode && item.Tag != "!!null" {
			cfg.Directories = append(cfg.Directories, item.Value)
		} else {
			cfg.Directories = append(cfg.Directories, "")
		}
	}
}

var stepNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func parseCustomSteps(node *yaml.Node, cfg *Config, errs *[]ValidationError) {
	if node.Tag == "!!null" {
		cfg.CustomSteps = map[string]CustomStep{}
		return
	}
	if node.Kind != yaml.MappingNode {
		*errs = append(*errs, newError(E042, "custom_steps", "custom_steps: expected mapping, got sequence or scalar"))
		return
	}
	cfg.CustomSteps = make(map[string]CustomStep)
	for i := 0; i < len(node.Content)-1; i += 2 {
		nameNode := node.Content[i]
		stepNode := node.Content[i+1]
		name := nameNode.Value

		step := CustomStep{
			Provides: []string{},
			Requires: []string{},
			Platform: []string{"any"},
			Apply:    map[string]string{},
			Rollback: map[string]string{},
			Env:      []string{},
			Tags:     []string{},
		}

		if stepNode.Kind == yaml.MappingNode {
			for j := 0; j < len(stepNode.Content)-1; j += 2 {
				key := stepNode.Content[j].Value
				val := stepNode.Content[j+1]
				switch key {
				case "description":
					if val.Tag != "!!null" {
						step.Description = val.Value
					}
				case "provides":
					step.Provides = parseStringList(val)
				case "requires":
					step.Requires = parseStringList(val)
				case "platform":
					step.Platform = parseStringList(val)
					if len(step.Platform) == 0 {
						step.Platform = []string{"any"}
					}
				case "check":
					if val.Tag != "!!null" {
						step.Check = val.Value
					}
				case "apply":
					step.Apply = parseStringMap(val)
				case "rollback":
					step.Rollback = parseStringMap(val)
				case "env":
					step.Env = parseStringList(val)
				case "tags":
					step.Tags = parseStringList(val)
				default:
					*errs = append(*errs, newError(E043,
						fmt.Sprintf("custom_steps.%s.%s", name, key),
						fmt.Sprintf("custom_steps.%s.%s: unknown field; valid fields are: description, provides, requires, platform, check, apply, rollback, env, tags", name, key)))
				}
			}
		}

		cfg.CustomSteps[name] = step
	}
}

func parseStringList(node *yaml.Node) []string {
	if node.Tag == "!!null" || node.Kind != yaml.SequenceNode {
		return []string{}
	}
	var result []string
	for _, item := range node.Content {
		if item.Kind == yaml.ScalarNode && item.Tag != "!!null" {
			result = append(result, item.Value)
		} else {
			result = append(result, "")
		}
	}
	return result
}

func parseStringMap(node *yaml.Node) map[string]string {
	if node.Tag == "!!null" || node.Kind != yaml.MappingNode {
		return map[string]string{}
	}
	m := make(map[string]string)
	for i := 0; i < len(node.Content)-1; i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		if v.Tag != "!!null" {
			m[k] = v.Value
		} else {
			m[k] = ""
		}
	}
	return m
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedStepKeys returns the keys of a CustomStep map in sorted order.
func sortedStepKeys(m map[string]CustomStep) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
