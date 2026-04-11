package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func parseTestYAML(t *testing.T, input string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

func nodeToYAML(t *testing.T, node *yaml.Node) string {
	t.Helper()
	if node == nil {
		return "null\n"
	}
	data, err := yaml.Marshal(node)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}
	return string(data)
}

// --- Scalar merge tests ---

func TestMerge_ScalarOverride(t *testing.T) {
	base := parseTestYAML(t, `
name: base-name
platform: darwin
`)
	override := parseTestYAML(t, `
name: override-name
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// Parse back to check values
	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	if cfg["name"] != "override-name" {
		t.Errorf("name = %v, want override-name", cfg["name"])
	}
	// platform from base should be preserved
	if cfg["platform"] != "darwin" {
		t.Errorf("platform = %v, want darwin", cfg["platform"])
	}
}

// --- Map merge tests ---

func TestMerge_MapDeepMerge(t *testing.T) {
	base := parseTestYAML(t, `
identity:
  git_name: "Base User"
  git_email: "base@example.com"
machine:
  hostname: base-host
`)
	override := parseTestYAML(t, `
identity:
  git_email: "override@example.com"
  github_user: overrideuser
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	identity := cfg["identity"].(map[string]interface{})
	if identity["git_name"] != "Base User" {
		t.Errorf("identity.git_name = %v, want Base User", identity["git_name"])
	}
	if identity["git_email"] != "override@example.com" {
		t.Errorf("identity.git_email = %v, want override@example.com", identity["git_email"])
	}
	if identity["github_user"] != "overrideuser" {
		t.Errorf("identity.github_user = %v, want overrideuser", identity["github_user"])
	}

	// machine from base should be preserved
	machine := cfg["machine"].(map[string]interface{})
	if machine["hostname"] != "base-host" {
		t.Errorf("machine.hostname = %v, want base-host", machine["hostname"])
	}
}

// --- List merge tests ---

func TestMerge_ListStringDedup(t *testing.T) {
	base := parseTestYAML(t, `
tags:
  - dev
  - personal
`)
	override := parseTestYAML(t, `
tags:
  - personal
  - work
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	tags := cfg["tags"].([]interface{})
	// Should be: dev, personal, work (dedup, first occurrence kept)
	if len(tags) != 3 {
		t.Fatalf("tags length = %d, want 3; tags = %v", len(tags), tags)
	}
	if tags[0] != "dev" || tags[1] != "personal" || tags[2] != "work" {
		t.Errorf("tags = %v, want [dev personal work]", tags)
	}
}

func TestMerge_ListObjectDedupByName(t *testing.T) {
	base := parseTestYAML(t, `
packages:
  - name: git
  - name: python
    version: "3.10"
`)
	override := parseTestYAML(t, `
packages:
  - name: python
    version: "3.11"
  - name: jq
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	packages := cfg["packages"].([]interface{})
	// git from base, python from override (replaced), jq from override
	if len(packages) != 3 {
		t.Fatalf("packages length = %d, want 3; packages = %v", len(packages), packages)
	}

	p0 := packages[0].(map[string]interface{})
	if p0["name"] != "git" {
		t.Errorf("packages[0].name = %v, want git", p0["name"])
	}

	p1 := packages[1].(map[string]interface{})
	if p1["name"] != "python" || p1["version"] != "3.11" {
		t.Errorf("packages[1] = %v, want {name: python, version: 3.11}", p1)
	}

	p2 := packages[2].(map[string]interface{})
	if p2["name"] != "jq" {
		t.Errorf("packages[2].name = %v, want jq", p2["name"])
	}
}

func TestMerge_ListMixedStringAndObject(t *testing.T) {
	base := parseTestYAML(t, `
packages:
  - git
  - name: python
    version: "3.10"
`)
	override := parseTestYAML(t, `
packages:
  - name: git
    version: "2.40"
  - jq
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	packages := cfg["packages"].([]interface{})
	// "git" string from base gets replaced by {name: git, version: 2.40} from override
	// python stays from base
	// jq from override (as object with name)
	if len(packages) != 3 {
		t.Fatalf("packages length = %d, want 3; packages = %v", len(packages), packages)
	}

	// First item should be the override's git object (replaces base string)
	p0 := packages[0].(map[string]interface{})
	if p0["name"] != "git" || p0["version"] != "2.40" {
		t.Errorf("packages[0] = %v, want {name: git, version: 2.40}", p0)
	}

	// Second item should be python from base
	p1 := packages[1].(map[string]interface{})
	if p1["name"] != "python" {
		t.Errorf("packages[1].name = %v, want python", p1["name"])
	}

	// Third item should be jq from override (normalized to object)
	p2 := packages[2].(map[string]interface{})
	if p2["name"] != "jq" {
		t.Errorf("packages[2].name = %v, want jq", p2["name"])
	}
}

func TestMerge_ListItemsWithoutName(t *testing.T) {
	base := parseTestYAML(t, `
items:
  - description: "item one"
  - description: "item two"
`)
	override := parseTestYAML(t, `
items:
  - description: "item three"
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	items := cfg["items"].([]interface{})
	// No name field, so concatenate without dedup
	if len(items) != 3 {
		t.Fatalf("items length = %d, want 3; items = %v", len(items), items)
	}
}

// --- Null override tests ---

func TestMerge_NullOverrideRemovesKey(t *testing.T) {
	base := parseTestYAML(t, `
name: base-name
platform: darwin
machine:
  hostname: base-host
`)
	override := parseTestYAML(t, `
machine: null
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	if _, ok := cfg["machine"]; ok {
		t.Error("machine should have been removed by null override")
	}
	if cfg["name"] != "base-name" {
		t.Errorf("name = %v, want base-name", cfg["name"])
	}
}

// --- Type mismatch tests ---

func TestMerge_TypeMismatchWarning(t *testing.T) {
	base := parseTestYAML(t, `
tags:
  - dev
  - personal
`)
	override := parseTestYAML(t, `
tags: "just-a-string"
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}

	w := warnings[0]
	if w.FieldPath != "tags" {
		t.Errorf("warning field path = %q, want tags", w.FieldPath)
	}
	if w.BaseType != "sequence" {
		t.Errorf("warning base type = %q, want sequence", w.BaseType)
	}
	if w.OverrideType != "scalar" {
		t.Errorf("warning override type = %q, want scalar", w.OverrideType)
	}

	// Override should win
	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	if cfg["tags"] != "just-a-string" {
		t.Errorf("tags = %v, want just-a-string", cfg["tags"])
	}
}

// --- Edge case tests ---

func TestMerge_NilBase(t *testing.T) {
	override := parseTestYAML(t, `
name: test
`)
	var warnings []MergeWarning
	result := mergeNodes(nil, override, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	if cfg["name"] != "test" {
		t.Errorf("name = %v, want test", cfg["name"])
	}
}

func TestMerge_NilOverride(t *testing.T) {
	base := parseTestYAML(t, `
name: test
`)
	var warnings []MergeWarning
	result := mergeNodes(base, nil, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	if cfg["name"] != "test" {
		t.Errorf("name = %v, want test", cfg["name"])
	}
}

func TestMerge_EmptyLists(t *testing.T) {
	base := parseTestYAML(t, `
tags: []
`)
	override := parseTestYAML(t, `
tags:
  - dev
`)
	var warnings []MergeWarning
	result := mergeNodes(base, override, "base.yaml", "override.yaml", "", &warnings)

	var cfg map[string]interface{}
	data, _ := yaml.Marshal(result)
	yaml.Unmarshal(data, &cfg)

	tags := cfg["tags"].([]interface{})
	if len(tags) != 1 || tags[0] != "dev" {
		t.Errorf("tags = %v, want [dev]", tags)
	}
}

func TestMerge_CloneNodeDeepCopy(t *testing.T) {
	base := parseTestYAML(t, `
identity:
  git_name: original
`)

	cloned := cloneNode(base)

	// Modify the clone
	for i := 0; i < len(cloned.Content)-1; i += 2 {
		if cloned.Content[i].Value == "identity" {
			for j := 0; j < len(cloned.Content[i+1].Content)-1; j += 2 {
				if cloned.Content[i+1].Content[j].Value == "git_name" {
					cloned.Content[i+1].Content[j+1].Value = "modified"
				}
			}
		}
	}

	// Original should be unchanged
	for i := 0; i < len(base.Content)-1; i += 2 {
		if base.Content[i].Value == "identity" {
			for j := 0; j < len(base.Content[i+1].Content)-1; j += 2 {
				if base.Content[i+1].Content[j].Value == "git_name" {
					if base.Content[i+1].Content[j+1].Value != "original" {
						t.Error("clone modified the original node")
					}
				}
			}
		}
	}
}

func TestMerge_MergeWarningString(t *testing.T) {
	w := MergeWarning{
		FieldPath:    "tags",
		BaseFile:     "base.yaml",
		BaseType:     "sequence",
		OverrideFile: "override.yaml",
		OverrideType: "scalar",
	}
	s := w.String()
	if s == "" {
		t.Error("expected non-empty warning string")
	}
	expected := `Warning: type mismatch at "tags"`
	if !containsSubstring(s, expected) {
		t.Errorf("warning string does not contain %q: %s", expected, s)
	}
}

func TestMerge_RemoveIncludeField(t *testing.T) {
	root := parseTestYAML(t, `
name: test
include:
  - shared.yaml
platform: darwin
`)
	result := removeIncludeField(root)
	yaml, _ := yaml.Marshal(result)
	s := string(yaml)
	if containsSubstring(s, "include") {
		t.Errorf("include field should have been removed: %s", s)
	}
	if !containsSubstring(s, "name") || !containsSubstring(s, "platform") {
		t.Errorf("name and platform should be preserved: %s", s)
	}
}

func TestMerge_GetIncludePaths(t *testing.T) {
	root := parseTestYAML(t, `
name: test
include:
  - shared/identity.yaml
  - shared/shell.yaml
`)
	paths := getIncludePaths(root)
	if len(paths) != 2 {
		t.Fatalf("expected 2 include paths, got %d", len(paths))
	}
	if paths[0] != "shared/identity.yaml" || paths[1] != "shared/shell.yaml" {
		t.Errorf("paths = %v", paths)
	}
}

func TestMerge_GetIncludePathsNone(t *testing.T) {
	root := parseTestYAML(t, `
name: test
platform: darwin
`)
	paths := getIncludePaths(root)
	if len(paths) != 0 {
		t.Errorf("expected 0 include paths, got %d: %v", len(paths), paths)
	}
}

func TestMerge_GetIncludePathsNull(t *testing.T) {
	root := parseTestYAML(t, `
name: test
include: null
`)
	paths := getIncludePaths(root)
	if len(paths) != 0 {
		t.Errorf("expected 0 include paths, got %d: %v", len(paths), paths)
	}
}

func TestMerge_IsHTTPURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"http://example.com/config.yaml", true},
		{"https://example.com/config.yaml", true},
		{"HTTP://EXAMPLE.COM/config.yaml", true},
		{"shared/config.yaml", false},
		{"/absolute/path.yaml", false},
		{"ftp://example.com/config.yaml", false},
	}
	for _, tt := range tests {
		got := isHTTPURL(tt.input)
		if got != tt.want {
			t.Errorf("isHTTPURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestMerge_YamlNodeTypeName(t *testing.T) {
	tests := []struct {
		kind yaml.Kind
		want string
	}{
		{yaml.ScalarNode, "scalar"},
		{yaml.SequenceNode, "sequence"},
		{yaml.MappingNode, "mapping"},
		{yaml.AliasNode, "alias"},
		{yaml.DocumentNode, "document"},
	}
	for _, tt := range tests {
		node := &yaml.Node{Kind: tt.kind}
		got := yamlNodeTypeName(node)
		if got != tt.want {
			t.Errorf("yamlNodeTypeName(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
