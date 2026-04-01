package sdk

import (
	"context"
	"fmt"
)

// ListDeployments returns deployments for a project.
// The API returns a paginated response with {pagination, records}.
func (c *Client) ListDeployments(ctx context.Context, projectID string) (*PaginatedResponse[Deployment], error) {
	var result PaginatedResponse[Deployment]
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/deployments", projectID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDeployment returns a single deployment by identifier.
func (c *Client) GetDeployment(ctx context.Context, projectID, deploymentID string) (*Deployment, error) {
	var deployment Deployment
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/deployments/%s", projectID, deploymentID), &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// CreateDeployment creates a new deployment.
func (c *Client) CreateDeployment(ctx context.Context, projectID string, req DeploymentCreateRequest) (*Deployment, error) {
	body := struct {
		Deployment DeploymentCreateRequest `json:"deployment"`
	}{Deployment: req}
	var deployment Deployment
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/deployments", projectID), body, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// AbortDeployment aborts a running deployment.
func (c *Client) AbortDeployment(ctx context.Context, projectID, deploymentID string) error {
	return c.post(ctx, fmt.Sprintf("/projects/%s/deployments/%s/abort", projectID, deploymentID), nil, nil)
}

// RollbackDeployment creates a rollback deployment.
func (c *Client) RollbackDeployment(ctx context.Context, projectID, deploymentID string) (*Deployment, error) {
	var deployment Deployment
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/deployments/%s/rollback", projectID, deploymentID), nil, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// RetryDeployment retries a failed or completed deployment.
func (c *Client) RetryDeployment(ctx context.Context, projectID, deploymentID string) (*Deployment, error) {
	var deployment Deployment
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/deployments/%s/retry", projectID, deploymentID), nil, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// GetDeploymentStepLogs returns logs for a specific deployment step.
func (c *Client) GetDeploymentStepLogs(ctx context.Context, projectID, deploymentID, stepID string) ([]DeploymentStepLog, error) {
	var logs []DeploymentStepLog
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/deployments/%s/steps/%s/logs", projectID, deploymentID, stepID), &logs); err != nil {
		return nil, err
	}
	return logs, nil
}
