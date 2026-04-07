package output

import "strings"

// Breadcrumb represents a suggested next action.
type Breadcrumb struct {
	Action string `json:"action"`
	Cmd    string `json:"cmd"`
}

// Response is the JSON envelope with breadcrumbs (Basecamp pattern).
type Response struct {
	OK          bool         `json:"ok"`
	Data        interface{}  `json:"data"`
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

// ErrorResponse creates an error response envelope.
func ErrorResponse(code, message, suggestion, docURL string) *Response {
	return &Response{
		OK: false,
		Data: map[string]string{
			"error":      message,
			"code":       code,
			"suggestion": suggestion,
			"doc_url":    docURL,
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
	}

	// Enrich API errors with actionable suggestions when no hint is set
	if hint == "" {
		hint = apiErrorHint(message)
	}

	return ErrorResponse(code, message, hint, "")
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
