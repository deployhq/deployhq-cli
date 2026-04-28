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
	"net/url"
	"syscall"
)

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
	case *NetworkError:
		return ExitNetworkError
	}
	if IsNetworkErr(err) {
		return ExitNetworkError
	}
	return ExitInternalError
}
