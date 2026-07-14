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

// CreateServerFromGlobal creates a project server from a global server template.
// The request body is FLAT (top-level global_server_id), not wrapped.
func (c *Client) CreateServerFromGlobal(ctx context.Context, projectID, globalServerID string) (*Server, error) {
	body := struct {
		GlobalServerID string `json:"global_server_id"`
	}{GlobalServerID: globalServerID}
	var server Server
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/servers/from_global", projectID), body, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// ServerMetrics is a point-in-time snapshot of a server's resource usage.
// The cpu/memory/disk/profile blocks vary in shape, so they are modeled as
// free-form maps.
// ServerMetrics is a point-in-time snapshot. The backend deep-camelizes every
// key, and the shapes vary by field, so the nested values are kept as generic
// JSON: status/cpu/memory/profile/lastDeploy are objects, disk is an array of
// per-partition objects, and uptime is an object ({formatted, seconds}), NOT a
// string. Mirrors the nine keys the server_metrics endpoint returns.
type ServerMetrics struct {
	Status        map[string]interface{}   `json:"status"`
	Uptime        ServerUptime             `json:"uptime"`
	CPU           map[string]interface{}   `json:"cpu"`
	Memory        map[string]interface{}   `json:"memory"`
	Disk          []map[string]interface{} `json:"disk"`
	LastDeploy    map[string]interface{}   `json:"lastDeploy"`
	Profile       map[string]interface{}   `json:"profile"`
	SharedHosting bool                     `json:"sharedHosting"`
	Hostname      string                   `json:"hostname"`
}

// ServerUptime is the nested uptime object returned by server_metrics.
type ServerUptime struct {
	Formatted string `json:"formatted"`
	Seconds   int64  `json:"seconds"`
}

// GetServerMetrics returns a point-in-time metrics snapshot for a server.
// Only available for beta accounts on SSH servers; otherwise the API returns
// 403 Not available.
func (c *Client) GetServerMetrics(ctx context.Context, projectID, serverID string) (*ServerMetrics, error) {
	var metrics ServerMetrics
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/servers/%s/server_metrics", projectID, serverID), &metrics); err != nil {
		return nil, err
	}
	return &metrics, nil
}
