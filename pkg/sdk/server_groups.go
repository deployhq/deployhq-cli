package sdk

import (
	"context"
	"fmt"
)

// ListServerGroups returns all server groups for a project.
func (c *Client) ListServerGroups(ctx context.Context, projectID string) ([]ServerGroup, error) {
	var groups []ServerGroup
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/server_groups", projectID), &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// GetServerGroup returns a single server group by identifier.
func (c *Client) GetServerGroup(ctx context.Context, projectID, groupID string) (*ServerGroup, error) {
	var group ServerGroup
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/server_groups/%s", projectID, groupID), &group); err != nil {
		return nil, err
	}
	return &group, nil
}

// CreateServerGroup creates a new server group in a project.
func (c *Client) CreateServerGroup(ctx context.Context, projectID string, req ServerGroupCreateRequest) (*ServerGroup, error) {
	body := struct {
		ServerGroup ServerGroupCreateRequest `json:"server_group"`
	}{ServerGroup: req}
	var group ServerGroup
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/server_groups", projectID), body, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

// UpdateServerGroup updates a server group.
func (c *Client) UpdateServerGroup(ctx context.Context, projectID, groupID string, req ServerGroupUpdateRequest) (*ServerGroup, error) {
	body := struct {
		ServerGroup ServerGroupUpdateRequest `json:"server_group"`
	}{ServerGroup: req}
	var group ServerGroup
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/server_groups/%s", projectID, groupID), body, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

// DeleteServerGroup deletes a server group.
func (c *Client) DeleteServerGroup(ctx context.Context, projectID, groupID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/server_groups/%s", projectID, groupID))
}
