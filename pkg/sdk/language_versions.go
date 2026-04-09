package sdk

import (
	"context"
	"fmt"
)

// ListLanguageVersions returns the available language versions for a project's
// build server. The result maps language names to their available versions.
func (c *Client) ListLanguageVersions(ctx context.Context, projectID string, opts *ListOptions) (map[string][]string, error) {
	var result map[string][]string
	path := appendListParams(fmt.Sprintf("/projects/%s/language_versions", projectID), opts)
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}
