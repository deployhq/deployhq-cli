package sdk

import (
	"context"
	"fmt"
)

// BuildConfig represents a build configuration/environment.
type BuildConfig struct {
	Identifier string                 `json:"identifier"`
	Packages   map[string]string      `json:"packages,omitempty"`
	Default    bool                   `json:"default"`
	Servers    []interface{}          `json:"servers,omitempty"`
}

// BuildConfigCreateRequest is the payload for creating/updating a build config.
type BuildConfigCreateRequest struct {
	Packages map[string]string `json:"packages,omitempty"`
}

func (c *Client) ListBuildConfigs(ctx context.Context, projectID string) ([]BuildConfig, error) {
	var configs []BuildConfig
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/build_configurations", projectID), &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

func (c *Client) GetBuildConfig(ctx context.Context, projectID, configID string) (*BuildConfig, error) {
	var config BuildConfig
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/build_configurations/%s", projectID, configID), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) GetDefaultBuildConfig(ctx context.Context, projectID string) (*BuildConfig, error) {
	var config BuildConfig
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/build_configuration", projectID), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) CreateBuildConfig(ctx context.Context, projectID string, req BuildConfigCreateRequest) (*BuildConfig, error) {
	body := struct {
		BuildEnvironment BuildConfigCreateRequest `json:"build_environment"`
	}{BuildEnvironment: req}
	var config BuildConfig
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/build_configurations", projectID), body, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) UpdateBuildConfig(ctx context.Context, projectID, configID string, req BuildConfigCreateRequest) (*BuildConfig, error) {
	body := struct {
		BuildEnvironment BuildConfigCreateRequest `json:"build_environment"`
	}{BuildEnvironment: req}
	var config BuildConfig
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/build_configurations/%s", projectID, configID), body, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) DeleteBuildConfig(ctx context.Context, projectID, configID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/build_configurations/%s", projectID, configID))
}
