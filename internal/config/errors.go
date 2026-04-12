package config

import "fmt"

// ErrorCode identifies a specific validation error type.
type ErrorCode string

const (
	E001 ErrorCode = "E001" // top-level document must be a YAML mapping
	E002 ErrorCode = "E002" // unknown field at top level
	E003 ErrorCode = "E003" // name required field missing
	E004 ErrorCode = "E004" // name must not be empty
	E005 ErrorCode = "E005" // name must not exceed 255 characters
	E006 ErrorCode = "E006" // platform required field missing
	E007 ErrorCode = "E007" // platform invalid value
	E008 ErrorCode = "E008" // tags element invalid
	E009 ErrorCode = "E009" // include path empty
	E010 ErrorCode = "E010" // include remote URLs rejected
	E011 ErrorCode = "E011" // machine.hostname invalid RFC 1123
	E012 ErrorCode = "E012" // identity.git_name empty if present
	E013 ErrorCode = "E013" // identity.git_email empty if present
	E014 ErrorCode = "E014" // identity.github_user empty if present
	E015 ErrorCode = "E015" // identity.github_user whitespace
	E016 ErrorCode = "E016" // secrets[i].name missing
	E017 ErrorCode = "E017" // secrets[i].name pattern mismatch
	E018 ErrorCode = "E018" // secrets[i].name duplicate
	E019 ErrorCode = "E019" // packages name invalid
	E020 ErrorCode = "E020" // packages version unquoted numeric
	E021 ErrorCode = "E021" // packages version empty if present
	E022 ErrorCode = "E022" // packages duplicate name
	E023 ErrorCode = "E023" // defaults domain key empty
	E024 ErrorCode = "E024" // defaults preference key empty
	E025 ErrorCode = "E025" // defaults null value
	E026 ErrorCode = "E026" // defaults unsupported type
	E027 ErrorCode = "E027" // dock.apps element empty
	E028 ErrorCode = "E028" // shell.default invalid value
	E029 ErrorCode = "E029" // shell.theme empty if present
	E030 ErrorCode = "E030" // shell.plugins element invalid
	E031 ErrorCode = "E031" // directories element empty
	E032 ErrorCode = "E032" // custom_steps name pattern mismatch
	E033 ErrorCode = "E033" // custom_steps provides element invalid
	E034 ErrorCode = "E034" // custom_steps requires element invalid
	E035 ErrorCode = "E035" // custom_steps platform element invalid
	E036 ErrorCode = "E036" // custom_steps apply key invalid
	E037 ErrorCode = "E037" // custom_steps apply command empty
	E038 ErrorCode = "E038" // custom_steps rollback key invalid
	E039 ErrorCode = "E039" // custom_steps rollback command empty
	E040 ErrorCode = "E040" // custom_steps env element invalid
	E041 ErrorCode = "E041" // custom_steps tags element invalid
	E042 ErrorCode = "E042" // type mismatch (expected X, got Y)
	E043 ErrorCode = "E043" // unknown field within section
)

// WarningCode identifies a specific validation warning type.
type WarningCode string

const (
	W001 WarningCode = "W001" // identity.git_email not an email
	W002 WarningCode = "W002" // directories duplicate entry
	W003 WarningCode = "W003" // type mismatch during merge
)

// ValidationError represents a single validation error with a code, field path, and message.
type ValidationError struct {
	Code    ErrorCode
	Field   string
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ValidationWarning represents a single validation warning with a code, field path, and message.
type ValidationWarning struct {
	Code    WarningCode
	Field   string
	Message string
}

// String returns a formatted warning string.
func (w ValidationWarning) String() string {
	return fmt.Sprintf("[%s] %s", w.Code, w.Message)
}

// newError creates a ValidationError with the given code, field, and message.
func newError(code ErrorCode, field, message string) ValidationError {
	return ValidationError{Code: code, Field: field, Message: message}
}

// newWarning creates a ValidationWarning with the given code, field, and message.
func newWarning(code WarningCode, field, message string) ValidationWarning {
	return ValidationWarning{Code: code, Field: field, Message: message}
}
