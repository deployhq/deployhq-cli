package sdk

import (
	"context"
	"fmt"
)

// ExcludedFile represents an excluded file pattern.
type ExcludedFile struct {
	Identifier string `json:"identifier"`
	Path       string `json:"path"`
	// Servers is the list of servers the rule applies to. It is deliberately
	// not omitempty: an empty list means the rule applies to no server, and
	// hiding that would make a no-op rule look like a working one.
	Servers []interface{} `json:"servers"`
}

// ExcludedFileCreateRequest is the payload for creating/updating an excluded file.
//
// Server targeting: set AllServers to apply the rule to every server in the
// project (including servers added later), or ServerIdentifiers to target
// specific servers or server groups. On create, leaving both unset stores a
// rule that applies to no server. On update, leaving both unset keeps the
// existing targeting. Never set both: AllServers wins server-side.
type ExcludedFileCreateRequest struct {
	Path              string   `json:"path,omitempty"`
	AllServers        bool     `json:"all_servers,omitempty"`
	ServerIdentifiers []string `json:"server_identifiers,omitempty"`
}

func (c *Client) ListExcludedFiles(ctx context.Context, projectID string, opts *ListOptions) ([]ExcludedFile, error) {
	var files []ExcludedFile
	path := appendListParams(fmt.Sprintf("/projects/%s/excluded_files", projectID), opts)
	if err := c.get(ctx, path, &files); err != nil {
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
