package sdk

import (
	"context"
	"fmt"
)

// AutoDeployment represents a webhook-triggered auto deployment config.
type AutoDeployment struct {
	WebhookURL  string       `json:"webhook_url"`
	Deployables []Deployable `json:"deployables,omitempty"`
}

// Deployable is a server or group that can be auto-deployed to.
type Deployable struct {
	Identifier      string `json:"identifier"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	AutoDeploy      bool   `json:"auto_deploy"`
	PreferredBranch string `json:"preferred_branch"`
}

// AutoDeployCreateRequest is the payload for creating an auto deployment.
type AutoDeployCreateRequest struct {
	Deployables []DeployableToggle `json:"deployables"`
}

// DeployableToggle sets auto_deploy on a server/group.
type DeployableToggle struct {
	Identifier string `json:"identifier"`
	AutoDeploy bool   `json:"auto_deploy"`
}

func (c *Client) ListAutoDeployments(ctx context.Context, projectID string, opts *ListOptions) (*AutoDeployment, error) {
	var result AutoDeployment
	path := appendListParams(fmt.Sprintf("/projects/%s/auto_deployments", projectID), opts)
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateAutoDeployment(ctx context.Context, projectID string, req AutoDeployCreateRequest) (*AutoDeployment, error) {
	var result AutoDeployment
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/auto_deployments", projectID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
