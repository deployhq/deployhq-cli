package sdk

import (
	"context"
	"fmt"
)

// Agent represents a network agent.
type Agent struct {
	ID         int     `json:"id"`
	CreatedAt  string  `json:"created_at"`
	Identifier string  `json:"identifier"`
	Name       string  `json:"name"`
	Online     bool    `json:"online"`
	RevokedAt  *string `json:"revoked_at,omitempty"`
	UpdatedAt  string  `json:"updated_at"`
}

// AgentCreateRequest is the payload for creating an agent.
type AgentCreateRequest struct {
	ClaimCode string `json:"claim_code"`
}

func (c *Client) ListAgents(ctx context.Context, opts *ListOptions) ([]Agent, error) {
	var agents []Agent
	path := appendListParams("/agents", opts)
	if err := c.get(ctx, path, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func (c *Client) CreateAgent(ctx context.Context, req AgentCreateRequest) (*Agent, error) {
	body := struct {
		Agent AgentCreateRequest `json:"agent"`
	}{Agent: req}
	var agent Agent
	if err := c.post(ctx, "/agents", body, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

func (c *Client) UpdateAgent(ctx context.Context, agentID string, name string) (*Agent, error) {
	body := struct {
		Agent map[string]string `json:"agent"`
	}{Agent: map[string]string{"name": name}}
	var agent Agent
	if err := c.put(ctx, fmt.Sprintf("/agents/%s", agentID), body, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

func (c *Client) DeleteAgent(ctx context.Context, agentID string) error {
	return c.delete(ctx, fmt.Sprintf("/agents/%s", agentID))
}

func (c *Client) RevokeAgent(ctx context.Context, agentID string) error {
	return c.post(ctx, fmt.Sprintf("/agents/%s/revoke", agentID), nil, nil)
}
