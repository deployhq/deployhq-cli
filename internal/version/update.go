// Package version provides version update checking against GitHub releases.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	releasesURL = "https://api.github.com/repos/deployhq/deployhq-cli/releases/latest"
	checkTimeout = 3 * time.Second
)

// UpdateInfo holds the result of an update check.
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	UpdateAvailable bool `json:"update_available"`
	URL       string `json:"url,omitempty"`
}

// Check compares the current version against the latest GitHub release.
// Returns quickly (3s timeout) and never errors — returns partial info on failure.
func Check(currentVersion string) UpdateInfo {
	info := UpdateInfo{Current: currentVersion}

	if currentVersion == "dev" {
		return info
	}

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return info
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return info
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return info
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	info.Latest = latest
	info.URL = release.HTMLURL
	info.UpdateAvailable = latest != strings.TrimPrefix(currentVersion, "v")

	return info
}

// FormatUpdateMessage returns a human-readable update notice, or empty string if up to date.
func FormatUpdateMessage(info UpdateInfo) string {
	if !info.UpdateAvailable || info.Latest == "" {
		return ""
	}
	return fmt.Sprintf("Update available: %s -> %s (%s)", info.Current, info.Latest, info.URL)
}
