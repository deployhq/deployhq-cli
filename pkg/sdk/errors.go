package sdk

import (
	"fmt"
	"net/http"
)

// APIError represents an error returned by the DeployHQ API.
type APIError struct {
	StatusCode int      `json:"status"`
	Message    string   `json:"error,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("deployhq api: %d %s", e.StatusCode, e.Message)
	}
	if len(e.Errors) == 1 {
		return fmt.Sprintf("deployhq api: %d %s", e.StatusCode, e.Errors[0])
	}
	if len(e.Errors) > 1 {
		msg := fmt.Sprintf("deployhq api: %d validation failed", e.StatusCode)
		for _, err := range e.Errors {
			msg += "\n  - " + err
		}
		return msg
	}
	return fmt.Sprintf("deployhq api: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401.
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsForbidden returns true if the error is a 403.
func (e *APIError) IsForbidden() bool {
	return e.StatusCode == http.StatusForbidden
}

// IsValidationError returns true if the error is a 422.
func (e *APIError) IsValidationError() bool {
	return e.StatusCode == http.StatusUnprocessableEntity
}

// IsServerError returns true if the error is a 5xx.
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= 500
}

// IsNotFound checks whether err is an APIError with status 404.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsNotFound()
	}
	return false
}

// IsUnauthorized checks whether err is an APIError with status 401.
func IsUnauthorized(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.IsUnauthorized()
	}
	return false
}
