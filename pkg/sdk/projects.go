package sdk

import (
	"context"
	"fmt"
)

// ListProjects returns all projects for the account.
func (c *Client) ListProjects(ctx context.Context, opts *ListOptions) ([]Project, error) {
	var projects []Project
	path := appendListParams("/projects", opts)
	if err := c.get(ctx, path, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// GetProject returns a single project by permalink or identifier.
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var wrapper struct {
		Project
	}
	// The API returns the project directly (not wrapped) for show
	var project Project
	if err := c.get(ctx, fmt.Sprintf("/projects/%s", id), &project); err != nil {
		return nil, err
	}
	_ = wrapper // unused, API returns flat
	return &project, nil
}

// CreateProject creates a new project.
func (c *Client) CreateProject(ctx context.Context, req ProjectCreateRequest) (*Project, error) {
	body := struct {
		Project ProjectCreateRequest `json:"project"`
	}{Project: req}
	var project Project
	if err := c.post(ctx, "/projects", body, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// UpdateProject updates a project by permalink or identifier.
func (c *Client) UpdateProject(ctx context.Context, id string, req ProjectUpdateRequest) (*Project, error) {
	body := struct {
		Project ProjectUpdateRequest `json:"project"`
	}{Project: req}
	var project Project
	if err := c.put(ctx, fmt.Sprintf("/projects/%s", id), body, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// DeleteProject deletes a project by permalink or identifier.
func (c *Client) DeleteProject(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s", id))
}

// StarProject toggles the starred status of a project.
func (c *Client) StarProject(ctx context.Context, id string) error {
	return c.post(ctx, fmt.Sprintf("/projects/%s/star", id), nil, nil)
}

// GetProjectInsights returns deployment insights for a project.
func (c *Client) GetProjectInsights(ctx context.Context, id string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/insights", id), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UploadProjectKey uploads a custom public key for a project.
func (c *Client) UploadProjectKey(ctx context.Context, id, publicKey string) (*Project, error) {
	body := struct {
		Project struct {
			PublicKey string `json:"public_key"`
		} `json:"project"`
	}{}
	body.Project.PublicKey = publicKey
	var project Project
	if err := c.patch(ctx, fmt.Sprintf("/projects/%s/upload_key", id), body, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// GetStatusBadge returns the SVG deployment status badge for a project.
func (c *Client) GetStatusBadge(ctx context.Context, id string) ([]byte, error) {
	return c.doRaw(ctx, "GET", fmt.Sprintf("/%s/status_badge.svg", id))
}

// RegenerateProjectKey regenerates the project's SSH deploy key pair and
// returns the new public key. keyType is optional ("ED25519" or "RSA"); pass
// an empty string to let the server choose its default.
func (c *Client) RegenerateProjectKey(ctx context.Context, projectID, keyType string) (string, error) {
	inner := struct {
		KeyType string `json:"key_type,omitempty"`
	}{KeyType: keyType}
	body := struct {
		Project interface{} `json:"project"`
	}{Project: inner}
	var resp struct {
		PublicKey string `json:"public_key"`
	}
	if err := c.patch(ctx, fmt.Sprintf("/projects/%s/regenerate_key", projectID), body, &resp); err != nil {
		return "", err
	}
	return resp.PublicKey, nil
}

// UndeployedCommit is a single commit reported by GetUndeployedChanges.
type UndeployedCommit struct {
	Ref          string `json:"ref"`
	Author       string `json:"author"`
	Email        string `json:"email"`
	Timestamp    string `json:"timestamp"`
	Message      string `json:"message"`
	ShortMessage string `json:"short_message"`
	URL          string `json:"url"`
}

// UndeployedChanges describes commits that have not yet been deployed for a
// project. Nullable fields are modeled as pointers.
type UndeployedChanges struct {
	Count                  int                `json:"count"`
	Truncated              bool               `json:"truncated"`
	LatestDeployedRevision *string            `json:"latest_deployed_revision"`
	CurrentRevision        *string            `json:"current_revision"`
	LastCheckedAt          *string            `json:"last_checked_at"`
	CheckInProgress        bool               `json:"check_in_progress"`
	CheckFailMessage       *string            `json:"check_fail_message"`
	Commits                []UndeployedCommit `json:"commits"`
}

// GetUndeployedChanges returns the commits that have not yet been deployed for
// the project.
func (c *Client) GetUndeployedChanges(ctx context.Context, projectID string) (*UndeployedChanges, error) {
	var changes UndeployedChanges
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/undeployed_changes", projectID), &changes); err != nil {
		return nil, err
	}
	return &changes, nil
}

// GenerateAIDeploymentOverview asks DeployHQ to generate an AI-written summary
// of the changes between startRef and endRef, returning the overview text.
// endRef is required; startRef is optional (empty means "from the last
// deployed revision").
func (c *Client) GenerateAIDeploymentOverview(ctx context.Context, projectID, startRef, endRef string) (string, error) {
	if endRef == "" {
		return "", fmt.Errorf("deployhq: end_ref is required")
	}
	// Body is FLAT (top-level), not wrapped in a "project" key.
	body := struct {
		StartRef string `json:"start_ref,omitempty"`
		EndRef   string `json:"end_ref"`
	}{StartRef: startRef, EndRef: endRef}
	var resp struct {
		Success         bool   `json:"success"`
		Overview        string `json:"overview"`
		AIPromptVersion string `json:"ai_prompt_version"`
	}
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/ai_deployment_overview", projectID), body, &resp); err != nil {
		return "", err
	}
	return resp.Overview, nil
}
