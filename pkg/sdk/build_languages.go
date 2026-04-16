package sdk

import (
	"context"
	"fmt"
)

// BuildLanguage represents a project's build language version setting.
type BuildLanguage struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

// BuildLanguageUpdateRequest is the payload for updating a build language version.
type BuildLanguageUpdateRequest struct {
	Version string `json:"version"`
}

func (c *Client) UpdateBuildLanguage(ctx context.Context, projectID, languageID string, req BuildLanguageUpdateRequest) (*BuildLanguage, error) {
	body := struct {
		BuildLanguage BuildLanguageUpdateRequest `json:"build_language"`
	}{BuildLanguage: req}
	var lang BuildLanguage
	if err := c.put(ctx, fmt.Sprintf("/projects/%s/build_languages/%s", projectID, languageID), body, &lang); err != nil {
		return nil, err
	}
	return &lang, nil
}

func (c *Client) UpdateBuildLanguageOverride(ctx context.Context, projectID, buildConfigID, languageID string, req BuildLanguageUpdateRequest) (*BuildLanguage, error) {
	body := struct {
		BuildLanguage BuildLanguageUpdateRequest `json:"build_language"`
	}{BuildLanguage: req}
	var lang BuildLanguage
	path := fmt.Sprintf("/projects/%s/build_configurations/%s/build_languages/%s", projectID, buildConfigID, languageID)
	if err := c.put(ctx, path, body, &lang); err != nil {
		return nil, err
	}
	return &lang, nil
}
