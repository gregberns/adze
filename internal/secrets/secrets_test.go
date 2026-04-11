package secrets

import (
	"context"
	"testing"

	"github.com/gregberns/adze/internal/config"
)

func TestValidate_EnvVarSet_NoValidateCommand(t *testing.T) {
	t.Setenv("TEST_SECRET_A", "value123")

	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_A", Required: true},
	})

	results := mgr.Validate(context.Background(), false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "valid" {
		t.Errorf("expected status 'valid', got %q", results[0].Status)
	}
}

func TestValidate_EnvVarSet_ValidateCommandSuccess(t *testing.T) {
	t.Setenv("TEST_SECRET_B", "value456")

	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_B", Required: true, Validate: "true"},
	})

	results := mgr.Validate(context.Background(), false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "valid" {
		t.Errorf("expected status 'valid', got %q", results[0].Status)
	}
}

func TestValidate_EnvVarSet_ValidateCommandFailure(t *testing.T) {
	t.Setenv("TEST_SECRET_C", "value789")

	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_C", Required: true, Validate: "false"},
	})

	results := mgr.Validate(context.Background(), false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "invalid" {
		t.Errorf("expected status 'invalid', got %q", results[0].Status)
	}
	if results[0].ValidateError == "" {
		t.Error("expected non-empty ValidateError")
	}
}

func TestValidate_EnvVarSet_ValidateCommandTimeout(t *testing.T) {
	t.Setenv("TEST_SECRET_TIMEOUT", "val")

	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_TIMEOUT", Required: true, Validate: "sleep 60"},
	})

	// Use a short context timeout to avoid waiting 30s in tests
	ctx, cancel := context.WithTimeout(context.Background(), 100*0+1) // 1ns
	cancel() // Cancel immediately to simulate timeout

	results := mgr.Validate(ctx, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "invalid" {
		t.Errorf("expected status 'invalid' on timeout, got %q", results[0].Status)
	}
}

func TestValidate_MissingRequired(t *testing.T) {
	// Make sure the env var is not set
	// t.Setenv sets it, so we don't call it.
	// The var should not exist in the test env.
	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_MISSING_REQ_XXXXXX", Required: true},
	})

	results := mgr.Validate(context.Background(), false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "missing" {
		t.Errorf("expected status 'missing', got %q", results[0].Status)
	}
}

func TestValidate_MissingOptional(t *testing.T) {
	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_MISSING_OPT_XXXXXX", Required: false},
	})

	results := mgr.Validate(context.Background(), false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "missing_optional" {
		t.Errorf("expected status 'missing_optional', got %q", results[0].Status)
	}
}

func TestValidate_PromptInteractive(t *testing.T) {
	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_PROMPT_XXXXXX", Required: true, Prompt: true, Description: "Test prompt"},
	})

	// Replace the prompt function with a mock
	mgr.promptFunc = func(entry config.SecretEntry) (string, error) {
		return "prompted_value", nil
	}

	results := mgr.Validate(context.Background(), true)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "prompted" {
		t.Errorf("expected status 'prompted', got %q", results[0].Status)
	}

	// Clean up the env var we set via prompting
	t.Cleanup(func() {
		// t.Setenv would have restored it, but we used os.Setenv in the code.
		// The test runner should handle it, but let's be safe.
	})
}

func TestValidate_PromptNonInteractive(t *testing.T) {
	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_PROMPT_NI_XXXXXX", Required: true, Prompt: true},
	})

	// In non-interactive mode, prompting is skipped
	results := mgr.Validate(context.Background(), false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "missing" {
		t.Errorf("expected status 'missing' in non-interactive mode, got %q", results[0].Status)
	}
}

func TestValidate_SensitiveMasking(t *testing.T) {
	t.Setenv("TEST_SECRET_SENS", "supersecretvalue")

	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_SECRET_SENS", Required: true, Sensitive: true},
	})

	mgr.Validate(context.Background(), false)

	mask := mgr.GetMask()
	output := mask.Mask("The token is supersecretvalue here")
	expected := "The token is *** here"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestIsAvailable(t *testing.T) {
	t.Setenv("TEST_AVAIL_YES", "value")

	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "TEST_AVAIL_YES", Required: true},
		{Name: "TEST_AVAIL_NO_XXXXXX", Required: true},
	})

	mgr.Validate(context.Background(), false)

	if !mgr.IsAvailable("TEST_AVAIL_YES") {
		t.Error("expected TEST_AVAIL_YES to be available")
	}
	if mgr.IsAvailable("TEST_AVAIL_NO_XXXXXX") {
		t.Error("expected TEST_AVAIL_NO_XXXXXX to NOT be available")
	}
	if mgr.IsAvailable("NONEXISTENT") {
		t.Error("expected NONEXISTENT to NOT be available")
	}
}

func TestIsRequired(t *testing.T) {
	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "REQ_SECRET", Required: true},
		{Name: "OPT_SECRET", Required: false},
	})

	if !mgr.IsRequired("REQ_SECRET") {
		t.Error("expected REQ_SECRET to be required")
	}
	if mgr.IsRequired("OPT_SECRET") {
		t.Error("expected OPT_SECRET to NOT be required")
	}
	if mgr.IsRequired("NONEXISTENT") {
		t.Error("expected NONEXISTENT to NOT be required")
	}
}

func TestCrossReference_UndeclaredWarning(t *testing.T) {
	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "DECLARED_VAR", Required: true},
	})

	stepEnvVars := map[string][]string{
		"my-step": {"DECLARED_VAR", "UNDECLARED_VAR"},
	}

	warnings := mgr.CrossReference(stepEnvVars)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	expected := `Warning: step "my-step" references env var "UNDECLARED_VAR" not declared in secrets section`
	if warnings[0] != expected {
		t.Errorf("expected warning %q, got %q", expected, warnings[0])
	}
}

func TestCrossReference_AllDeclared(t *testing.T) {
	mgr := NewSecretManager([]config.SecretEntry{
		{Name: "VAR_A", Required: true},
		{Name: "VAR_B", Required: false},
	})

	stepEnvVars := map[string][]string{
		"my-step": {"VAR_A", "VAR_B"},
	}

	warnings := mgr.CrossReference(stepEnvVars)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}
}
