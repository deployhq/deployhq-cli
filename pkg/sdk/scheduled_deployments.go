package sdk

import (
	"context"
	"fmt"
)

// ScheduledDeployment represents a scheduled deployment.
type ScheduledDeployment struct {
	Identifier       string `json:"identifier"`
	Server           string `json:"server"`
	StartRevision    string `json:"start_revision"`
	EndRevision      string `json:"end_revision"`
	Frequency        string `json:"frequency"`
	At               string `json:"at"`
	NextDeploymentAt string `json:"next_deployment_at"`
	CopyConfigFiles  bool   `json:"copy_config_files"`
	RunBuildCommands bool   `json:"run_build_commands"`
	UseBuildCache    bool   `json:"use_build_cache"`
}

func (c *Client) ListScheduledDeployments(ctx context.Context, projectID string, opts *ListOptions) ([]ScheduledDeployment, error) {
	var result []ScheduledDeployment
	path := appendListParams(fmt.Sprintf("/projects/%s/scheduled_deployments", projectID), opts)
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetScheduledDeployment(ctx context.Context, projectID, id string) (*ScheduledDeployment, error) {
	var result ScheduledDeployment
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/scheduled_deployments/%s", projectID, id), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteScheduledDeployment(ctx context.Context, projectID, id string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/scheduled_deployments/%s", projectID, id))
}

// ScheduledDeploymentUpdateRequest is the payload for updating a scheduled deployment.
type ScheduledDeploymentUpdateRequest struct {
	ServerIdentifier string `json:"server_identifier,omitempty"`
	StartRevision    string `json:"start_revision,omitempty"`
	EndRevision      string `json:"end_revision,omitempty"`
	Frequency        string `json:"frequency,omitempty"`
	At               string `json:"at,omitempty"`
	CopyConfigFiles  *bool  `json:"copy_config_files,omitempty"`
	RunBuildCommands *bool  `json:"run_build_commands,omitempty"`
	UseBuildCache    *bool  `json:"use_build_cache,omitempty"`
}

func (c *Client) UpdateScheduledDeployment(ctx context.Context, projectID, id string, req ScheduledDeploymentUpdateRequest) (*ScheduledDeployment, error) {
	body := struct {
		ScheduledDeployment ScheduledDeploymentUpdateRequest `json:"scheduled_deployment"`
	}{ScheduledDeployment: req}
	var result ScheduledDeployment
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/scheduled_deployments/%s", projectID, id), body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ScheduledDeploymentCreateRequest is the payload for creating a scheduled deployment.
type ScheduledDeploymentCreateRequest struct {
	ServerIdentifier string `json:"server_identifier"`
	StartRevision    string `json:"start_revision,omitempty"`
	EndRevision      string `json:"end_revision,omitempty"`
	Frequency        string `json:"frequency"`
	At               string `json:"at"`
	CopyConfigFiles  bool   `json:"copy_config_files"`
	RunBuildCommands bool   `json:"run_build_commands"`
	UseBuildCache    bool   `json:"use_build_cache"`
}

func (c *Client) CreateScheduledDeployment(ctx context.Context, projectID string, req ScheduledDeploymentCreateRequest) (*ScheduledDeployment, error) {
	body := struct {
		ScheduledDeployment ScheduledDeploymentCreateRequest `json:"scheduled_deployment"`
	}{ScheduledDeployment: req}
	var result ScheduledDeployment
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/scheduled_deployments", projectID), body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
