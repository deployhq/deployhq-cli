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
	// ExcludedFiles and BuildCacheFiles are the suggested deploy-exclude and
	// build-cache patterns for the detected stack — the same source the web
	// onboarding wizard uses. Optional/additive; empty on older backends.
	ExcludedFiles   []DetectionFile `json:"excluded_files,omitempty"`
	BuildCacheFiles []DetectionFile `json:"build_cache_files,omitempty"`
	// AIAssisted is true when the backend's AI services contributed to the
	// result (only when the account has AI features enabled and the rule-based
	// result was ambiguous). Optional/additive; absent on older backends.
	AIAssisted bool `json:"ai_assisted,omitempty"`
}

// DetectionFile is a single suggested file path — an excluded-file pattern or a
// build-cache entry.
type DetectionFile struct {
	Path string `json:"path"`
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
