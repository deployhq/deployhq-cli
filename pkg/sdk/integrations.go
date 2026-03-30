package sdk

import (
	"context"
	"fmt"
)

// Integration represents a webhook/notification integration.
type Integration struct {
	Identifier       string `json:"identifier"`
	HookType         string `json:"hook_type"`
	Name             string `json:"name"`
	SendOnStart      bool   `json:"send_on_start"`
	SendOnCompletion bool   `json:"send_on_completion"`
	SendOnFailure    bool   `json:"send_on_failure"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// IntegrationCreateRequest is the payload for creating/updating an integration.
type IntegrationCreateRequest struct {
	HookType         string `json:"hook_type"`
	Name             string `json:"name,omitempty"`
	SendOnStart      *bool  `json:"send_on_start,omitempty"`
	SendOnCompletion *bool  `json:"send_on_completion,omitempty"`
	SendOnFailure    *bool  `json:"send_on_failure,omitempty"`
}

func (c *Client) ListIntegrations(ctx context.Context, projectID string) ([]Integration, error) {
	var integrations []Integration
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/integrations", projectID), &integrations); err != nil {
		return nil, err
	}
	return integrations, nil
}

func (c *Client) GetIntegration(ctx context.Context, projectID, integrationID string) (*Integration, error) {
	var integration Integration
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/integrations/%s", projectID, integrationID), &integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

func (c *Client) CreateIntegration(ctx context.Context, projectID string, req IntegrationCreateRequest) (*Integration, error) {
	body := struct {
		Integration IntegrationCreateRequest `json:"integration"`
	}{Integration: req}
	var integration Integration
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/integrations", projectID), body, &integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

func (c *Client) UpdateIntegration(ctx context.Context, projectID, integrationID string, req IntegrationCreateRequest) (*Integration, error) {
	body := struct {
		Integration IntegrationCreateRequest `json:"integration"`
	}{Integration: req}
	var integration Integration
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/integrations/%s", projectID, integrationID), body, &integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

func (c *Client) DeleteIntegration(ctx context.Context, projectID, integrationID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/integrations/%s", projectID, integrationID))
}
