package sdk

import (
	"context"
	"fmt"
)

// DeploymentCheck represents a deployment check configured on a project.
// A check has a stage (pre_build or post_deploy) and a check_type (ssh, http,
// or vulnerability_scan).
type DeploymentCheck struct {
	Identifier         string        `json:"identifier"`
	Name               string        `json:"name"`
	Description        string        `json:"description,omitempty"`
	Stage              string        `json:"stage"`
	CheckType          string        `json:"check_type"`
	Enabled            bool          `json:"enabled"`
	Position           int           `json:"position"`
	TimeoutSeconds     int           `json:"timeout_seconds,omitempty"`
	Command            string        `json:"command,omitempty"`
	Servers            []interface{} `json:"servers,omitempty"`
	HTTPMethod         string        `json:"http_method,omitempty"`
	HTTPURL            string        `json:"http_url,omitempty"`
	HTTPExpectedStatus int           `json:"http_expected_status,omitempty"`
	HTTPBodyMatch      string        `json:"http_body_match,omitempty"`
	Scanner            string        `json:"scanner,omitempty"`
	ScanTargetKind     string        `json:"scan_target_kind,omitempty"`
	ScanTarget         string        `json:"scan_target,omitempty"`
	SeverityThreshold  string        `json:"severity_threshold,omitempty"`
	FailOnUnfixedOnly  bool          `json:"fail_on_unfixed_only,omitempty"`
	SARIFOutputPath    string        `json:"sarif_output_path,omitempty"`
}

// DeploymentCheckCreateRequest is the payload for creating or updating a deployment check.
// Pointer fields are omitted from the JSON body when nil, so callers only send what they
// want to set (important for partial updates via PATCH).
type DeploymentCheckCreateRequest struct {
	Name               string   `json:"name,omitempty"`
	Description        string   `json:"description,omitempty"`
	Stage              string   `json:"stage,omitempty"`
	CheckType          string   `json:"check_type,omitempty"`
	Enabled            *bool    `json:"enabled,omitempty"`
	TimeoutSeconds     *int     `json:"timeout_seconds,omitempty"`
	Command            string   `json:"command,omitempty"`
	Servers            []string `json:"servers,omitempty"`
	HTTPMethod         string   `json:"http_method,omitempty"`
	HTTPURL            string   `json:"http_url,omitempty"`
	HTTPExpectedStatus *int     `json:"http_expected_status,omitempty"`
	HTTPBodyMatch      string   `json:"http_body_match,omitempty"`
	Scanner            string   `json:"scanner,omitempty"`
	ScanTargetKind     string   `json:"scan_target_kind,omitempty"`
	ScanTarget         string   `json:"scan_target,omitempty"`
	SeverityThreshold  string   `json:"severity_threshold,omitempty"`
	FailOnUnfixedOnly  *bool    `json:"fail_on_unfixed_only,omitempty"`
	SARIFOutputPath    string   `json:"sarif_output_path,omitempty"`
}

func (c *Client) ListDeploymentChecks(ctx context.Context, projectID string, opts *ListOptions) ([]DeploymentCheck, error) {
	var checks []DeploymentCheck
	path := appendListParams(fmt.Sprintf("/projects/%s/deployment_checks", projectID), opts)
	if err := c.get(ctx, path, &checks); err != nil {
		return nil, err
	}
	return checks, nil
}

func (c *Client) GetDeploymentCheck(ctx context.Context, projectID, checkID string) (*DeploymentCheck, error) {
	var check DeploymentCheck
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/deployment_checks/%s", projectID, checkID), &check); err != nil {
		return nil, err
	}
	return &check, nil
}

func (c *Client) CreateDeploymentCheck(ctx context.Context, projectID string, req DeploymentCheckCreateRequest) (*DeploymentCheck, error) {
	body := struct {
		DeploymentCheck DeploymentCheckCreateRequest `json:"deployment_check"`
	}{DeploymentCheck: req}
	var check DeploymentCheck
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/deployment_checks", projectID), body, &check); err != nil {
		return nil, err
	}
	return &check, nil
}

func (c *Client) UpdateDeploymentCheck(ctx context.Context, projectID, checkID string, req DeploymentCheckCreateRequest) (*DeploymentCheck, error) {
	body := struct {
		DeploymentCheck DeploymentCheckCreateRequest `json:"deployment_check"`
	}{DeploymentCheck: req}
	var check DeploymentCheck
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/deployment_checks/%s", projectID, checkID), body, &check); err != nil {
		return nil, err
	}
	return &check, nil
}

func (c *Client) DeleteDeploymentCheck(ctx context.Context, projectID, checkID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/deployment_checks/%s", projectID, checkID))
}
