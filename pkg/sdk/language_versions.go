package sdk

import (
	"context"
	"fmt"
)

// ListLanguageVersions returns the available language versions for a project's
// build server. The result maps language names to their available versions.
func (c *Client) ListLanguageVersions(ctx context.Context, projectID string) (map[string][]string, error) {
	var result map[string][]string
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/language_versions", projectID), &result); err != nil {
		return nil, err
	}
	return result, nil
}
