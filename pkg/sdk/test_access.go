package sdk

import (
	"context"
	"fmt"
)

// TestAccessRun represents a test access run.
type TestAccessRun struct {
	Identifier  string             `json:"identifier"`
	Status      string             `json:"status"`
	CreatedAt   string             `json:"created_at"`
	CompletedAt string             `json:"completed_at,omitempty"`
	Results     *TestAccessResults `json:"results,omitempty"`
}

// TestAccessResults contains the repository and server test outcomes.
type TestAccessResults struct {
	Repository *TestAccessResult       `json:"repository,omitempty"`
	Servers    []TestAccessServerResult `json:"servers,omitempty"`
}

// TestAccessResult is the outcome of a single connectivity test.
type TestAccessResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// TestAccessServerResult is the outcome of a server connectivity test.
type TestAccessServerResult struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

// RunTestAccess triggers a test access run for all servers in a project.
func (c *Client) RunTestAccess(ctx context.Context, projectID string) (*TestAccessRun, error) {
	var run TestAccessRun
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/test_access", projectID), nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// RunServerTestAccess triggers a test access run for a single server.
func (c *Client) RunServerTestAccess(ctx context.Context, projectID, serverID string) (*TestAccessRun, error) {
	var run TestAccessRun
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/servers/%s/test_access", projectID, serverID), nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// GetTestAccess returns the status and results of a test access run.
func (c *Client) GetTestAccess(ctx context.Context, projectID, runID string) (*TestAccessRun, error) {
	var run TestAccessRun
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/test_access/%s", projectID, runID), &run); err != nil {
		return nil, err
	}
	return &run, nil
}
