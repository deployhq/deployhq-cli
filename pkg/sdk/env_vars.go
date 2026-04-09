package sdk

import (
	"context"
	"fmt"
)

// EnvVar represents an environment variable.
type EnvVar struct {
	Identifier    int    `json:"identifier"`
	Name          string `json:"name"`
	MaskedValue   string `json:"masked_value"`
	Locked        bool   `json:"locked"`
	BuildPipeline bool   `json:"build_pipeline"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// EnvVarCreateRequest is the payload for creating/updating an env var.
type EnvVarCreateRequest struct {
	Name          string `json:"name"`
	Value         string `json:"value"`
	Locked        *bool  `json:"locked,omitempty"`
	BuildPipeline *bool  `json:"build_pipeline,omitempty"`
}

// Project-scoped env vars

func (c *Client) ListEnvVars(ctx context.Context, projectID string, opts *ListOptions) ([]EnvVar, error) {
	var vars []EnvVar
	path := appendListParams(fmt.Sprintf("/projects/%s/environment_variables", projectID), opts)
	if err := c.get(ctx, path, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

func (c *Client) GetEnvVar(ctx context.Context, projectID, varID string) (*EnvVar, error) {
	var v EnvVar
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/environment_variables/%s", projectID, varID), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *Client) CreateEnvVar(ctx context.Context, projectID string, req EnvVarCreateRequest) (*EnvVar, error) {
	body := struct {
		EnvVar EnvVarCreateRequest `json:"environment_variable"`
	}{EnvVar: req}
	var v EnvVar
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/environment_variables", projectID), body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *Client) UpdateEnvVar(ctx context.Context, projectID, varID string, req EnvVarCreateRequest) (*EnvVar, error) {
	body := struct {
		EnvVar EnvVarCreateRequest `json:"environment_variable"`
	}{EnvVar: req}
	var v EnvVar
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/environment_variables/%s", projectID, varID), body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *Client) DeleteEnvVar(ctx context.Context, projectID, varID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/environment_variables/%s", projectID, varID))
}

// Global env vars

func (c *Client) ListGlobalEnvVars(ctx context.Context, opts *ListOptions) ([]EnvVar, error) {
	var vars []EnvVar
	path := appendListParams("/global_environment_variables", opts)
	if err := c.get(ctx, path, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

func (c *Client) GetGlobalEnvVar(ctx context.Context, varID string) (*EnvVar, error) {
	var v EnvVar
	if err := c.get(ctx, fmt.Sprintf("/global_environment_variables/%s", varID), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *Client) CreateGlobalEnvVar(ctx context.Context, req EnvVarCreateRequest) (*EnvVar, error) {
	body := struct {
		EnvVar EnvVarCreateRequest `json:"environment_variable"`
	}{EnvVar: req}
	var v EnvVar
	if err := c.post(ctx, "/global_environment_variables", body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *Client) UpdateGlobalEnvVar(ctx context.Context, varID string, req EnvVarCreateRequest) (*EnvVar, error) {
	body := struct {
		EnvVar EnvVarCreateRequest `json:"environment_variable"`
	}{EnvVar: req}
	var v EnvVar
	if err := c.put(ctx, fmt.Sprintf("/global_environment_variables/%s", varID), body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *Client) DeleteGlobalEnvVar(ctx context.Context, varID string) error {
	return c.delete(ctx, fmt.Sprintf("/global_environment_variables/%s", varID))
}
