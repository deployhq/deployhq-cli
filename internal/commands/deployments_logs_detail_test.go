package commands

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each log's detail is printed under its message, as the DeployHQ web app
// shows it: without it, "Excluded files for server ..." showed no files.
func TestPrintStepLogs_PrintsDetail(t *testing.T) {
	detail := "  - deploy.sh (1 file)\n"
	blank := "  \n"
	var out bytes.Buffer
	printStepLogs(&out, []sdk.DeploymentStepLog{
		{Message: "Excluded files for server 'site'", Detail: &detail},
		{Message: "Skipped due to project/account settings"},
		{Message: "Done", Detail: &blank},
	})
	assert.Equal(t, "Excluded files for server 'site'\n  - deploy.sh (1 file)\nSkipped due to project/account settings\nDone\n", out.String())
}

// Without --step, --json returns every step's logs (with their detail) instead
// of printing text.
func TestDeploymentsLogs_AllStepsJSON(t *testing.T) {
	withResolvableContext(t)
	out := filepath.Join(t.TempDir(), "out.jsonl")
	t.Setenv("DEPLOYHQ_OUTPUT_FILE", out)
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/projects/my-app/deployments/dep1"):
			body = `{"identifier":"dep1","status":"completed","steps":[
				{"identifier":"s1","description":"Generating manifest","status":"completed","logs":true},
				{"identifier":"s2","description":"Transferring files","status":"completed","logs":false}]}`
		case strings.HasSuffix(r.URL.Path, "/deployments/dep1/steps/s1/logs"):
			body = `[{"id":1,"step":"s1","message":"Excluded files for server 'site'","detail":"  - deploy.sh (1 file)"}]`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)),
			Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}, nil
	})

	require.NoError(t, runServersCmd(t, "deployments", "logs", "dep1", "--json"))
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"description":"Generating manifest"`)
	assert.Contains(t, string(b), `deploy.sh (1 file)`)
	assert.NotContains(t, string(b), "Transferring files", "steps without logs are left out")
}
