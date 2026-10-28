package commands

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The table-mode tip must point at a workflow that works: build-configs
// create/update reject --packages, so the tip must not suggest it.
func TestLanguageVersionsList_TipPointsAtBuildLanguagesSet(t *testing.T) {
	withResolvableContext(t)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		if strings.HasSuffix(r.URL.Path, "/language_versions") {
			body = `{"node":["18","20"],"ruby":["3.3"]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	stderr, err := runBuildConfigsCmd(t, "language-versions", "list", "--table")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Tip:")
	assert.NotContains(t, stderr, "--packages")
	assert.Contains(t, stderr, "build-languages set")
	// Both suggested commands name the project, so they work when it was only
	// given with -p on this command.
	assert.Contains(t, stderr, "dhq build-configs create -p my-app --server")
	assert.Contains(t, stderr, "dhq build-languages set -p my-app <language>")
}
