package sdk

import (
	"context"
	"fmt"
)

// BuildKnownHost represents an SSH known host entry for the build server.
type BuildKnownHost struct {
	Identifier string `json:"identifier"`
	Hostname   string `json:"hostname"`
	PublicKey  string `json:"public_key"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// BuildKnownHostCreateRequest is the payload for creating a build known host.
type BuildKnownHostCreateRequest struct {
	Hostname  string `json:"hostname"`
	PublicKey string `json:"public_key"`
}

func (c *Client) ListBuildKnownHosts(ctx context.Context, projectID string) ([]BuildKnownHost, error) {
	var hosts []BuildKnownHost
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/build_known_hosts", projectID), &hosts); err != nil {
		return nil, err
	}
	return hosts, nil
}

func (c *Client) CreateBuildKnownHost(ctx context.Context, projectID string, req BuildKnownHostCreateRequest) (*BuildKnownHost, error) {
	body := struct {
		BuildKnownHost BuildKnownHostCreateRequest `json:"build_known_host"`
	}{BuildKnownHost: req}
	var host BuildKnownHost
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/build_known_hosts", projectID), body, &host); err != nil {
		return nil, err
	}
	return &host, nil
}

func (c *Client) DeleteBuildKnownHost(ctx context.Context, projectID, hostID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/build_known_hosts/%s", projectID, hostID))
}
