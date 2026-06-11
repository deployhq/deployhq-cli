package sdk

import "context"

// DetectionPayload is the uploaded manifest for POST /detection: a filename
// listing (for existence checks) plus the contents of a bounded set of
// manifest files (for content-based detection). Files the server doesn't
// receive simply lower precision; they never error.
type DetectionPayload struct {
	Filenames []string          `json:"filenames"`
	Files     map[string]string `json:"files,omitempty"`
}

// DetectionResponse is the backend's framework detection result — the same
// StackDetector pipeline the web onboarding wizard uses.
type DetectionResponse struct {
	Stack             string                  `json:"stack"`
	Version           string                  `json:"version"`
	Evidence          []string                `json:"evidence"`
	Description       string                  `json:"description"`
	SuggestedProtocol string                  `json:"suggested_protocol"`
	StaticHosting     DetectionStaticHosting  `json:"static_hosting"`
	BuildCommands     []DetectionBuildCommand `json:"build_commands"`
}

// DetectionStaticHosting holds the static-hosting assessment for the detected stack.
type DetectionStaticHosting struct {
	Eligibility string `json:"eligibility"`
	RootPath    string `json:"root_path"`
	SPAMode     bool   `json:"spa_mode"`
	Confidence  string `json:"confidence"`
}

// DetectionBuildCommand is a single suggested build step.
type DetectionBuildCommand struct {
	Description string `json:"description"`
	Command     string `json:"command"`
}

// DetectFramework asks the backend to detect the project's framework from an
// uploaded manifest. The result mirrors the web onboarding wizard's detection,
// keeping the CLI's recommendation in lockstep with the backend.
func (c *Client) DetectFramework(ctx context.Context, payload DetectionPayload) (*DetectionResponse, error) {
	body := struct {
		Detection DetectionPayload `json:"detection"`
	}{Detection: payload}
	var resp DetectionResponse
	if err := c.post(ctx, "/detection", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
