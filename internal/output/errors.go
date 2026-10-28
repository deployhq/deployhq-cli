// Package output provides the CLI output engine.
//
// Design principles (Wrangler pattern):
//   - stderr = human-readable messages (status, errors, prompts)
//   - stdout = machine-readable data (JSON or table)
//   - errors are classified for appropriate exit codes
package output

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// ExitCode constants for error classification.
const (
	ExitOK             = 0
	ExitUserError      = 1   // bad input, missing args, validation
	ExitInternalError  = 2   // unexpected failures, panics
	ExitAuthError      = 3   // authentication/authorization failures
	ExitNetworkError   = 4   // connectivity issues
	ExitNotFoundError  = 5   // resource not found
	ExitConflictError  = 6   // resource conflict (already exists, etc.)
	ExitInterruptError = 130 // SIGINT
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

// ReportedError wraps an error whose structured JSON error response has already
// been written to stdout. The top-level handler must not write a second one, so
// stdout carries exactly one JSON object. The exit code is that of the wrapped
// error.
type ReportedError struct {
	Err error
}

func (e *ReportedError) Error() string { return e.Err.Error() }
func (e *ReportedError) Unwrap() error { return e.Err }

// ExitCoder is implemented by an error that chooses its own exit code, such as
// a structured refusal whose kind (user error, auth, conflict) is decided where
// it is built rather than by its Go type.
type ExitCoder interface {
	ExitCode() int
}

// IsReported reports whether err's JSON error response was already written.
func IsReported(err error) bool {
	var reported *ReportedError
	return errors.As(err, &reported)
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

// NetworkError represents a connectivity failure (timeout, DNS, refused
// connection, unreachable host). These produce exit code 4 so wrappers can
// distinguish "the network is broken" from "the CLI itself is broken".
type NetworkError struct {
	Message string
	Cause   error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *NetworkError) Unwrap() error { return e.Cause }

// Email-verification refusal copy. The API rejects every request made with an
// account's API key until the account owner opens the verification link that
// was emailed at sign-up, so the fix is never "log in again".
const (
	EmailNotVerifiedMessage = "Your DeployHQ email address isn't verified yet, so DeployHQ is refusing API requests"
	EmailNotVerifiedHint    = "Open the verification link DeployHQ emailed you when the account was created (check your spam folder too), then retry. Run 'dhq auth status' to check."
)

// EmailNotVerifiedError is returned when the API refuses a request because the
// account's email address has not been verified yet. It exits with the auth
// exit code but, unlike AuthError, logging in again does not fix it.
type EmailNotVerifiedError struct {
	Cause error
}

func (e *EmailNotVerifiedError) Error() string {
	return fmt.Sprintf("%s\n\nHint: %s", EmailNotVerifiedMessage, EmailNotVerifiedHint)
}

func (e *EmailNotVerifiedError) Unwrap() error { return e.Cause }

// IsEmailVerificationRequired reports whether err, or any error it wraps, is the
// API's "email not verified" refusal (a 403 with error
// "email_verification_required"). A generic 403 returns false.
func IsEmailVerificationRequired(err error) bool {
	var apiErr *sdk.APIError
	return errors.As(err, &apiErr) && apiErr.IsEmailVerificationRequired()
}

// RefineError replaces API errors that have a specific, known cause with a typed
// error carrying an actionable message. Other errors are returned unchanged.
// It is idempotent.
func RefineError(err error) error {
	if err == nil {
		return nil
	}
	var already *EmailNotVerifiedError
	if errors.As(err, &already) {
		return err
	}
	if IsEmailVerificationRequired(err) {
		return &EmailNotVerifiedError{Cause: err}
	}
	return err
}

// IsNetworkErr reports whether err (or any error it wraps) is a connectivity
// failure originating from the network/transport layer. It returns false for
// successful HTTP exchanges that returned non-2xx status (those should be
// classified by status code, not bucketed as network).
func IsNetworkErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return IsNetworkErr(urlErr.Err)
	}
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	return false
}

// ClassifyError returns the appropriate exit code for an error.
//
// Precedence: an error implementing ExitCoder (anywhere in the chain) decides
// its own code. Otherwise typed errors (UserError, InternalError, etc.) the command code
// chose explicitly always win. Raw *sdk.APIError values returned without
// wrapping fall back to a status-code mapping so 4xx doesn't get reported as
// an internal CLI bug.
func ClassifyError(err error) int {
	if err == nil {
		return ExitOK
	}
	var reported *ReportedError
	if errors.As(err, &reported) {
		return ClassifyError(reported.Err)
	}
	var coder ExitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	err = RefineError(err)
	switch err.(type) {
	case *EmailNotVerifiedError:
		return ExitAuthError
	case *UserError:
		return ExitUserError
	case *InternalError:
		return ExitInternalError
	case *AuthError:
		return ExitAuthError
	case *NetworkError:
		return ExitNetworkError
	}
	var apiErr *sdk.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusUnauthorized,
			apiErr.StatusCode == http.StatusForbidden:
			return ExitAuthError
		case apiErr.StatusCode == http.StatusNotFound:
			return ExitNotFoundError
		case apiErr.StatusCode == http.StatusConflict:
			return ExitConflictError
		case apiErr.StatusCode >= 400 && apiErr.StatusCode < 500:
			return ExitUserError
		case apiErr.StatusCode >= 500:
			return ExitInternalError
		}
	}
	if IsNetworkErr(err) {
		return ExitNetworkError
	}
	return ExitInternalError
}
