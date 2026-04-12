package config

import (
	"strings"
	"testing"
)

// --- E001: top-level document must be a YAML mapping ---

func TestE001_EmptyDocument(t *testing.T) {
	_, errs, _, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E001)
}

func TestE001_ScalarDocument(t *testing.T) {
	_, errs, _, err := Parse([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E001)
}

func TestE001_SequenceDocument(t *testing.T) {
	_, errs, _, err := Parse([]byte("- a\n- b"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E001)
}

// --- E002: unknown top-level field ---

func TestE002_UnknownField(t *testing.T) {
	input := `
name: test
platform: darwin
unknown_field: value
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E002)
	assertErrorContains(t, errs, E002, "unknown_field")
}

// --- E003: name required field missing ---

func TestE003_NameMissing(t *testing.T) {
	input := `platform: darwin`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E003)
}

// --- E004: name must not be empty ---

func TestE004_NameEmpty(t *testing.T) {
	input := `
name: ""
platform: darwin
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E004)
}

// --- E005: name must not exceed 255 characters ---

func TestE005_NameTooLong(t *testing.T) {
	name := strings.Repeat("x", 256)
	input := "name: " + name + "\nplatform: darwin"
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E005)
}

func TestE005_NameExactly255(t *testing.T) {
	name := strings.Repeat("x", 255)
	input := "name: " + name + "\nplatform: darwin"
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E005)
}

// --- E006: platform required field missing ---

func TestE006_PlatformMissing(t *testing.T) {
	input := `name: test`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E006)
}

// --- E007: platform invalid value ---

func TestE007_PlatformInvalid(t *testing.T) {
	input := `
name: test
platform: windows
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E007)
	assertErrorContains(t, errs, E007, "windows")
}

// --- E008: tags element invalid ---

func TestE008_EmptyTag(t *testing.T) {
	input := `
name: test
platform: darwin
tags: [""]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E008)
}

func TestE008_TagWithWhitespace(t *testing.T) {
	input := `
name: test
platform: darwin
tags: ["has space"]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E008)
}

// --- E009: include path empty ---

func TestE009_IncludeEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
include:
  - ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E009)
}

// --- E010: include remote URLs rejected ---

func TestE010_IncludeRemoteURL(t *testing.T) {
	urls := []string{
		"http://example.com/config.yaml",
		"https://example.com/config.yaml",
		"ftp://example.com/config.yaml",
		"ssh://example.com/config.yaml",
		"git://example.com/config.yaml",
	}
	for _, u := range urls {
		input := "name: test\nplatform: darwin\ninclude:\n  - " + u
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error for URL %q: %v", u, err)
		}
		if !hasErrorCode(errs, E010) {
			t.Errorf("expected E010 for URL %q", u)
		}
	}
}

func TestE010_LocalPathAllowed(t *testing.T) {
	input := `
name: test
platform: darwin
include:
  - shared/git.yaml
  - /absolute/path.yaml
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E010)
}

// --- E011: machine.hostname invalid RFC 1123 ---

func TestE011_HostnameInvalid(t *testing.T) {
	tests := []struct {
		hostname string
	}{
		{"-leading-hyphen"},
		{"trailing-hyphen-"},
		{strings.Repeat("a", 64)},
		{"has space"},
		{"has_underscore"},
	}
	for _, tt := range tests {
		input := "name: test\nplatform: darwin\nmachine:\n  hostname: " + tt.hostname
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasErrorCode(errs, E011) {
			t.Errorf("expected E011 for hostname %q", tt.hostname)
		}
	}
}

func TestE011_HostnameValid(t *testing.T) {
	tests := []string{
		"greg-mbp",
		"my.host.name",
		"a",
		"a-b-c",
	}
	for _, h := range tests {
		input := "name: test\nplatform: darwin\nmachine:\n  hostname: " + h
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasErrorCode(errs, E011) {
			t.Errorf("unexpected E011 for valid hostname %q", h)
		}
	}
}

// --- E012: identity.git_name empty if present ---

func TestE012_GitNameEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  git_name: ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E012)
}

// --- E013: identity.git_email empty if present ---

func TestE013_GitEmailEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  git_email: ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E013)
}

// --- W001: identity.git_email not an email ---

func TestW001_GitEmailNoAt(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  git_email: "not-an-email"
`
	_, _, warns, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasWarningCode(warns, W001) {
		t.Error("expected W001 for email without @")
	}
}

func TestW001_GitEmailWithAt(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  git_email: "greg@example.com"
`
	_, _, warns, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasWarningCode(warns, W001) {
		t.Error("unexpected W001 for valid email")
	}
}

// --- E014: identity.github_user empty if present ---

func TestE014_GithubUserEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  github_user: ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E014)
}

// --- E015: identity.github_user whitespace ---

func TestE015_GithubUserWhitespace(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  github_user: "has space"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E015)
}

// --- E016: secrets[i].name missing ---

func TestE016_SecretNameMissing(t *testing.T) {
	input := `
name: test
platform: darwin
secrets:
  - description: "no name"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E016)
}

// --- E017: secrets[i].name pattern mismatch ---

func TestE017_SecretNamePattern(t *testing.T) {
	tests := []string{
		"lowercase",
		"123START",
		"HAS SPACE",
		"has-hyphen",
	}
	for _, name := range tests {
		input := "name: test\nplatform: darwin\nsecrets:\n  - name: " + name
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasErrorCode(errs, E017) {
			t.Errorf("expected E017 for secret name %q", name)
		}
	}
}

func TestE017_ValidSecretNames(t *testing.T) {
	tests := []string{
		"GITHUB_TOKEN",
		"A",
		"ABC123",
		"MY_VAR_2",
	}
	for _, name := range tests {
		input := "name: test\nplatform: darwin\nsecrets:\n  - name: " + name
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasErrorCode(errs, E017) {
			t.Errorf("unexpected E017 for valid secret name %q", name)
		}
	}
}

// --- E018: secrets[i].name duplicate ---

func TestE018_DuplicateSecret(t *testing.T) {
	input := `
name: test
platform: darwin
secrets:
  - name: GITHUB_TOKEN
  - name: GITHUB_TOKEN
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E018)
}

// --- E019: packages name invalid ---

func TestE019_PackageNameEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E019)
}

func TestE019_PackageNameWhitespace(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - "has space"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E019)
}

// --- E020: packages version unquoted numeric ---

func TestE020_UnquotedFloatVersion(t *testing.T) {
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
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E020)
}

func TestE020_UnquotedIntVersion(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: node
      version: 20
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E020)
}

func TestE020_QuotedVersionOK(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: python
      version: "3.11"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E020)
}

// --- E021: packages version empty if present ---

func TestE021_VersionEmptyIfPresent(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: terraform
      version: ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E021)
}

func TestE021_VersionAbsentIsOK(t *testing.T) {
	// When version key is absent entirely, no E021
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: terraform
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E021)
}

func TestE021_VersionNonEmptyIsOK(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: terraform
      version: "1.7.5"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E021)
}

func TestE021_ShortFormNoE021(t *testing.T) {
	// Short form packages should never trigger E021
	input := `
name: test
platform: darwin
packages:
  brew:
    - terraform
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E021)
}

// --- E022: packages duplicate name ---

func TestE022_DuplicatePackage(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - git
    - git
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E022)
}

func TestE022_DuplicateAcrossListsOK(t *testing.T) {
	// Same package name in brew and cask should be allowed
	input := `
name: test
platform: darwin
packages:
  brew:
    - git
  cask:
    - git
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E022)
}

// --- E023: defaults domain key empty ---

func TestE023_EmptyDomainKey(t *testing.T) {
	input := `
name: test
platform: darwin
defaults:
  "":
    key: value
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E023)
}

// --- E024: defaults preference key empty ---

func TestE024_EmptyPreferenceKey(t *testing.T) {
	input := `
name: test
platform: darwin
defaults:
  com.test:
    "": value
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E024)
}

// --- E025: defaults null value ---

func TestE025_DefaultsNullValue(t *testing.T) {
	input := `
name: test
platform: darwin
defaults:
  com.test:
    key: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E025)
}

// --- E026: defaults unsupported type ---

func TestE026_DefaultsUnsupportedType(t *testing.T) {
	input := `
name: test
platform: darwin
defaults:
  com.test:
    key: [1, 2, 3]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E026)
}

// --- E027: dock.apps element empty ---

func TestE027_DockAppEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
dock:
  apps:
    - ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E027)
}

// --- E028: shell.default invalid value ---

func TestE028_ShellDefaultInvalid(t *testing.T) {
	input := `
name: test
platform: darwin
shell:
  default: powershell
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E028)
}

func TestE028_ShellDefaultValid(t *testing.T) {
	for _, sh := range []string{"zsh", "bash", "fish"} {
		input := "name: test\nplatform: darwin\nshell:\n  default: " + sh
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNoError(t, errs, E028)
	}
}

// --- E029: shell.theme empty if present ---

func TestE029_ShellThemeEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
shell:
  theme: ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E029)
}

// --- E030: shell.plugins element invalid ---

func TestE030_PluginEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
shell:
  plugins:
    - ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E030)
}

func TestE030_PluginWhitespace(t *testing.T) {
	input := `
name: test
platform: darwin
shell:
  plugins:
    - "has space"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E030)
}

// --- E031: directories element empty ---

func TestE031_DirectoryEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
directories:
  - ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E031)
}

// --- W002: directories duplicate entry ---

func TestW002_DuplicateDirectory(t *testing.T) {
	input := `
name: test
platform: darwin
directories:
  - ~/github
  - ~/github
`
	_, _, warns, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasWarningCode(warns, W002) {
		t.Error("expected W002 for duplicate directory")
	}
}

func TestW002_NoDuplicateDirectory(t *testing.T) {
	input := `
name: test
platform: darwin
directories:
  - ~/github
  - ~/gitlab
`
	_, _, warns, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasWarningCode(warns, W002) {
		t.Error("unexpected W002 for non-duplicate directories")
	}
}

// --- E032: custom_steps name pattern mismatch ---

func TestE032_StepNameInvalid(t *testing.T) {
	tests := []string{
		"Has_Underscore",
		"UPPERCASE",
		"123start",
		"-leading-hyphen",
	}
	for _, name := range tests {
		input := "name: test\nplatform: darwin\ncustom_steps:\n  " + name + ":\n    description: test"
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasErrorCode(errs, E032) {
			t.Errorf("expected E032 for step name %q", name)
		}
	}
}

func TestE032_StepNameValid(t *testing.T) {
	tests := []string{
		"my-step",
		"a",
		"abc123",
		"my-go-tool",
	}
	for _, name := range tests {
		input := "name: test\nplatform: darwin\ncustom_steps:\n  " + name + ":\n    description: test"
		_, errs, _, err := Parse([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasErrorCode(errs, E032) {
			t.Errorf("unexpected E032 for valid step name %q", name)
		}
	}
}

// --- E033: custom_steps provides element invalid ---

func TestE033_ProvidesEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    provides: [""]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E033)
}

func TestE033_ProvidesWhitespace(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    provides: ["has space"]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E033)
}

// --- SEM-23: custom_steps provides uniqueness ---

func TestSEM23_ProvidesDuplicate(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    provides: [foo, bar, foo]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E033)
	assertErrorContains(t, errs, E033, "duplicate entry")
}

func TestSEM23_ProvidesUnique(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    provides: [foo, bar, baz]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not have a duplicate provides error
	for _, e := range errs {
		if e.Code == E033 && strings.Contains(e.Message, "duplicate") {
			t.Errorf("unexpected duplicate provides error: %v", e)
		}
	}
}

func TestSEM23_ProvidesDuplicateMultipleSteps(t *testing.T) {
	// Duplicate within the same step's provides should error,
	// but same provides value in different steps should be OK
	input := `
name: test
platform: darwin
custom_steps:
  step-a:
    provides: [foo]
  step-b:
    provides: [foo]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range errs {
		if e.Code == E033 && strings.Contains(e.Message, "duplicate") {
			t.Errorf("unexpected duplicate provides error across steps: %v", e)
		}
	}
}

// --- E034: custom_steps requires element invalid ---

func TestE034_RequiresEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    requires: [""]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E034)
}

// --- E035: custom_steps platform element invalid ---

func TestE035_PlatformInvalid(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    platform: [windows]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E035)
}

func TestE035_PlatformValid(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    platform: [darwin, ubuntu, debian, any]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E035)
}

// --- E036: custom_steps apply key invalid ---

func TestE036_ApplyKeyInvalid(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    apply:
      windows: "echo hi"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E036)
}

// --- E037: custom_steps apply command empty ---

func TestE037_ApplyCommandEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    apply:
      darwin: ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E037)
}

// --- E038: custom_steps rollback key invalid ---

func TestE038_RollbackKeyInvalid(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    rollback:
      windows: "echo hi"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E038)
}

// --- E039: custom_steps rollback command empty ---

func TestE039_RollbackCommandEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    rollback:
      darwin: ""
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E039)
}

// --- E040: custom_steps env element invalid ---

func TestE040_EnvInvalid(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    env: [lowercase]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E040)
}

func TestE040_EnvValid(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    env: [GOPRIVATE, MY_VAR]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoError(t, errs, E040)
}

// --- E041: custom_steps tags element invalid ---

func TestE041_TagsEmpty(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    tags: [""]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E041)
}

func TestE041_TagsWhitespace(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    tags: ["has space"]
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E041)
}

// --- E042: type mismatch ---

func TestE042_TagsNotSequence(t *testing.T) {
	input := `
name: test
platform: darwin
tags: "not a list"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E042)
}

func TestE042_MachineNotMapping(t *testing.T) {
	input := `
name: test
platform: darwin
machine: "not a mapping"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E042)
}

func TestE042_IncludeNotSequence(t *testing.T) {
	input := `
name: test
platform: darwin
include: "not a list"
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E042)
}

// --- TYP-04: null values rejected for scalar fields ---

func TestTYP04_NullMachineHostname(t *testing.T) {
	input := `
name: test
platform: darwin
machine:
  hostname: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHasError(t, errs, E042)
	assertErrorContains(t, errs, E042, "machine.hostname")
}

func TestTYP04_NullIdentityFields(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  git_name: null
  git_email: null
  github_user: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should get 3 E042 errors, one for each null identity field
	count := 0
	for _, e := range errs {
		if e.Code == E042 && strings.Contains(e.Message, "identity.") {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 E042 errors for null identity fields, got %d; errors: %v", count, errs)
	}
}

func TestTYP04_NullShellScalarFields(t *testing.T) {
	input := `
name: test
platform: darwin
shell:
  default: null
  oh_my_zsh: null
  theme: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, e := range errs {
		if e.Code == E042 && strings.Contains(e.Message, "shell.") {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 E042 errors for null shell fields, got %d; errors: %v", count, errs)
	}
}

func TestTYP04_NullCustomStepScalarFields(t *testing.T) {
	input := `
name: test
platform: darwin
custom_steps:
  my-step:
    description: null
    check: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, e := range errs {
		if e.Code == E042 && strings.Contains(e.Message, "custom_steps.") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 E042 errors for null custom_step fields, got %d; errors: %v", count, errs)
	}
}

func TestTYP04_NullPackageEntryFields(t *testing.T) {
	input := `
name: test
platform: darwin
packages:
  brew:
    - name: null
      version: null
      pinned: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, e := range errs {
		if e.Code == E042 && strings.Contains(e.Message, "packages.brew") {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 E042 errors for null package entry fields, got %d; errors: %v", count, errs)
	}
}

func TestTYP04_NullSecretFields(t *testing.T) {
	input := `
name: test
platform: darwin
secrets:
  - name: null
    required: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, e := range errs {
		if e.Code == E042 && strings.Contains(e.Message, "secrets[]") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 E042 errors for null secret fields, got %d; errors: %v", count, errs)
	}
}

func TestTYP04_NullNameAndPlatform(t *testing.T) {
	input := `
name: null
platform: null
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, e := range errs {
		if e.Code == E042 {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 E042 errors for null name and platform, got %d; errors: %v", count, errs)
	}
}

// --- Warning separation ---

func TestWarningsSeparateFromErrors(t *testing.T) {
	input := `
name: test
platform: darwin
identity:
  git_email: "noemail"
directories:
  - ~/github
  - ~/github
`
	_, errs, warns, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have warnings but no errors (for this particular input)
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}

	// W001 for email, W002 for duplicate directory
	if !hasWarningCode(warns, W001) {
		t.Error("expected W001")
	}
	if !hasWarningCode(warns, W002) {
		t.Error("expected W002")
	}
}

// --- Multiple errors collected ---

func TestMultipleErrorsCollected(t *testing.T) {
	input := `
unknown_field: foo
tags: [""]
shell:
  default: powershell
`
	_, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: E002, E003, E006, E008, E028
	if len(errs) < 4 {
		t.Errorf("expected at least 4 errors (validate-all), got %d: %v", len(errs), errs)
	}
	assertHasError(t, errs, E002)
	assertHasError(t, errs, E003)
	assertHasError(t, errs, E006)
	assertHasError(t, errs, E008)
	assertHasError(t, errs, E028)
}

// --- Nil/empty section normalization ---

func TestNilSectionNormalization(t *testing.T) {
	input := `
name: test
platform: darwin
tags: null
include: null
secrets: null
packages: null
defaults: null
dock: null
shell: null
directories: null
custom_steps: null
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if cfg.Tags == nil {
		t.Error("tags should be empty slice, not nil")
	}
	if cfg.Include == nil {
		t.Error("include should be empty slice, not nil")
	}
	if cfg.Secrets == nil {
		t.Error("secrets should be empty slice, not nil")
	}
	if cfg.Defaults == nil {
		t.Error("defaults should be empty map, not nil")
	}
	if cfg.Dock.Apps == nil {
		t.Error("dock.apps should be empty slice, not nil")
	}
	if cfg.Shell.Plugins == nil {
		t.Error("shell.plugins should be empty slice, not nil")
	}
	if cfg.Directories == nil {
		t.Error("directories should be empty slice, not nil")
	}
	if cfg.CustomSteps == nil {
		t.Error("custom_steps should be empty map, not nil")
	}
}

// --- Defaults with various types ---

func TestDefaultsAllTypes(t *testing.T) {
	input := `
name: test
platform: darwin
defaults:
  domain:
    boolTrue: true
    boolFalse: false
    intVal: 42
    floatVal: 3.14
    strQuoted: "hello"
    strUnquoted: world
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	d := cfg.Defaults["domain"]

	if v := d["boolTrue"].Value; v != true {
		t.Errorf("boolTrue = %v (%T), want true", v, v)
	}
	if v := d["boolFalse"].Value; v != false {
		t.Errorf("boolFalse = %v (%T), want false", v, v)
	}
	if v := d["intVal"].Value; v != 42 {
		t.Errorf("intVal = %v (%T), want 42", v, v)
	}
	if v := d["floatVal"].Value; v != 3.14 {
		t.Errorf("floatVal = %v (%T), want 3.14", v, v)
	}
	if v := d["strQuoted"].Value; v != "hello" {
		t.Errorf("strQuoted = %v (%T), want hello", v, v)
	}
	if v := d["strUnquoted"].Value; v != "world" {
		t.Errorf("strUnquoted = %v (%T), want world", v, v)
	}
}

// --- Edge case: YAML 1.1 booleans treated as strings ---

func TestYAML11BooleansAsStrings(t *testing.T) {
	// In YAML 1.2 (yaml.v3), yes/no/on/off are strings, not booleans
	input := `
name: test
platform: darwin
tags:
  - "yes"
  - "no"
  - "on"
  - "off"
`
	cfg, errs, _, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// yaml.v3 treats these as strings in YAML 1.2
	expected := []string{"yes", "no", "on", "off"}
	for i, want := range expected {
		if i < len(cfg.Tags) && cfg.Tags[i] != want {
			t.Errorf("tags[%d] = %q, want %q", i, cfg.Tags[i], want)
		}
	}
}

// --- Helper assertions ---

func assertHasError(t *testing.T, errs []ValidationError, code ErrorCode) {
	t.Helper()
	if !hasErrorCode(errs, code) {
		t.Errorf("expected error code %s, got codes: %v", code, errorCodes(errs))
	}
}

func assertNoError(t *testing.T, errs []ValidationError, code ErrorCode) {
	t.Helper()
	if hasErrorCode(errs, code) {
		t.Errorf("unexpected error code %s", code)
	}
}

func assertErrorContains(t *testing.T, errs []ValidationError, code ErrorCode, substr string) {
	t.Helper()
	for _, e := range errs {
		if e.Code == code {
			if strings.Contains(e.Message, substr) {
				return
			}
		}
	}
	t.Errorf("expected error %s containing %q, not found in: %v", code, substr, errs)
}
