package sdk

import (
	"context"
	"fmt"
)

// GlobalServer is the same as Server but at account level.
// Reuses the Server type.

func (c *Client) ListGlobalServers(ctx context.Context) ([]Server, error) {
	var servers []Server
	if err := c.get(ctx, "/global_servers", &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func (c *Client) GetGlobalServer(ctx context.Context, serverID string) (*Server, error) {
	var server Server
	if err := c.get(ctx, fmt.Sprintf("/global_servers/%s", serverID), &server); err != nil {
		return nil, err
	}
	return &server, nil
}

func (c *Client) CreateGlobalServer(ctx context.Context, req ServerCreateRequest) (*Server, error) {
	body := struct {
		Server ServerCreateRequest `json:"server"`
	}{Server: req}
	var server Server
	if err := c.post(ctx, "/global_servers", body, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

func (c *Client) UpdateGlobalServer(ctx context.Context, serverID string, req ServerUpdateRequest) (*Server, error) {
	body := struct {
		Server ServerUpdateRequest `json:"server"`
	}{Server: req}
	var server Server
	if err := c.put(ctx, fmt.Sprintf("/global_servers/%s", serverID), body, &server); err != nil {
		return nil, err
	}
	return &server, nil
}

func (c *Client) DeleteGlobalServer(ctx context.Context, serverID string) error {
	return c.delete(ctx, fmt.Sprintf("/global_servers/%s", serverID))
}

func (c *Client) CopyGlobalServerToProject(ctx context.Context, serverID string, projectID string) error {
	body := map[string]string{"project_id": projectID}
	return c.post(ctx, fmt.Sprintf("/global_servers/%s/copy_to_project", serverID), body, nil)
}
