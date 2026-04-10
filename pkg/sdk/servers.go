package sdk

import (
	"context"
	"fmt"
)

// ListServers returns all servers for a project.
func (c *Client) ListServers(ctx context.Context, projectID string, opts *ListOptions) ([]Server, error) {
	var servers []Server
	path := appendListParams(fmt.Sprintf("/projects/%s/servers", projectID), opts)
	if err := c.get(ctx, path, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

// GetServer returns a single server by identifier.
func (c *Client) GetServer(ctx context.Context, projectID, serverID string) (*Server, error) {
	var server Server
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/servers/%s", projectID, serverID), &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// CreateServer creates a new server in a project.
func (c *Client) CreateServer(ctx context.Context, projectID string, req ServerCreateRequest) (*Server, error) {
	body := struct {
		Server ServerCreateRequest `json:"server"`
	}{Server: req}
	var server Server
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/servers", projectID), body, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// UpdateServer updates a server.
func (c *Client) UpdateServer(ctx context.Context, projectID, serverID string, req ServerUpdateRequest) (*Server, error) {
	body := struct {
		Server ServerUpdateRequest `json:"server"`
	}{Server: req}
	var server Server
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/servers/%s", projectID, serverID), body, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// DeleteServer deletes a server.
func (c *Client) DeleteServer(ctx context.Context, projectID, serverID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/servers/%s", projectID, serverID))
}

// ResetServerHostKey resets the SSH host key for a server.
func (c *Client) ResetServerHostKey(ctx context.Context, projectID, serverID string) error {
	return c.post(ctx, fmt.Sprintf("/projects/%s/servers/%s/reset_host_key", projectID, serverID), nil, nil)
}
