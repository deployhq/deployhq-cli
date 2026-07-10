package sdk

import (
	"context"
	"fmt"
)

// Folder represents an account-level project folder (project category).
// The API keys folders by their string identifier (a UUID), never an integer id.
type Folder struct {
	Identifier    string `json:"identifier"`
	Name          string `json:"name"`
	Position      *int   `json:"position"` // nullable, read-only via the API (reorder is UI-only)
	ProjectsCount int    `json:"projects_count"`
}

// FolderCreateRequest is the payload for creating or updating a folder.
// Only name is writable.
type FolderCreateRequest struct {
	Name string `json:"name"`
}

func (c *Client) ListFolders(ctx context.Context, opts *ListOptions) ([]Folder, error) {
	var folders []Folder
	path := appendListParams("/folders", opts)
	if err := c.get(ctx, path, &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

func (c *Client) CreateFolder(ctx context.Context, req FolderCreateRequest) (*Folder, error) {
	body := struct {
		Folder FolderCreateRequest `json:"folder"`
	}{Folder: req}
	var folder Folder
	if err := c.post(ctx, "/folders", body, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

func (c *Client) UpdateFolder(ctx context.Context, identifier string, req FolderCreateRequest) (*Folder, error) {
	body := struct {
		Folder FolderCreateRequest `json:"folder"`
	}{Folder: req}
	var folder Folder
	if err := c.put(ctx, fmt.Sprintf("/folders/%s", identifier), body, &folder); err != nil {
		return nil, err
	}
	return &folder, nil
}

func (c *Client) DeleteFolder(ctx context.Context, identifier string) error {
	return c.delete(ctx, fmt.Sprintf("/folders/%s", identifier))
}
