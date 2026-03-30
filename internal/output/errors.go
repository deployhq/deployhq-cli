// Package output provides the CLI output engine.
//
// Design principles (Wrangler pattern):
//   - stderr = human-readable messages (status, errors, prompts)
//   - stdout = machine-readable data (JSON or table)
//   - errors are classified for appropriate exit codes
package output

import "fmt"

// ExitCode constants for error classification.
const (
	ExitOK              = 0
	ExitUserError       = 1 // bad input, missing args, validation
	ExitInternalError   = 2 // unexpected failures, panics
	ExitAuthError       = 3 // authentication/authorization failures
	ExitNetworkError    = 4 // connectivity issues
	ExitNotFoundError   = 5 // resource not found
	ExitConflictError   = 6 // resource conflict (already exists, etc.)
	ExitInterruptError  = 130 // SIGINT
)

// UserError represents a user-caused error (bad input, missing config, etc.).
// These produce helpful messages and exit code 1.
type UserError struct {
	Message string
	Hint    string // optional suggestion for fixing the error
}

func (e *UserError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n\nHint: %s", e.Message, e.Hint)
	}
	return e.Message
}

// InternalError represents an unexpected failure.
// These produce a generic message and point to the debug log.
type InternalError struct {
	Message string
	Cause   error
}

func (e *InternalError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *InternalError) Unwrap() error { return e.Cause }

// AuthError represents an authentication/authorization failure.
type AuthError struct {
	Message string
	Hint    string
}

func (e *AuthError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n\nHint: %s", e.Message, e.Hint)
	}
	return e.Message
}

// ClassifyError returns the appropriate exit code for an error.
func ClassifyError(err error) int {
	if err == nil {
		return ExitOK
	}
	switch err.(type) {
	case *UserError:
		return ExitUserError
	case *InternalError:
		return ExitInternalError
	case *AuthError:
		return ExitAuthError
	default:
		return ExitInternalError
	}
}
