package secrets

import "fmt"

// FormatValidationResults produces the human-readable pre-flight output.
//
// Output format:
//
//	Pre-flight: Secrets
//	  ✓ GITHUB_TOKEN       set (validated via: gh auth status)
//	  ⚠ OPENAI_API_KEY     not set (optional)
//	  ✗ SSH_PASSPHRASE     not set (required — will prompt during apply)
//	  ⚠ GOPRIVATE          referenced by step "my-go-tool" but not declared in secrets
func FormatValidationResults(results []SecretStatus, crossRefWarnings []string) string {
	var out string
	out += "Pre-flight: Secrets\n"

	for _, r := range results {
		switch r.Status {
		case "valid":
			line := fmt.Sprintf("  ✓ %-20s set", r.Name)
			if r.ValidateCmd != "" {
				line += fmt.Sprintf(" (validated via: %s)", r.ValidateCmd)
			}
			out += line + "\n"
		case "prompted":
			out += fmt.Sprintf("  ✓ %-20s set (prompted)\n", r.Name)
		case "invalid":
			line := fmt.Sprintf("  ✗ %-20s set but validation failed", r.Name)
			if r.ValidateError != "" {
				line += fmt.Sprintf(": %s", r.ValidateError)
			}
			out += line + "\n"
		case "missing":
			out += fmt.Sprintf("  ✗ %-20s not set (required — will prompt during apply)\n", r.Name)
		case "missing_optional":
			out += fmt.Sprintf("  ⚠ %-20s not set (optional)\n", r.Name)
		}
	}

	for _, w := range crossRefWarnings {
		// Extract step name and var name from the warning string for formatting
		out += "  ⚠ " + w + "\n"
	}

	return out
}
