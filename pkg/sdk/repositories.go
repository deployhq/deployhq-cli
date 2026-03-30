package sdk

import (
	"context"
	"fmt"
)

// GetRepository returns the repository configuration for a project.
func (c *Client) GetRepository(ctx context.Context, projectID string) (*Repository, error) {
	var repo Repository
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/repository", projectID), &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// CreateRepository creates the repository for a project.
func (c *Client) CreateRepository(ctx context.Context, projectID string, req RepositoryCreateRequest) (*Repository, error) {
	body := struct {
		Repository RepositoryCreateRequest `json:"repository"`
	}{Repository: req}
	var repo Repository
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/repository", projectID), body, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// UpdateRepository updates the repository for a project.
func (c *Client) UpdateRepository(ctx context.Context, projectID string, req RepositoryCreateRequest) (*Repository, error) {
	body := struct {
		Repository RepositoryCreateRequest `json:"repository"`
	}{Repository: req}
	var repo Repository
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/repository", projectID), body, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// ListBranches returns the branches for a project's repository.
// The API returns a map of branch_name -> latest_commit_sha.
func (c *Client) ListBranches(ctx context.Context, projectID string) (map[string]string, error) {
	var branches map[string]string
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/repository/branches", projectID), &branches); err != nil {
		return nil, err
	}
	return branches, nil
}

// GetLatestRevision returns the latest revision for a project's repository.
func (c *Client) GetLatestRevision(ctx context.Context, projectID string) (string, error) {
	var result struct {
		Ref string `json:"ref"`
	}
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/repository/latest_revision", projectID), &result); err != nil {
		return "", err
	}
	return result.Ref, nil
}

// ListRecentCommits returns recent commits for a project's repository.
func (c *Client) ListRecentCommits(ctx context.Context, projectID string) (*CommitsTagsReleases, error) {
	var result CommitsTagsReleases
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/repository/recent_commits", projectID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCommitInfo returns information about a specific commit.
func (c *Client) GetCommitInfo(ctx context.Context, projectID, ref string) (*Commit, error) {
	var commit Commit
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/repository/commit_info?ref=%s", projectID, ref), &commit); err != nil {
		return nil, err
	}
	return &commit, nil
}
