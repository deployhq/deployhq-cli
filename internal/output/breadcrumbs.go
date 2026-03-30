package output

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
