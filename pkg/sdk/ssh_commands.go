package sdk

import (
	"context"
	"fmt"
)

// SSHCommand represents an SSH command configured on a project.
type SSHCommand struct {
	Identifier  string   `json:"identifier"`
	CBack       string   `json:"cback,omitempty"`
	Position    int      `json:"position"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	HaltOnError bool     `json:"halt_on_error"`
	Servers     []interface{} `json:"servers,omitempty"`
	Timing      string   `json:"timing"`
	Timeout     int      `json:"timeout"`
	Enabled     bool     `json:"enabled"`
}

// SSHCommandCreateRequest is the payload for creating/updating an SSH command.
type SSHCommandCreateRequest struct {
	Description string `json:"description,omitempty"`
	Command     string `json:"command"`
	HaltOnError *bool  `json:"halt_on_error,omitempty"`
	Timing      string `json:"timing,omitempty"`
	Timeout     *int   `json:"timeout,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

func (c *Client) ListSSHCommands(ctx context.Context, projectID string, opts *ListOptions) ([]SSHCommand, error) {
	var cmds []SSHCommand
	path := appendListParams(fmt.Sprintf("/projects/%s/commands", projectID), opts)
	if err := c.get(ctx, path, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

func (c *Client) GetSSHCommand(ctx context.Context, projectID, cmdID string) (*SSHCommand, error) {
	var cmd SSHCommand
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/commands/%s", projectID, cmdID), &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) CreateSSHCommand(ctx context.Context, projectID string, req SSHCommandCreateRequest) (*SSHCommand, error) {
	body := struct {
		Command SSHCommandCreateRequest `json:"command"`
	}{Command: req}
	var cmd SSHCommand
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/commands", projectID), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) UpdateSSHCommand(ctx context.Context, projectID, cmdID string, req SSHCommandCreateRequest) (*SSHCommand, error) {
	body := struct {
		Command SSHCommandCreateRequest `json:"command"`
	}{Command: req}
	var cmd SSHCommand
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/commands/%s", projectID, cmdID), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) DeleteSSHCommand(ctx context.Context, projectID, cmdID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/commands/%s", projectID, cmdID))
}
