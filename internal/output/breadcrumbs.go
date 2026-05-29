package output

import (
	"errors"
	"net/http"
	"strings"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// Breadcrumb represents a suggested next action.
type Breadcrumb struct {
	Action   string `json:"action"`
	Cmd      string `json:"cmd"`
	Resource string `json:"resource,omitempty"` // resource type (deployment, server, project)
	ID       string `json:"id,omitempty"`       // resource identifier
}

// Pagination metadata for JSON output.
type Pagination struct {
	CurrentPage  int `json:"current_page"`
	TotalPages   int `json:"total_pages"`
	TotalRecords int `json:"total_records"`
	Offset       int `json:"offset"`
}

// Response is the JSON envelope with breadcrumbs (Basecamp pattern).
type Response struct {
	OK          bool         `json:"ok"`
	Data        interface{}  `json:"data"`
	Pagination  *Pagination  `json:"pagination,omitempty"`
	Summary     string       `json:"summary,omitempty"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs,omitempty"`
}

// NewResponse creates a success response with optional breadcrumbs.
func NewResponse(data interface{}, summary string, breadcrumbs ...Breadcrumb) *Response {
	return &Response{
		OK:          true,
		Data:        data,
		Summary:     summary,
		Breadcrumbs: breadcrumbs,
	}
}

// NewPaginatedResponse creates a success response with pagination metadata and optional breadcrumbs.
func NewPaginatedResponse(data interface{}, pagination Pagination, summary string, breadcrumbs ...Breadcrumb) *Response {
	p := pagination
	return &Response{
		OK:          true,
		Data:        data,
		Pagination:  &p,
		Summary:     summary,
		Breadcrumbs: breadcrumbs,
	}
}

// ErrorData is the structured error payload inside a Response.
type ErrorData struct {
	Error      string       `json:"error"`
	Code       string       `json:"code"`        // user_error, auth_error, internal_error, not_found, conflict, network_error
	ExitCode   int          `json:"exit_code"`    // numeric exit code (1-6)
	Retryable  bool         `json:"retryable"`    // safe to retry the same command
	Suggestion string       `json:"suggestion,omitempty"`
	DocURL     string       `json:"doc_url,omitempty"`
	Recovery   []Breadcrumb `json:"recovery,omitempty"` // suggested recovery actions
}

// ErrorResponse creates an error response envelope.
func ErrorResponse(code, message, suggestion, docURL string) *Response {
	return &Response{
		OK: false,
		Data: ErrorData{
			Error:      message,
			Code:       code,
			ExitCode:   exitCodeForErrorCode(code),
			Retryable:  isRetryable(code, message),
			Suggestion: suggestion,
			DocURL:     docURL,
		},
	}
}

// ErrorResponseFromErr creates a structured error response from a Go error.
func ErrorResponseFromErr(err error) *Response {
	if err == nil {
		return &Response{OK: true}
	}

	code := "error"
	message := err.Error()
	hint := ""
	exitCode := ClassifyError(err)

	switch e := err.(type) {
	case *UserError:
		code = "user_error"
		message = e.Message
		hint = e.Hint
	case *AuthError:
		code = "auth_error"
		message = e.Message
		hint = e.Hint
	case *InternalError:
		code = "internal_error"
		message = e.Message
	case *NetworkError:
		code = "network_error"
		message = e.Message
	}

	// Mirror the status-code mapping from ClassifyError so a raw *sdk.APIError
	// returned without wrapping produces a `code` field consistent with the
	// exit_code agents already follow.
	if code == "error" {
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) {
			switch {
			case apiErr.StatusCode == http.StatusUnauthorized,
				apiErr.StatusCode == http.StatusForbidden:
				code = "auth_error"
			case apiErr.StatusCode == http.StatusNotFound:
				code = "not_found"
			case apiErr.StatusCode == http.StatusConflict:
				code = "conflict"
			case apiErr.StatusCode >= 400 && apiErr.StatusCode < 500:
				code = "user_error"
			case apiErr.StatusCode >= 500:
				code = "internal_error"
			}
		}
	}

	// Enrich errors with actionable suggestions when no hint is set
	if hint == "" {
		hint = apiErrorHint(message)
	}
	if hint == "" {
		hint = cobraArgHint(message)
	}

	return &Response{
		OK: false,
		Data: ErrorData{
			Error:      message,
			Code:       code,
			ExitCode:   exitCode,
			Retryable:  isRetryable(code, message),
			Suggestion: hint,
			Recovery:   recoveryActions(code, message),
		},
	}
}

// exitCodeForErrorCode maps error code strings to numeric exit codes.
func exitCodeForErrorCode(code string) int {
	switch code {
	case "user_error":
		return ExitUserError
	case "auth_error":
		return ExitAuthError
	case "internal_error":
		return ExitInternalError
	case "not_found":
		return ExitNotFoundError
	case "conflict":
		return ExitConflictError
	case "network_error":
		return ExitNetworkError
	default:
		return ExitInternalError
	}
}

// isRetryable determines if an error is safe to retry.
func isRetryable(code, message string) bool {
	msg := strings.ToLower(message)
	if strings.Contains(msg, "rate_limit") || strings.Contains(msg, "timeout") ||
		code == "network_error" {
		return true
	}
	return false
}

// recoveryActions returns suggested next commands for common errors.
func recoveryActions(code, message string) []Breadcrumb {
	msg := strings.ToLower(message)
	switch {
	case code == "auth_error":
		return []Breadcrumb{
			{Action: "login", Cmd: "dhq auth login"},
			{Action: "check_status", Cmd: "dhq auth status"},
		}
	case strings.Contains(msg, "not logged in") || strings.Contains(msg, "not authenticated"):
		return []Breadcrumb{
			{Action: "login", Cmd: "dhq auth login"},
		}
	case strings.Contains(msg, "no project"):
		return []Breadcrumb{
			{Action: "list_projects", Cmd: "dhq projects list --json"},
			{Action: "set_project", Cmd: "dhq config set project <permalink>"},
		}
	case strings.Contains(msg, "not found"):
		return []Breadcrumb{
			{Action: "list_resources", Cmd: "dhq projects list --json"},
		}
	}
	return nil
}

// apiErrorHint returns a suggestion for known API error patterns.
func apiErrorHint(message string) string {
	msg := strings.ToLower(message)
	switch {
	case strings.Contains(msg, "project_limit_reached"):
		return "Your plan's project limit has been reached. Upgrade at Account Settings > Plan or delete unused projects with 'dhq projects delete <permalink>'"
	case strings.Contains(msg, "server_limit_reached"):
		return "Your plan's server limit has been reached. Upgrade at Account Settings > Plan or delete unused servers with 'dhq servers delete <project> <id>'"
	case strings.Contains(msg, "not_found"):
		return "The resource was not found. Check the identifier and try 'dhq projects list' or 'dhq servers list' to see available resources"
	case strings.Contains(msg, "rate_limit"):
		return "API rate limit exceeded. Wait a moment and retry"
	}
	return ""
}

// cobraArgHint detects cobra argument-count errors often caused by --json
// field selection without using = syntax (e.g. --json status,steps instead
// of --json=status,steps).
func cobraArgHint(message string) string {
	if strings.Contains(message, "arg(s), received") {
		return "If you used --json with field names, use --json=field1,field2 (with '=') so the fields aren't treated as extra arguments"
	}
	return ""
}
