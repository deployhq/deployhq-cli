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
	// The managed-resource provisioning params are top-level siblings of `server`
	// in the request body — the backend reads params[:region], params[:os_image],
	// params[:hosted_website_attributes], etc. (NOT params[:server][:region]).
	// They are tagged json:"-" on ServerCreateRequest so they don't leak into the
	// nested server object; hoist them here.
	body := map[string]any{"server": req}
	if req.HostedWebsiteAttributes != nil {
		body["hosted_website_attributes"] = req.HostedWebsiteAttributes
	}
	if req.Region != "" {
		body["region"] = req.Region
	}
	if req.Size != "" {
		body["size"] = req.Size
	}
	if req.OSImage != "" {
		body["os_image"] = req.OSImage
	}
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
