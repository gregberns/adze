package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates a file at the given path with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}

// --- Basic include merging ---

func TestInclude_BasicMerge(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "shared.yaml"), `
identity:
  git_name: "Shared User"
  git_email: "shared@example.com"
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared.yaml
identity:
  github_user: testuser
`)

	cfg, errs, warns, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}
	_ = warns

	if cfg.Identity.GitName != "Shared User" {
		t.Errorf("identity.git_name = %q, want Shared User", cfg.Identity.GitName)
	}
	if cfg.Identity.GitEmail != "shared@example.com" {
		t.Errorf("identity.git_email = %q, want shared@example.com", cfg.Identity.GitEmail)
	}
	if cfg.Identity.GithubUser != "testuser" {
		t.Errorf("identity.github_user = %q, want testuser", cfg.Identity.GithubUser)
	}
}

// --- Nested includes (A includes B includes C) ---

func TestInclude_NestedIncludes(t *testing.T) {
	dir := t.TempDir()

	// C defines git_name
	writeTestFile(t, filepath.Join(dir, "shared", "c.yaml"), `
identity:
  git_name: "C User"
`)

	// B includes C and adds git_email
	writeTestFile(t, filepath.Join(dir, "shared", "b.yaml"), `
include:
  - c.yaml
identity:
  git_email: "b@example.com"
`)

	// A (main) includes B and adds platform/name/github_user
	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared/b.yaml
identity:
  github_user: auser
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	// C's git_name should flow through
	if cfg.Identity.GitName != "C User" {
		t.Errorf("identity.git_name = %q, want C User", cfg.Identity.GitName)
	}
	// B's git_email should be present
	if cfg.Identity.GitEmail != "b@example.com" {
		t.Errorf("identity.git_email = %q, want b@example.com", cfg.Identity.GitEmail)
	}
	// A's github_user should be present
	if cfg.Identity.GithubUser != "auser" {
		t.Errorf("identity.github_user = %q, want auser", cfg.Identity.GithubUser)
	}
}

// --- Override behavior (main overrides include) ---

func TestInclude_MainOverridesInclude(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "shared.yaml"), `
identity:
  git_name: "Shared User"
  git_email: "shared@example.com"
shell:
  default: bash
  theme: default-theme
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared.yaml
identity:
  git_name: "Main User"
shell:
  default: zsh
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	// Main should override
	if cfg.Identity.GitName != "Main User" {
		t.Errorf("identity.git_name = %q, want Main User", cfg.Identity.GitName)
	}
	// Include value should be preserved where not overridden
	if cfg.Identity.GitEmail != "shared@example.com" {
		t.Errorf("identity.git_email = %q, want shared@example.com", cfg.Identity.GitEmail)
	}
	if cfg.Shell.Default != "zsh" {
		t.Errorf("shell.default = %q, want zsh", cfg.Shell.Default)
	}
	if cfg.Shell.Theme != "default-theme" {
		t.Errorf("shell.theme = %q, want default-theme", cfg.Shell.Theme)
	}
}

// --- List concatenation and deduplication ---

func TestInclude_ListConcatAndDedup(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "shared.yaml"), `
tags:
  - dev
  - personal
directories:
  - ~/github
  - ~/projects
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared.yaml
tags:
  - personal
  - work
directories:
  - ~/projects
  - ~/work
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	// Tags: dev, personal, work (dedup by exact match)
	if len(cfg.Tags) != 3 {
		t.Fatalf("tags length = %d, want 3; tags = %v", len(cfg.Tags), cfg.Tags)
	}
	if cfg.Tags[0] != "dev" || cfg.Tags[1] != "personal" || cfg.Tags[2] != "work" {
		t.Errorf("tags = %v, want [dev personal work]", cfg.Tags)
	}

	// Directories: ~/github, ~/projects, ~/work
	if len(cfg.Directories) != 3 {
		t.Fatalf("directories length = %d, want 3; dirs = %v", len(cfg.Directories), cfg.Directories)
	}
	if cfg.Directories[0] != "~/github" || cfg.Directories[1] != "~/projects" || cfg.Directories[2] != "~/work" {
		t.Errorf("directories = %v, want [~/github ~/projects ~/work]", cfg.Directories)
	}
}

// --- Package entry dedup (string form, object form, mixed) ---

func TestInclude_PackageEntryDedup(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "shared.yaml"), `
packages:
  brew:
    - git
    - name: python
      version: "3.10"
    - jq
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared.yaml
packages:
  brew:
    - name: python
      version: "3.11"
    - name: git
      version: "2.40"
    - ripgrep
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	// Expect: git (override object form), python (override version 3.11), jq (base), ripgrep (override)
	brew := cfg.Packages.Brew
	if len(brew) != 4 {
		t.Fatalf("packages.brew length = %d, want 4; brew = %v", len(brew), brew)
	}

	// git should be object form from override
	if brew[0].Name != "git" || brew[0].Version != "2.40" {
		t.Errorf("brew[0] = %+v, want {Name:git Version:2.40}", brew[0])
	}
	// python should have override version
	if brew[1].Name != "python" || brew[1].Version != "3.11" {
		t.Errorf("brew[1] = %+v, want {Name:python Version:3.11}", brew[1])
	}
	// jq from base
	if brew[2].Name != "jq" {
		t.Errorf("brew[2] = %+v, want {Name:jq}", brew[2])
	}
	// ripgrep from override
	if brew[3].Name != "ripgrep" {
		t.Errorf("brew[3] = %+v, want {Name:ripgrep}", brew[3])
	}
}

// --- Null override removes keys ---

func TestInclude_NullOverrideRemovesKeys(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "shared.yaml"), `
identity:
  git_name: "Shared User"
  git_email: "shared@example.com"
machine:
  hostname: shared-host
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared.yaml
machine: null
identity:
  git_name: "Main User"
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	// machine should be removed by null override
	if cfg.Machine.Hostname != "" {
		t.Errorf("machine.hostname = %q, want empty (null override)", cfg.Machine.Hostname)
	}
	// identity should have overridden git_name
	if cfg.Identity.GitName != "Main User" {
		t.Errorf("identity.git_name = %q, want Main User", cfg.Identity.GitName)
	}
	// identity.git_email from include should be preserved
	if cfg.Identity.GitEmail != "shared@example.com" {
		t.Errorf("identity.git_email = %q, want shared@example.com", cfg.Identity.GitEmail)
	}
}

// --- Type mismatch warning ---

func TestInclude_TypeMismatchWarning(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "shared.yaml"), `
tags:
  - dev
  - personal
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared.yaml
tags: "not-a-list"
`)

	cfg, errs, warns, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The type mismatch means override wins, tags becomes a scalar.
	// The parser will then fail to parse tags as a list.
	// Check for either a validation error about tags or the warning.
	_ = cfg
	_ = errs

	// Should have at least one W003 warning about type mismatch
	hasTypeMismatch := false
	for _, w := range warns {
		if w.Code == W003 {
			hasTypeMismatch = true
			break
		}
	}
	if !hasTypeMismatch {
		t.Error("expected W003 type mismatch warning")
	}
}

// --- Circular include detection ---

func TestInclude_CircularDetection(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "a.yaml"), `
name: test
platform: darwin
include:
  - b.yaml
`)

	writeTestFile(t, filepath.Join(dir, "b.yaml"), `
include:
  - c.yaml
`)

	writeTestFile(t, filepath.Join(dir, "c.yaml"), `
include:
  - a.yaml
`)

	_, _, _, err := LoadConfig(filepath.Join(dir, "a.yaml"), false)
	if err == nil {
		t.Fatal("expected error for circular include")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "circular include detected") {
		t.Errorf("expected circular include error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "cycle") {
		t.Errorf("expected 'cycle' in error message, got: %s", errMsg)
	}
}

// --- Depth limit exceeded ---

func TestInclude_DepthLimitExceeded(t *testing.T) {
	dir := t.TempDir()

	// Create a chain of 11 files, each including the next
	for i := 0; i <= 10; i++ {
		var content string
		if i < 10 {
			content = "include:\n  - file" + itoa(i+1) + ".yaml\n"
		} else {
			content = "identity:\n  git_name: deep\n"
		}
		writeTestFile(t, filepath.Join(dir, "file"+itoa(i)+".yaml"), content)
	}

	// Main includes file0
	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - file0.yaml
`)

	_, _, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err == nil {
		t.Fatal("expected error for depth limit exceeded")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "include depth limit") {
		t.Errorf("expected depth limit error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "10") {
		t.Errorf("expected '10' in error message, got: %s", errMsg)
	}
}

// --- Missing include file ---

func TestInclude_MissingFile(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - nonexistent.yaml
`)

	_, _, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err == nil {
		t.Fatal("expected error for missing include file")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "include file not found") {
		t.Errorf("expected 'include file not found' error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "nonexistent.yaml") {
		t.Errorf("expected path in error, got: %s", errMsg)
	}
}

// --- Remote URL rejection ---

func TestInclude_RemoteURLRejection(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - https://example.com/shared.yaml
`)

	_, _, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err == nil {
		t.Fatal("expected error for remote URL include")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "remote includes are not supported") {
		t.Errorf("expected remote URL rejection, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "https://example.com/shared.yaml") {
		t.Errorf("expected URL in error, got: %s", errMsg)
	}
}

func TestInclude_RemoteHTTPURLRejection(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - http://example.com/shared.yaml
`)

	_, _, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err == nil {
		t.Fatal("expected error for remote URL include")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "remote includes are not supported") {
		t.Errorf("expected remote URL rejection, got: %s", errMsg)
	}
}

// --- URL-sourced config includes rejection ---

func TestInclude_URLSourcedConfigRejectsIncludes(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared.yaml
`)

	_, _, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), true)
	if err == nil {
		t.Fatal("expected error for URL-sourced config with includes")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "URL-sourced configs cannot use includes") {
		t.Errorf("expected URL-sourced rejection, got: %s", errMsg)
	}
}

// --- Relative path resolution ---

func TestInclude_RelativePathResolution(t *testing.T) {
	dir := t.TempDir()

	// Create nested structure: main includes shared/base.yaml
	// shared/base.yaml includes extras/shell.yaml (relative to shared/)
	writeTestFile(t, filepath.Join(dir, "shared", "extras", "shell.yaml"), `
shell:
  default: zsh
  theme: shared-theme
`)

	writeTestFile(t, filepath.Join(dir, "shared", "base.yaml"), `
include:
  - extras/shell.yaml
identity:
  git_name: "Base User"
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - shared/base.yaml
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	if cfg.Identity.GitName != "Base User" {
		t.Errorf("identity.git_name = %q, want Base User", cfg.Identity.GitName)
	}
	if cfg.Shell.Default != "zsh" {
		t.Errorf("shell.default = %q, want zsh", cfg.Shell.Default)
	}
	if cfg.Shell.Theme != "shared-theme" {
		t.Errorf("shell.theme = %q, want shared-theme", cfg.Shell.Theme)
	}
}

// --- No includes: passes through to normal parsing ---

func TestInclude_NoIncludes(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
identity:
  git_name: "Direct User"
  git_email: "direct@example.com"
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	if cfg.Identity.GitName != "Direct User" {
		t.Errorf("identity.git_name = %q, want Direct User", cfg.Identity.GitName)
	}
}

// --- Multiple includes, later overrides earlier ---

func TestInclude_MultipleIncludesOrder(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "first.yaml"), `
identity:
  git_name: "First User"
  git_email: "first@example.com"
shell:
  default: bash
`)

	writeTestFile(t, filepath.Join(dir, "second.yaml"), `
identity:
  git_name: "Second User"
  github_user: seconduser
shell:
  default: zsh
`)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - first.yaml
  - second.yaml
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	// Second should override first for conflicting values
	if cfg.Identity.GitName != "Second User" {
		t.Errorf("identity.git_name = %q, want Second User", cfg.Identity.GitName)
	}
	// First's unique value should be preserved
	if cfg.Identity.GitEmail != "first@example.com" {
		t.Errorf("identity.git_email = %q, want first@example.com", cfg.Identity.GitEmail)
	}
	// Second's unique value
	if cfg.Identity.GithubUser != "seconduser" {
		t.Errorf("identity.github_user = %q, want seconduser", cfg.Identity.GithubUser)
	}
	// Second overrides first's shell
	if cfg.Shell.Default != "zsh" {
		t.Errorf("shell.default = %q, want zsh", cfg.Shell.Default)
	}
}

// --- Self-include detection ---

func TestInclude_SelfIncludeDetection(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - main.yaml
`)

	_, _, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err == nil {
		t.Fatal("expected error for self-include")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "circular include detected") {
		t.Errorf("expected circular include error, got: %s", errMsg)
	}
}

// --- Empty include file ---

func TestInclude_EmptyIncludeFile(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, filepath.Join(dir, "empty.yaml"), ``)

	writeTestFile(t, filepath.Join(dir, "main.yaml"), `
name: test
platform: darwin
include:
  - empty.yaml
`)

	cfg, errs, _, err := LoadConfig(filepath.Join(dir, "main.yaml"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
		t.FailNow()
	}

	if cfg.Name != "test" {
		t.Errorf("name = %q, want test", cfg.Name)
	}
}

// simple itoa for small non-negative integers
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
