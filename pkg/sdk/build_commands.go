package sdk

import (
	"context"
	"fmt"
)

// BuildCommand represents a build command.
type BuildCommand struct {
	ID           int     `json:"id"`
	Identifier   string  `json:"identifier"`
	Description  string  `json:"description"`
	Command      string  `json:"command"`
	HaltOnError  *bool   `json:"halt_on_error,omitempty"`
	Position     int     `json:"position"`
	Enabled      bool    `json:"enabled"`
	TemplateName *string `json:"template_name,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func (b BuildCommand) NumericID() int { return b.ID }
func (b BuildCommand) UUID() string   { return b.Identifier }

// BuildCommandCreateRequest is the payload for creating/updating a build command.
type BuildCommandCreateRequest struct {
	Description string `json:"description,omitempty"`
	Command     string `json:"command"`
	HaltOnError *bool  `json:"halt_on_error,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

func (c *Client) ListBuildCommands(ctx context.Context, projectID string, opts *ListOptions) ([]BuildCommand, error) {
	var cmds []BuildCommand
	path := appendListParams(fmt.Sprintf("/projects/%s/build_commands", projectID), opts)
	if err := c.get(ctx, path, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

func (c *Client) CreateBuildCommand(ctx context.Context, projectID string, req BuildCommandCreateRequest) (*BuildCommand, error) {
	body := struct {
		BuildCommand BuildCommandCreateRequest `json:"build_command"`
	}{BuildCommand: req}
	var cmd BuildCommand
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/build_commands", projectID), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) UpdateBuildCommand(ctx context.Context, projectID, cmdID string, req BuildCommandCreateRequest) (*BuildCommand, error) {
	body := struct {
		BuildCommand BuildCommandCreateRequest `json:"build_command"`
	}{BuildCommand: req}
	var cmd BuildCommand
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/build_commands/%s", projectID, cmdID), body, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (c *Client) DeleteBuildCommand(ctx context.Context, projectID, cmdID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/build_commands/%s", projectID, cmdID))
}
