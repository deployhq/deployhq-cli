package sdk

import (
	"context"
	"fmt"
)

// ConfigFile represents a project config file.
type ConfigFile struct {
	Identifier  string   `json:"identifier"`
	Description string   `json:"description"`
	Path        string   `json:"path"`
	Body        string   `json:"body"`
	Build       bool     `json:"build"`
	Servers     []interface{} `json:"servers,omitempty"`
}

// ConfigFileCreateRequest is the payload for creating/updating a config file.
type ConfigFileCreateRequest struct {
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
	Body        string `json:"body"`
	Build       *bool  `json:"build,omitempty"`
}

func (c *Client) ListConfigFiles(ctx context.Context, projectID string, opts *ListOptions) ([]ConfigFile, error) {
	var files []ConfigFile
	path := appendListParams(fmt.Sprintf("/projects/%s/config_files", projectID), opts)
	if err := c.get(ctx, path, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *Client) GetConfigFile(ctx context.Context, projectID, fileID string) (*ConfigFile, error) {
	var file ConfigFile
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/config_files/%s", projectID, fileID), &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) CreateConfigFile(ctx context.Context, projectID string, req ConfigFileCreateRequest) (*ConfigFile, error) {
	body := struct {
		ConfigFile ConfigFileCreateRequest `json:"config_file"`
	}{ConfigFile: req}
	var file ConfigFile
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/config_files", projectID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) UpdateConfigFile(ctx context.Context, projectID, fileID string, req ConfigFileCreateRequest) (*ConfigFile, error) {
	body := struct {
		ConfigFile ConfigFileCreateRequest `json:"config_file"`
	}{ConfigFile: req}
	var file ConfigFile
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/config_files/%s", projectID, fileID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) DeleteConfigFile(ctx context.Context, projectID, fileID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/config_files/%s", projectID, fileID))
}
