package secrets

import (
	"strings"
	"testing"
)

func TestFormatValidationResults(t *testing.T) {
	results := []SecretStatus{
		{Name: "GITHUB_TOKEN", Status: "valid", ValidateCmd: "gh auth status"},
		{Name: "OPENAI_API_KEY", Status: "missing_optional"},
		{Name: "SSH_PASSPHRASE", Status: "missing"},
		{Name: "DB_PASSWORD", Status: "invalid", ValidateError: "connection refused"},
		{Name: "PROMPTED_VAR", Status: "prompted"},
	}

	warnings := []string{
		`Warning: step "my-go-tool" references env var "GOPRIVATE" not declared in secrets section`,
	}

	output := FormatValidationResults(results, warnings)

	// Check for expected substrings
	checks := []string{
		"Pre-flight: Secrets",
		"GITHUB_TOKEN",
		"set (validated via: gh auth status)",
		"OPENAI_API_KEY",
		"not set (optional)",
		"SSH_PASSPHRASE",
		"not set (required",
		"DB_PASSWORD",
		"validation failed",
		"PROMPTED_VAR",
		"set (prompted)",
		"GOPRIVATE",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q, got:\n%s", check, output)
		}
	}
}
