package sdk

import (
	"context"
	"fmt"
)

// ListProjects returns all projects for the account.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	if err := c.get(ctx, "/projects", &projects); err != nil {
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
