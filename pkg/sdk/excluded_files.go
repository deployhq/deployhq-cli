package sdk

import (
	"context"
	"fmt"
)

// ExcludedFile represents an excluded file pattern.
type ExcludedFile struct {
	Identifier string   `json:"identifier"`
	Path       string   `json:"path"`
	Servers    []string `json:"servers,omitempty"`
}

// ExcludedFileCreateRequest is the payload for creating/updating an excluded file.
type ExcludedFileCreateRequest struct {
	Path string `json:"path"`
}

func (c *Client) ListExcludedFiles(ctx context.Context, projectID string) ([]ExcludedFile, error) {
	var files []ExcludedFile
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/excluded_files", projectID), &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *Client) GetExcludedFile(ctx context.Context, projectID, fileID string) (*ExcludedFile, error) {
	var file ExcludedFile
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/excluded_files/%s", projectID, fileID), &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) CreateExcludedFile(ctx context.Context, projectID string, req ExcludedFileCreateRequest) (*ExcludedFile, error) {
	body := struct {
		ExcludedFile ExcludedFileCreateRequest `json:"excluded_file"`
	}{ExcludedFile: req}
	var file ExcludedFile
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/excluded_files", projectID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) UpdateExcludedFile(ctx context.Context, projectID, fileID string, req ExcludedFileCreateRequest) (*ExcludedFile, error) {
	body := struct {
		ExcludedFile ExcludedFileCreateRequest `json:"excluded_file"`
	}{ExcludedFile: req}
	var file ExcludedFile
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/excluded_files/%s", projectID, fileID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) DeleteExcludedFile(ctx context.Context, projectID, fileID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/excluded_files/%s", projectID, fileID))
}
