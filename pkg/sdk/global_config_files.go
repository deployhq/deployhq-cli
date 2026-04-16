package sdk

import (
	"context"
	"fmt"
)

// GlobalConfigFile represents an account-level config file.
type GlobalConfigFile struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// GlobalConfigFileCreateRequest is the payload for creating/updating a global config file.
type GlobalConfigFileCreateRequest struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func (c *Client) ListGlobalConfigFiles(ctx context.Context) ([]GlobalConfigFile, error) {
	var files []GlobalConfigFile
	if err := c.get(ctx, "/global_config_files", &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *Client) GetGlobalConfigFile(ctx context.Context, fileID string) (*GlobalConfigFile, error) {
	var file GlobalConfigFile
	if err := c.get(ctx, fmt.Sprintf("/global_config_files/%s", fileID), &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) CreateGlobalConfigFile(ctx context.Context, req GlobalConfigFileCreateRequest) (*GlobalConfigFile, error) {
	body := struct {
		GlobalConfigFile GlobalConfigFileCreateRequest `json:"global_config_file"`
	}{GlobalConfigFile: req}
	var file GlobalConfigFile
	if err := c.post(ctx, "/global_config_files", body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) UpdateGlobalConfigFile(ctx context.Context, fileID string, req GlobalConfigFileCreateRequest) (*GlobalConfigFile, error) {
	body := struct {
		GlobalConfigFile GlobalConfigFileCreateRequest `json:"global_config_file"`
	}{GlobalConfigFile: req}
	var file GlobalConfigFile
	if err := c.put(ctx, fmt.Sprintf("/global_config_files/%s", fileID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) DeleteGlobalConfigFile(ctx context.Context, fileID string) error {
	return c.delete(ctx, fmt.Sprintf("/global_config_files/%s", fileID))
}
