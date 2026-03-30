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

func (c *Client) ListScheduledDeployments(ctx context.Context, projectID string) ([]ScheduledDeployment, error) {
	var result []ScheduledDeployment
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/scheduled_deployments", projectID), &result); err != nil {
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
