// Package secrets provides secret validation, interactive prompting, and output masking.
package secrets

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/gregberns/adze/internal/config"
)

// SecretStatus represents the validation outcome for a single secret.
type SecretStatus struct {
	Name          string
	Status        string // "valid", "invalid", "missing", "missing_optional", "prompted"
	Description   string
	Sensitive     bool
	ValidateCmd   string
	ValidateError string // non-empty if validation command failed
}

// SecretManager manages secret validation, prompting, and masking.
type SecretManager struct {
	secrets []config.SecretEntry
	results map[string]SecretStatus
	mask    *MaskingFilter

	// promptFunc is the function used to prompt users for secrets.
	// It can be replaced in tests.
	promptFunc func(entry config.SecretEntry) (string, error)
}

// NewSecretManager creates a new manager from config secret entries.
func NewSecretManager(secrets []config.SecretEntry) *SecretManager {
	return &SecretManager{
		secrets:    secrets,
		results:    make(map[string]SecretStatus),
		mask:       NewMaskingFilter(),
		promptFunc: promptUser,
	}
}

// validateTimeout is the timeout for validation commands.
const validateTimeout = 30 * time.Second

// Validate runs pre-flight validation for all secrets.
// interactive: whether to prompt for missing secrets with prompt:true
func (m *SecretManager) Validate(ctx context.Context, interactive bool) []SecretStatus {
	var results []SecretStatus

	for _, entry := range m.secrets {
		status := m.validateEntry(ctx, entry, interactive)
		m.results[entry.Name] = status

		// Register sensitive values for masking
		if entry.Sensitive && (status.Status == "valid" || status.Status == "prompted") {
			val := os.Getenv(entry.Name)
			if val != "" {
				m.mask.Register(val)
			}
		}

		results = append(results, status)
	}

	return results
}

func (m *SecretManager) validateEntry(ctx context.Context, entry config.SecretEntry, interactive bool) SecretStatus {
	status := SecretStatus{
		Name:        entry.Name,
		Description: entry.Description,
		Sensitive:   entry.Sensitive,
		ValidateCmd: entry.Validate,
	}

	val := os.Getenv(entry.Name)

	if val != "" {
		// Value is set — run validation command if specified
		if entry.Validate != "" {
			err := runValidateCommand(ctx, entry.Validate)
			if err != nil {
				status.Status = "invalid"
				status.ValidateError = err.Error()
				return status
			}
		}
		status.Status = "valid"
		return status
	}

	// Value is NOT set
	if entry.Prompt && interactive {
		// Try to prompt the user
		prompted, err := m.promptFunc(entry)
		if err == nil && prompted != "" {
			os.Setenv(entry.Name, prompted)
			status.Status = "prompted"
			return status
		}
	}

	if entry.Required {
		status.Status = "missing"
	} else {
		status.Status = "missing_optional"
	}

	return status
}

// runValidateCommand executes a validation command with a timeout.
func runValidateCommand(ctx context.Context, command string) error {
	ctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	return cmd.Run()
}

// CrossReference checks step env vars against declared secrets.
// stepEnvVars maps step names to their list of env var names.
// Returns warnings for undeclared references.
func (m *SecretManager) CrossReference(stepEnvVars map[string][]string) []string {
	declared := make(map[string]bool)
	for _, s := range m.secrets {
		declared[s.Name] = true
	}

	var warnings []string
	for stepName, vars := range stepEnvVars {
		for _, v := range vars {
			if !declared[v] {
				warnings = append(warnings, `Warning: step "`+stepName+`" references env var "`+v+`" not declared in secrets section`)
			}
		}
	}

	return warnings
}

// IsAvailable returns whether a secret is available (set and valid).
func (m *SecretManager) IsAvailable(name string) bool {
	s, ok := m.results[name]
	if !ok {
		return false
	}
	return s.Status == "valid" || s.Status == "prompted"
}

// IsRequired returns whether a secret is required.
func (m *SecretManager) IsRequired(name string) bool {
	for _, entry := range m.secrets {
		if entry.Name == name {
			return entry.Required
		}
	}
	return false
}

// GetMask returns the masking filter for wrapping output writers.
func (m *SecretManager) GetMask() *MaskingFilter {
	return m.mask
}
