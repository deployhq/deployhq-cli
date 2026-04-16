package sdk

import (
	"context"
	"fmt"
)

// BuildCacheFile represents a build cache file entry.
type BuildCacheFile struct {
	Identifier string `json:"identifier"`
	Path       string `json:"path"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// BuildCacheFileCreateRequest is the payload for creating/updating a build cache file.
type BuildCacheFileCreateRequest struct {
	Path string `json:"path"`
}

func (c *Client) ListBuildCacheFiles(ctx context.Context, projectID string) ([]BuildCacheFile, error) {
	var files []BuildCacheFile
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/build_cache_files", projectID), &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *Client) CreateBuildCacheFile(ctx context.Context, projectID string, req BuildCacheFileCreateRequest) (*BuildCacheFile, error) {
	body := struct {
		BuildCacheFile BuildCacheFileCreateRequest `json:"build_cache_file"`
	}{BuildCacheFile: req}
	var file BuildCacheFile
	if err := c.post(ctx, fmt.Sprintf("/projects/%s/build_cache_files", projectID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) UpdateBuildCacheFile(ctx context.Context, projectID, fileID string, req BuildCacheFileCreateRequest) (*BuildCacheFile, error) {
	body := struct {
		BuildCacheFile BuildCacheFileCreateRequest `json:"build_cache_file"`
	}{BuildCacheFile: req}
	var file BuildCacheFile
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/build_cache_files/%s", projectID, fileID), body, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

func (c *Client) DeleteBuildCacheFile(ctx context.Context, projectID, fileID string) error {
	return c.delete(ctx, fmt.Sprintf("/projects/%s/build_cache_files/%s", projectID, fileID))
}
