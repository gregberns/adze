package config

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	secretNameRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	envVarRe     = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// validate performs all validation on a parsed Config.
// seenFields tracks which top-level YAML keys were actually present in the document.
// fieldNodes maps top-level keys to their YAML nodes (used for "present but empty" checks).
func validate(cfg *Config, seenFields map[string]bool, fieldNodes map[string]*yaml.Node) ([]ValidationError, []ValidationWarning) {
	var errs []ValidationError
	var warns []ValidationWarning

	// Required fields
	validateRequired(cfg, seenFields, fieldNodes, &errs)

	// Name validation
	validateName(cfg, seenFields, &errs)

	// Platform validation
	validatePlatform(cfg, seenFields, &errs)

	// Tags validation
	validateTags(cfg, &errs)

	// Include validation
	validateInclude(cfg, &errs)

	// Machine validation
	validateMachine(cfg, seenFields, fieldNodes, &errs)

	// Identity validation
	validateIdentity(cfg, seenFields, fieldNodes, &errs, &warns)

	// Secrets validation
	validateSecrets(cfg, &errs)

	// Packages validation
	validatePackages(cfg, &errs)

	// Defaults validation (most is done at parse time, but we still
	// need to check for empty domain keys at the top level)
	// Already done in parseDefaults

	// Dock validation
	validateDock(cfg, &errs)

	// Shell validation
	validateShell(cfg, seenFields, fieldNodes, &errs)

	// Directories validation
	validateDirectories(cfg, &errs, &warns)

	// Custom steps validation
	validateCustomSteps(cfg, &errs)

	return errs, warns
}

func validateRequired(cfg *Config, seenFields map[string]bool, fieldNodes map[string]*yaml.Node, errs *[]ValidationError) {
	if !seenFields["name"] {
		*errs = append(*errs, newError(E003, "name", "name: required field is missing"))
	}
	if !seenFields["platform"] {
		*errs = append(*errs, newError(E006, "platform", "platform: required field is missing"))
	}
}

func validateName(cfg *Config, seenFields map[string]bool, errs *[]ValidationError) {
	if !seenFields["name"] {
		return // already reported E003
	}
	if cfg.Name == "" {
		*errs = append(*errs, newError(E004, "name", "name: must not be empty"))
		return
	}
	if len(cfg.Name) > 255 {
		*errs = append(*errs, newError(E005, "name",
			"name: must not exceed 255 characters"))
	}
}

func validatePlatform(cfg *Config, seenFields map[string]bool, errs *[]ValidationError) {
	if !seenFields["platform"] {
		return // already reported E006
	}
	if !isValidPlatform(cfg.Platform) {
		*errs = append(*errs, newError(E007, "platform",
			fmt.Sprintf("platform: invalid value %q; must be one of: darwin, ubuntu, debian, any", cfg.Platform)))
	}
}

func validateTags(cfg *Config, errs *[]ValidationError) {
	for i, tag := range cfg.Tags {
		if tag == "" || containsWhitespace(tag) {
			*errs = append(*errs, newError(E008, fmt.Sprintf("tags[%d]", i),
				fmt.Sprintf("tags[%d]: tag must be a non-empty string without whitespace", i)))
		}
	}
}

func validateInclude(cfg *Config, errs *[]ValidationError) {
	for i, path := range cfg.Include {
		if path == "" {
			*errs = append(*errs, newError(E009, fmt.Sprintf("include[%d]", i),
				fmt.Sprintf("include[%d]: path must not be empty", i)))
			continue
		}
		if isRemoteURL(path) {
			*errs = append(*errs, newError(E010, fmt.Sprintf("include[%d]", i),
				fmt.Sprintf("include[%d]: remote URLs are not supported; only local file paths are allowed", i)))
		}
	}
}

func validateMachine(cfg *Config, seenFields map[string]bool, fieldNodes map[string]*yaml.Node, errs *[]ValidationError) {
	if !seenFields["machine"] {
		return
	}
	// Check if hostname was specified in the YAML node
	node := fieldNodes["machine"]
	if node == nil || node.Tag == "!!null" || node.Kind != yaml.MappingNode {
		return
	}

	hostnamePresent := false
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == "hostname" {
			hostnamePresent = true
			break
		}
	}

	if hostnamePresent && cfg.Machine.Hostname == "" {
		// Hostname key was present but empty
		*errs = append(*errs, newError(E011, "machine.hostname",
			"machine.hostname: invalid hostname \"\"; must match RFC 1123"))
		return
	}

	if cfg.Machine.Hostname != "" && !isValidRFC1123Hostname(cfg.Machine.Hostname) {
		*errs = append(*errs, newError(E011, "machine.hostname",
			fmt.Sprintf("machine.hostname: invalid hostname %q; must match RFC 1123", cfg.Machine.Hostname)))
	}
}

func validateIdentity(cfg *Config, seenFields map[string]bool, fieldNodes map[string]*yaml.Node, errs *[]ValidationError, warns *[]ValidationWarning) {
	if !seenFields["identity"] {
		return
	}
	node := fieldNodes["identity"]
	if node == nil || node.Tag == "!!null" || node.Kind != yaml.MappingNode {
		return
	}

	// Check which identity fields were present in YAML
	identityFieldPresent := make(map[string]bool)
	for i := 0; i < len(node.Content)-1; i += 2 {
		identityFieldPresent[node.Content[i].Value] = true
	}

	if identityFieldPresent["git_name"] {
		if cfg.Identity.GitName == "" {
			*errs = append(*errs, newError(E012, "identity.git_name",
				"identity.git_name: must not be empty if present"))
		}
	}

	if identityFieldPresent["git_email"] {
		if cfg.Identity.GitEmail == "" {
			*errs = append(*errs, newError(E013, "identity.git_email",
				"identity.git_email: must not be empty if present"))
		} else if !strings.Contains(cfg.Identity.GitEmail, "@") {
			*warns = append(*warns, newWarning(W001, "identity.git_email",
				"identity.git_email: value does not look like an email address"))
		}
	}

	if identityFieldPresent["github_user"] {
		if cfg.Identity.GithubUser == "" {
			*errs = append(*errs, newError(E014, "identity.github_user",
				"identity.github_user: must not be empty if present"))
		} else if containsWhitespace(cfg.Identity.GithubUser) {
			*errs = append(*errs, newError(E015, "identity.github_user",
				"identity.github_user: must not contain whitespace"))
		}
	}
}

func validateSecrets(cfg *Config, errs *[]ValidationError) {
	seen := make(map[string]bool)
	for i, s := range cfg.Secrets {
		if s.Name == "" {
			*errs = append(*errs, newError(E016, fmt.Sprintf("secrets[%d].name", i),
				fmt.Sprintf("secrets[%d].name: required field is missing", i)))
			continue
		}
		if !secretNameRe.MatchString(s.Name) {
			*errs = append(*errs, newError(E017, fmt.Sprintf("secrets[%d].name", i),
				fmt.Sprintf("secrets[%d].name: must match pattern [A-Z][A-Z0-9_]* (e.g., GITHUB_TOKEN)", i)))
		}
		if seen[s.Name] {
			*errs = append(*errs, newError(E018, fmt.Sprintf("secrets[%d].name", i),
				fmt.Sprintf("secrets[%d].name: duplicate secret name %q; each secret must be unique", i, s.Name)))
		}
		seen[s.Name] = true
	}
}

func validatePackages(cfg *Config, errs *[]ValidationError) {
	validatePackageList(cfg.Packages.Brew, "packages.brew", errs)
	validatePackageList(cfg.Packages.Cask, "packages.cask", errs)
	validatePackageList(cfg.Packages.Apt, "packages.apt", errs)
}

func validatePackageList(entries []PackageEntry, path string, errs *[]ValidationError) {
	seen := make(map[string]bool)
	for i, e := range entries {
		if e.Name == "" || containsWhitespace(e.Name) {
			*errs = append(*errs, newError(E019, fmt.Sprintf("%s[%d].name", path, i),
				fmt.Sprintf("%s[%d].name: must be a non-empty string without whitespace", path, i)))
		}

		// E020 is checked at parse time (version tag inspection)
		// E021: version empty if present
		if e.VersionPresent && e.Version == "" {
			*errs = append(*errs, newError(E021, fmt.Sprintf("%s[%d].version", path, i),
				fmt.Sprintf("%s[%d].version: must not be empty if present", path, i)))
		}

		if e.Name != "" {
			if seen[e.Name] {
				*errs = append(*errs, newError(E022, path,
					fmt.Sprintf("%s: duplicate package name %q", path, e.Name)))
			}
			seen[e.Name] = true
		}
	}
}

func validateDock(cfg *Config, errs *[]ValidationError) {
	for i, app := range cfg.Dock.Apps {
		if app == "" {
			*errs = append(*errs, newError(E027, fmt.Sprintf("dock.apps[%d]", i),
				fmt.Sprintf("dock.apps[%d]: must be a non-empty string", i)))
		}
	}
}

func validateShell(cfg *Config, seenFields map[string]bool, fieldNodes map[string]*yaml.Node, errs *[]ValidationError) {
	if !seenFields["shell"] {
		return
	}

	// Check shell.default
	if cfg.Shell.Default != "" && !isValidShell(cfg.Shell.Default) {
		*errs = append(*errs, newError(E028, "shell.default",
			fmt.Sprintf("shell.default: invalid value %q; must be one of: zsh, bash, fish (or omit to leave shell unchanged)", cfg.Shell.Default)))
	}

	// Check shell.theme: must not be empty if present
	node := fieldNodes["shell"]
	if node != nil && node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content)-1; i += 2 {
			if node.Content[i].Value == "theme" {
				if cfg.Shell.Theme == "" {
					*errs = append(*errs, newError(E029, "shell.theme",
						"shell.theme: must not be empty if present"))
				}
				break
			}
		}
	}

	// Check shell.plugins
	for i, p := range cfg.Shell.Plugins {
		if p == "" || containsWhitespace(p) {
			*errs = append(*errs, newError(E030, fmt.Sprintf("shell.plugins[%d]", i),
				fmt.Sprintf("shell.plugins[%d]: must be a non-empty string without whitespace", i)))
		}
	}
}

func validateDirectories(cfg *Config, errs *[]ValidationError, warns *[]ValidationWarning) {
	seen := make(map[string]bool)
	for i, d := range cfg.Directories {
		if d == "" {
			*errs = append(*errs, newError(E031, fmt.Sprintf("directories[%d]", i),
				fmt.Sprintf("directories[%d]: must be a non-empty string", i)))
			continue
		}
		if seen[d] {
			*warns = append(*warns, newWarning(W002, "directories",
				fmt.Sprintf("directories: duplicate entry %q", d)))
		}
		seen[d] = true
	}
}

func validateCustomSteps(cfg *Config, errs *[]ValidationError) {
	for _, name := range sortedStepKeys(cfg.CustomSteps) {
		step := cfg.CustomSteps[name]

		// E032: step name pattern
		if !stepNameRe.MatchString(name) {
			*errs = append(*errs, newError(E032, fmt.Sprintf("custom_steps.%s", name),
				fmt.Sprintf("custom_steps.%s: step name must match pattern [a-z][a-z0-9-]* (e.g., my-go-tool)", name)))
		}

		// E033: provides elements
		for i, p := range step.Provides {
			if p == "" || containsWhitespace(p) {
				*errs = append(*errs, newError(E033, fmt.Sprintf("custom_steps.%s.provides[%d]", name, i),
					fmt.Sprintf("custom_steps.%s.provides[%d]: must be a non-empty string without whitespace", name, i)))
			}
		}

		// SEM-23: provides entries must be unique within the list
		seenProvides := make(map[string]bool)
		for i, p := range step.Provides {
			if p == "" {
				continue
			}
			if seenProvides[p] {
				*errs = append(*errs, newError(E033, fmt.Sprintf("custom_steps.%s.provides[%d]", name, i),
					fmt.Sprintf("custom_steps.%s.provides: duplicate entry %q", name, p)))
			}
			seenProvides[p] = true
		}

		// E034: requires elements
		for i, r := range step.Requires {
			if r == "" || containsWhitespace(r) {
				*errs = append(*errs, newError(E034, fmt.Sprintf("custom_steps.%s.requires[%d]", name, i),
					fmt.Sprintf("custom_steps.%s.requires[%d]: must be a non-empty string without whitespace", name, i)))
			}
		}

		// E035: platform elements
		for i, p := range step.Platform {
			if !isValidPlatform(p) {
				*errs = append(*errs, newError(E035, fmt.Sprintf("custom_steps.%s.platform[%d]", name, i),
					fmt.Sprintf("custom_steps.%s.platform[%d]: invalid value %q; must be one of: darwin, ubuntu, debian, any", name, i, p)))
			}
		}

		// E036, E037: apply keys and values
		for _, k := range sortedKeys(step.Apply) {
			v := step.Apply[k]
			if !isValidPlatform(k) {
				*errs = append(*errs, newError(E036, fmt.Sprintf("custom_steps.%s.apply.%s", name, k),
					fmt.Sprintf("custom_steps.%s.apply.%s: invalid platform key %q; must be one of: darwin, ubuntu, debian, any", name, k, k)))
			}
			if v == "" {
				*errs = append(*errs, newError(E037, fmt.Sprintf("custom_steps.%s.apply.%s", name, k),
					fmt.Sprintf("custom_steps.%s.apply.%s: command must not be empty", name, k)))
			}
		}

		// E038, E039: rollback keys and values
		for _, k := range sortedKeys(step.Rollback) {
			v := step.Rollback[k]
			if !isValidPlatform(k) {
				*errs = append(*errs, newError(E038, fmt.Sprintf("custom_steps.%s.rollback.%s", name, k),
					fmt.Sprintf("custom_steps.%s.rollback.%s: invalid platform key %q; must be one of: darwin, ubuntu, debian, any", name, k, k)))
			}
			if v == "" {
				*errs = append(*errs, newError(E039, fmt.Sprintf("custom_steps.%s.rollback.%s", name, k),
					fmt.Sprintf("custom_steps.%s.rollback.%s: command must not be empty", name, k)))
			}
		}

		// E040: env elements
		for i, e := range step.Env {
			if !envVarRe.MatchString(e) {
				*errs = append(*errs, newError(E040, fmt.Sprintf("custom_steps.%s.env[%d]", name, i),
					fmt.Sprintf("custom_steps.%s.env[%d]: must match pattern [A-Z][A-Z0-9_]* (e.g., GITHUB_TOKEN)", name, i)))
			}
		}

		// E041: tags elements
		for i, t := range step.Tags {
			if t == "" || containsWhitespace(t) {
				*errs = append(*errs, newError(E041, fmt.Sprintf("custom_steps.%s.tags[%d]", name, i),
					fmt.Sprintf("custom_steps.%s.tags[%d]: must be a non-empty string without whitespace", name, i)))
			}
		}
	}
}

// --- Helper functions ---

func isValidPlatform(s string) bool {
	for _, p := range validPlatforms {
		if s == p {
			return true
		}
	}
	return false
}

func isValidShell(s string) bool {
	for _, sh := range validShells {
		if s == sh {
			return true
		}
	}
	return false
}

func containsWhitespace(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return true
		}
	}
	return false
}

func isRemoteURL(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "ftp://") ||
		strings.HasPrefix(lower, "ssh://") ||
		strings.HasPrefix(lower, "git://")
}

// isValidRFC1123Hostname validates a hostname per RFC 1123.
// Labels are alphanumeric and hyphens only, max 63 chars per label,
// max 253 chars total, no leading/trailing hyphens per label.
func isValidRFC1123Hostname(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 253 {
		return false
	}
	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	return true
}
