package sdk

import (
	"context"
	"fmt"
	"net/http"
)

// ConfigFile represents a project config file.
type ConfigFile struct {
	Identifier  string        `json:"identifier"`
	Description string        `json:"description"`
	Path        string        `json:"path"`
	Body        string        `json:"body"`
	Build       bool          `json:"build"`
	Servers     []interface{} `json:"servers,omitempty"`
}

// ConfigFileCreateRequest is the payload for creating a config file.
type ConfigFileCreateRequest struct {
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
	Body        string `json:"body"`
	Build       *bool  `json:"build,omitempty"`
}

// ConfigFileUpdateRequest is the payload for updating a config file. All fields
// are pointers so only the flags the caller actually changed are sent; an
// omitted path/body is left untouched server-side rather than cleared.
type ConfigFileUpdateRequest struct {
	Description *string `json:"description,omitempty"`
	Path        *string `json:"path,omitempty"`
	Body        *string `json:"body,omitempty"`
	Build       *bool   `json:"build,omitempty"`
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

func (c *Client) UpdateConfigFile(ctx context.Context, projectID, fileID string, req ConfigFileUpdateRequest) (*ConfigFile, error) {
	body := struct {
		ConfigFile ConfigFileUpdateRequest `json:"config_file"`
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

// LinkGlobalConfigFile links an account-wide global config file into a project.
// The request body is FLAT (top-level config_file_id), not wrapped.
func (c *Client) LinkGlobalConfigFile(ctx context.Context, projectID, configFileID string) (*ConfigFile, error) {
	body := struct {
		ConfigFileID string `json:"config_file_id"`
	}{ConfigFileID: configFileID}
	var file ConfigFile
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/config_files/link_global", projectID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// UnlinkGlobalConfigFile removes the link between a project and a global config
// file. This is a DELETE request that carries a FLAT body.
func (c *Client) UnlinkGlobalConfigFile(ctx context.Context, projectID, configFileID string) error {
	body := struct {
		ConfigFileID string `json:"config_file_id"`
	}{ConfigFileID: configFileID}
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/projects/%s/config_files/unlink_global", projectID), body, nil)
}
