package config

import (
	"strings"
	"testing"
)

// fullConfig is the complete example from the spec.
const fullConfig = `
name: "Greg's MacBook Pro"
platform: darwin
tags: [dev, personal]

include:
  - shared/git.yaml
  - shared/shell.yaml

machine:
  hostname: greg-mbp

identity:
  git_name: "Greg Berns"
  git_email: "greg@example.com"
  github_user: gregberns

secrets:
  - name: GITHUB_TOKEN
    description: "GitHub personal access token for private repos"
    required: true
    sensitive: true
    validate: "gh auth status"
    prompt: false
  - name: NPM_TOKEN
    description: "npm registry auth token"
    required: false
    sensitive: true
    prompt: true
  - name: GOPRIVATE
    description: "Go private module path pattern"
    required: false
    sensitive: false

packages:
  brew:
    - git
    - jq
    - ripgrep
    - fzf
    - bat
    - fd
    - neovim
    - tmux
    - fnm
    - go
    - name: python
      version: "3.11"
    - name: node
      version: "20"
    - name: terraform
      version: "1.7.5"
      pinned: true
  cask:
    - iterm2
    - vscodium
    - google-chrome
    - slack
    - flux
    - diffmerge
  apt: []

defaults:
  NSGlobalDomain:
    AppleShowAllExtensions: true
    NSDocumentSaveNewDocumentsToCloud: false
    NSAutomaticQuoteSubstitutionEnabled: false
    AppleKeyboardUIMode: 3
  com.apple.dock:
    autohide: true
    autohide-delay: 0
    tilesize: 36
    mru-spaces: false
  com.apple.finder:
    FXPreferredViewStyle: Clmv
  com.apple.screencapture:
    location: "~/Screenshots"
    type: png

dock:
  apps:
    - /Applications/Google Chrome.app
    - /Applications/VSCodium.app
    - /Applications/iTerm.app

shell:
  default: zsh
  oh_my_zsh: true
  theme: brad-muse
  plugins:
    - zsh-syntax-highlighting

directories:
  - ~/github
  - ~/gitlab
  - ~/screenshots

custom_steps:
  my-go-tool:
    description: "Internal Go tool for project scaffolding"
    provides: [my-go-tool]
    requires: [go]
    platform: [darwin, ubuntu]
    check: "command -v my-go-tool"
    apply:
      darwin: "go install git.internal.com/tools/my-go-tool@latest"
      ubuntu: "go install git.internal.com/tools/my-go-tool@latest"
    rollback:
      darwin: "rm -f $(go env GOPATH)/bin/my-go-tool"
      ubuntu: "rm -f $(go env GOPATH)/bin/my-go-tool"
    env: [GOPRIVATE]
    tags: [dev]
  configure-ssh-agent:
    description: "Configure SSH agent to load keys at login"
    provides: [ssh-agent-configured]
    requires: []
    platform: [darwin]
    check: "grep -q 'AddKeysToAgent' ~/.ssh/config 2>/dev/null"
    apply:
      darwin: "mkdir -p ~/.ssh && printf 'Host *\n  AddKeysToAgent yes\n  UseKeychain yes\n' >> ~/.ssh/config"
    env: []
    tags: []
`

func TestParseFullConfig(t *testing.T) {
	cfg, errs, warns, err := Parse([]byte(fullConfig))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("unexpected validation error: %v", e)
		}
		t.FailNow()
	}
	if len(warns) > 0 {
		for _, w := range warns {
			t.Errorf("unexpected warning: %v", w)
		}
	}

	// Verify top-level fields
	if cfg.Name != "Greg's MacBook Pro" {
		t.Errorf("name = %q, want %q", cfg.Name, "Greg's MacBook Pro")
	}
	if cfg.Platform != "darwin" {
		t.Errorf("platform = %q, want %q", cfg.Platform, "darwin")
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "dev" || cfg.Tags[1] != "personal" {
		t.Errorf("tags = %v, want [dev personal]", cfg.Tags)
	}
	if len(cfg.Include) != 2 {
		t.Errorf("include length = %d, want 2", len(cfg.Include))
	}

	// Machine
	if cfg.Machine.Hostname != "greg-mbp" {
		t.Errorf("machine.hostname = %q, want %q", cfg.Machine.Hostname, "greg-mbp")
	}

	// Identity
	if cfg.Identity.GitName != "Greg Berns" {
		t.Errorf("identity.git_name = %q, want %q", cfg.Identity.GitName, "Greg Berns")
	}
	if cfg.Identity.GitEmail != "greg@example.com" {
		t.Errorf("identity.git_email = %q, want %q", cfg.Identity.GitEmail, "greg@example.com")
	}
	if cfg.Identity.GithubUser != "gregberns" {
		t.Errorf("identity.github_user = %q, want %q", cfg.Identity.GithubUser, "gregberns")
	}

	// Secrets
	if len(cfg.Secrets) != 3 {
		t.Fatalf("secrets length = %d, want 3", len(cfg.Secrets))
	}
	if cfg.Secrets[0].Name != "GITHUB_TOKEN" {
		t.Errorf("secrets[0].name = %q, want GITHUB_TOKEN", cfg.Secrets[0].Name)
	}
	if !cfg.Secrets[0].Required {
		t.Error("secrets[0].required = false, want true")
	}
	if !cfg.Secrets[0].Sensitive {
		t.Error("secrets[0].sensitive = false, want true")
	}
	if cfg.Secrets[1].Required {
		t.Error("secrets[1].required = true, want false")
	}

	// Packages
	if len(cfg.Packages.Brew) != 13 {
		t.Errorf("packages.brew length = %d, want 13", len(cfg.Packages.Brew))
	}
	// Check short form
	if cfg.Packages.Brew[0].Name != "git" {
		t.Errorf("packages.brew[0].name = %q, want git", cfg.Packages.Brew[0].Name)
	}
	if cfg.Packages.Brew[0].Version != "" {
		t.Errorf("packages.brew[0].version = %q, want empty", cfg.Packages.Brew[0].Version)
	}
	if cfg.Packages.Brew[0].Pinned {
		t.Error("packages.brew[0].pinned = true, want false")
	}
	// Check object form
	python := cfg.Packages.Brew[10] // python
	if python.Name != "python" || python.Version != "3.11" {
		t.Errorf("packages.brew[10] = {%q, %q}, want {python, 3.11}", python.Name, python.Version)
	}
	terraform := cfg.Packages.Brew[12] // terraform
	if terraform.Name != "terraform" || terraform.Version != "1.7.5" || !terraform.Pinned {
		t.Errorf("packages.brew[12] = {%q, %q, %v}, want {terraform, 1.7.5, true}",
			terraform.Name, terraform.Version, terraform.Pinned)
	}

	if len(cfg.Packages.Cask) != 6 {
		t.Errorf("packages.cask length = %d, want 6", len(cfg.Packages.Cask))
	}
	if len(cfg.Packages.Apt) != 0 {
		t.Errorf("packages.apt length = %d, want 0", len(cfg.Packages.Apt))
	}

	// Defaults
	if len(cfg.Defaults) != 4 {
		t.Errorf("defaults domains = %d, want 4", len(cfg.Defaults))
	}
	nsg := cfg.Defaults["NSGlobalDomain"]
	if v, ok := nsg["AppleShowAllExtensions"]; !ok || v.Value != true {
		t.Errorf("defaults.NSGlobalDomain.AppleShowAllExtensions = %v", v.Value)
	}
	if v, ok := nsg["AppleKeyboardUIMode"]; !ok || v.Value != 3 {
		t.Errorf("defaults.NSGlobalDomain.AppleKeyboardUIMode = %v", v.Value)
	}
	dock := cfg.Defaults["com.apple.dock"]
	if v, ok := dock["tilesize"]; !ok || v.Value != 36 {
		t.Errorf("defaults.com.apple.dock.tilesize = %v", v.Value)
	}
	finder := cfg.Defaults["com.apple.finder"]
	if v, ok := finder["FXPreferredViewStyle"]; !ok || v.Value != "Clmv" {
		t.Errorf("defaults.com.apple.finder.FXPreferredViewStyle = %v", v.Value)
	}
	screencap := cfg.Defaults["com.apple.screencapture"]
	if v, ok := screencap["location"]; !ok || v.Value != "~/Screenshots" {
		t.Errorf("defaults.com.apple.screencapture.location = %v", v.Value)
	}

	// Dock
	if len(cfg.Dock.Apps) != 3 {
		t.Errorf("dock.apps length = %d, want 3", len(cfg.Dock.Apps))
	}

	// Shell
	if cfg.Shell.Default != "zsh" {
		t.Errorf("shell.default = %q, want zsh", cfg.Shell.Default)
	}
	if !cfg.Shell.OhMyZsh {
		t.Error("shell.oh_my_zsh = false, want true")
	}
	if cfg.Shell.Theme != "brad-muse" {
		t.Errorf("shell.theme = %q, want brad-muse", cfg.Shell.Theme)
	}
	if len(cfg.Shell.Plugins) != 1 || cfg.Shell.Plugins[0] != "zsh-syntax-highlighting" {
		t.Errorf("shell.plugins = %v, want [zsh-syntax-highlighting]", cfg.Shell.Plugins)
	}

	// Directories
	if len(cfg.Directories) != 3 {
		t.Errorf("directories length = %d, want 3", len(cfg.Directories))
	}

	// Custom steps
	if len(cfg.CustomSteps) != 2 {
		t.Errorf("custom_steps length = %d, want 2", len(cfg.CustomSteps))
	}
	goTool, ok := cfg.CustomSteps["my-go-tool"]
	if !ok {
		t.Fatal("missing custom_steps.my-go-tool")
	}
	if goTool.Description != "Internal Go tool for project scaffolding" {
		t.Errorf("custom_steps.my-go-tool.description = %q", goTool.Description)
	}
	if len(goTool.Provides) != 1 || goTool.Provides[0] != "my-go-tool" {
		t.Errorf("custom_steps.my-go-tool.provides = %v", goTool.Provides)
	}
	if len(goTool.Platform) != 2 {
		t.Errorf("custom_steps.my-go-tool.platform = %v", goTool.Platform)
	}
	if len(goTool.Apply) != 2 {
		t.Errorf("custom_steps.my-go-tool.apply length = %d", len(goTool.Apply))
	}
	if len(goTool.Env) != 1 || goTool.Env[0] != "GOPRIVATE" {
		t.Errorf("custom_steps.my-go-tool.env = %v", goTool.Env)
	}
}

func TestParseMinimalConfig(t *testing.T) {
	input := `
name: minimal
platform: any
`
	cfg, errs, warns, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("unexpected validation error: %v", e)
		}
	}
	if len(warns) > 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	if cfg.Name != "minimal" {
		t.Errorf("name = %q, want minimal", cfg.Name)
	}
	if cfg.Platform != "any" {
		t.Errorf("platform = %q, want any", cfg.Platform)
	}

	// Check defaults for optional fields
	if len(cfg.Tags) != 0 {
		t.Errorf("tags = %v, want empty", cfg.Tags)
	}
	if len(cfg.Include) != 0 {
		t.Errorf("include = %v, want empty", cfg.Include)
	}
	if len(cfg.Secrets) != 0 {
		t.Errorf("secrets = %v, want empty", cfg.Secrets)
	}
	if len(cfg.Packages.Brew) != 0 {
		t.Errorf("packages.brew = %v, want empty", cfg.Packages.Brew)
	}
	if len(cfg.Defaults) != 0 {
		t.Errorf("defaults = %v, want empty", cfg.Defaults)
	}
	if len(cfg.Dock.Apps) != 0 {
		t.Errorf("dock.apps = %v, want empty", cfg.Dock.Apps)
	}
	if len(cfg.Shell.Plugins) != 0 {
		t.Errorf("shell.plugins = %v, want empty", cfg.Shell.Plugins)
	}
	if len(cfg.Directories) != 0 {
		t.Errorf("directories = %v, want empty", cfg.Directories)
	}
	if len(cfg.CustomSteps) != 0 {
		t.Errorf("custom_steps = %v, want empty", cfg.CustomSteps)
	}
}

func TestParseShortFormPackages(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - git
    - jq
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(cfg.Packages.Brew) != 2 {
		t.Fatalf("brew length = %d, want 2", len(cfg.Packages.Brew))
	}
	if cfg.Packages.Brew[0].Name != "git" || cfg.Packages.Brew[0].Version != "" || cfg.Packages.Brew[0].Pinned {
		t.Errorf("brew[0] = %+v, want {Name:git Version:'' Pinned:false}", cfg.Packages.Brew[0])
	}
}

func TestParseObjectFormPackages(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: terraform
      version: "1.7.5"
      pinned: true
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(cfg.Packages.Brew) != 1 {
		t.Fatalf("brew length = %d, want 1", len(cfg.Packages.Brew))
	}
	p := cfg.Packages.Brew[0]
	if p.Name != "terraform" || p.Version != "1.7.5" || !p.Pinned {
		t.Errorf("brew[0] = %+v, want {Name:terraform Version:1.7.5 Pinned:true}", p)
	}
}

func TestParseMixedPackages(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - git
    - name: python
      version: "3.11"
    - jq
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(cfg.Packages.Brew) != 3 {
		t.Fatalf("brew length = %d, want 3", len(cfg.Packages.Brew))
	}
	if cfg.Packages.Brew[0].Name != "git" {
		t.Errorf("brew[0].name = %q, want git", cfg.Packages.Brew[0].Name)
	}
	if cfg.Packages.Brew[1].Name != "python" || cfg.Packages.Brew[1].Version != "3.11" {
		t.Errorf("brew[1] = %+v", cfg.Packages.Brew[1])
	}
	if cfg.Packages.Brew[2].Name != "jq" {
		t.Errorf("brew[2].name = %q, want jq", cfg.Packages.Brew[2].Name)
	}
}

func TestParseYAMLSyntaxError(t *testing.T) {
	input := `
name: test
platform: darwin
  bad_indent: foo
`
	_, _, _, err := Parse([]byte(input))
	if err == nil {
		t.Fatal("expected YAML syntax error, got nil")
	}
}

func TestParseEmptyDocument(t *testing.T) {
	// Empty YAML
	_, errs, _, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorCode(errs, E001) {
		t.Error("expected E001 for empty document")
	}
}

func TestParseScalarDocument(t *testing.T) {
	_, errs, _, err := Parse([]byte("just a string"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorCode(errs, E001) {
		t.Error("expected E001 for scalar document")
	}
}

func TestParseSequenceDocument(t *testing.T) {
	_, errs, _, err := Parse([]byte("- item1\n- item2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasErrorCode(errs, E001) {
		t.Error("expected E001 for sequence document")
	}
}

func TestVersionTagDetection(t *testing.T) {
	// Unquoted float
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: python
      version: 3.11
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !hasErrorCode(errs, E020) {
		t.Error("expected E020 for unquoted float version")
	}

	// Unquoted integer
	input2 := `
name: test
platform: darwin
packages:
  brew:
    - name: node
      version: 20
`
	_, errs2, _, err2 := Parse([]byte(input2))
	if err2 != nil {
		t.Fatalf("unexpected parse error: %v", err2)
	}
	if !hasErrorCode(errs2, E020) {
		t.Error("expected E020 for unquoted integer version")
	}

	// Quoted version should be fine
	input3 := `
name: test
platform: darwin
packages:
  brew:
    - name: python
      version: "3.11"
`
	_, errs3, _, err3 := Parse([]byte(input3))
	if err3 != nil {
		t.Fatalf("unexpected parse error: %v", err3)
	}
	if hasErrorCode(errs3, E020) {
		t.Error("did not expect E020 for quoted version")
	}
}

func TestValidateAllBehavior(t *testing.T) {
	// Config with multiple errors: should collect all of them
	input := `
platform: invalid-platform
tags: [""]
secrets:
  - name: bad_name
  - name: GITHUB_TOKEN
  - name: GITHUB_TOKEN
shell:
  default: powershell
  theme: ""
directories:
  - ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Should have: E003 (name missing), E007 (bad platform), E008 (empty tag),
	// E017 (bad secret name), E018 (duplicate secret), E028 (bad shell),
	// E029 (empty theme), E031 (empty directory)
	codes := errorCodes(errs)

	expectCodes := []ErrorCode{E003, E007, E008, E017, E018, E028, E029, E031}
	for _, expect := range expectCodes {
		if !containsCode(codes, expect) {
			t.Errorf("expected error code %s in errors: %v", expect, codes)
		}
	}

	// Verify we got multiple errors (validate-all pattern)
	if len(errs) < 5 {
		t.Errorf("expected at least 5 errors (validate-all), got %d", len(errs))
	}
}

func TestDefaultValueTypes(t *testing.T) {
	input := `
name: test
platform: darwin
defaults:
  com.test:
    bool_val: true
    int_val: 42
    float_val: 3.14
    str_val: "hello"
    unquoted_str: Clmv
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	d := cfg.Defaults["com.test"]
	if v, ok := d["bool_val"]; !ok || v.Value != true {
		t.Errorf("bool_val = %v", v.Value)
	}
	if v, ok := d["int_val"]; !ok || v.Value != 42 {
		t.Errorf("int_val = %v", v.Value)
	}
	if v, ok := d["float_val"]; !ok || v.Value != 3.14 {
		t.Errorf("float_val = %v", v.Value)
	}
	if v, ok := d["str_val"]; !ok || v.Value != "hello" {
		t.Errorf("str_val = %v", v.Value)
	}
	if v, ok := d["unquoted_str"]; !ok || v.Value != "Clmv" {
		t.Errorf("unquoted_str = %v", v.Value)
	}
}

func TestDefaultsNullValue(t *testing.T) {
	input := `
name: test
platform: darwin
defaults:
  com.test:
    key: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !hasErrorCode(errs, E025) {
		t.Error("expected E025 for null default value")
	}
}

func TestRFC1123Hostname(t *testing.T) {
	tests := []struct {
		hostname string
		valid    bool
	}{
		{"greg-mbp", true},
		{"my.host.name", true},
		{"a", true},
		{"a-b-c", true},
		{"123abc", true},
		{"-invalid", false},
		{"invalid-", false},
		{"in valid", false},
		{"", false},
		{strings.Repeat("a", 64), false},     // label too long
		{strings.Repeat("a", 63), true},       // label exactly at limit
		{strings.Repeat("a.", 127) + "a", false}, // total too long
	}

	for _, tt := range tests {
		result := isValidRFC1123Hostname(tt.hostname)
		if result != tt.valid {
			t.Errorf("isValidRFC1123Hostname(%q) = %v, want %v", tt.hostname, result, tt.valid)
		}
	}
}

func TestParseAllPlatforms(t *testing.T) {
	for _, p := range []string{"darwin", "ubuntu", "debian", "any"} {
		input := "name: test\nplatform: " + p
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error for platform %q: %v", p, err)
		}
		if hasErrorCode(errs, E007) {
			t.Errorf("unexpected E007 for valid platform %q", p)
		}
	}
}

func TestSecretDefaults(t *testing.T) {
	input := `
name: test
platform: darwin
secrets:
  - name: MY_SECRET
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	s := cfg.Secrets[0]
	if !s.Required {
		t.Error("secret.required default should be true")
	}
	if s.Sensitive {
		t.Error("secret.sensitive default should be false")
	}
	if s.Prompt {
		t.Error("secret.prompt default should be false")
	}
	if s.Description != "" {
		t.Errorf("secret.description default should be empty, got %q", s.Description)
	}
}

func TestCustomStepDefaults(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    description: "test step"
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	step := cfg.CustomSteps["my-step"]
	if len(step.Platform) != 1 || step.Platform[0] != "any" {
		t.Errorf("step.platform default = %v, want [any]", step.Platform)
	}
	if len(step.Provides) != 0 {
		t.Errorf("step.provides default = %v, want []", step.Provides)
	}
	if len(step.Requires) != 0 {
		t.Errorf("step.requires default = %v, want []", step.Requires)
	}
	if len(step.Apply) != 0 {
		t.Errorf("step.apply default = %v, want {}", step.Apply)
	}
	if len(step.Rollback) != 0 {
		t.Errorf("step.rollback default = %v, want {}", step.Rollback)
	}
	if len(step.Env) != 0 {
		t.Errorf("step.env default = %v, want []", step.Env)
	}
	if len(step.Tags) != 0 {
		t.Errorf("step.tags default = %v, want []", step.Tags)
	}
}

// Helper functions

func hasErrorCode(errs []ValidationError, code ErrorCode) bool {
	for _, e := range errs {
		if e.Code == code {
			return true
		}
	}
	return false
}

func hasWarningCode(warns []ValidationWarning, code WarningCode) bool {
	for _, w := range warns {
		if w.Code == code {
			return true
		}
	}
	return false
}

func errorCodes(errs []ValidationError) []ErrorCode {
	codes := make([]ErrorCode, len(errs))
	for i, e := range errs {
		codes[i] = e.Code
	}
	return codes
}

func containsCode(codes []ErrorCode, code ErrorCode) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

